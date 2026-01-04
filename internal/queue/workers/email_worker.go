// -----------------------------------------------------------------------
// EmailWorker - Sends email notifications with job results
// Used as a step in job definitions to email results/summaries to users
// -----------------------------------------------------------------------

package workers

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/services/mailer"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// EmailWorker handles email notification steps in job definitions.
// This worker executes synchronously (no child jobs) and sends a single email
// with content from the step configuration or from previous step outputs.
type EmailWorker struct {
	mailerService   *mailer.Service
	documentStorage interfaces.DocumentStorage
	searchService   interfaces.SearchService
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	serverHost      string
	serverPort      int
}

// Compile-time assertion: EmailWorker implements DefinitionWorker interface
var _ interfaces.DefinitionWorker = (*EmailWorker)(nil)

// NewEmailWorker creates a new email worker
func NewEmailWorker(
	mailerService *mailer.Service,
	documentStorage interfaces.DocumentStorage,
	searchService interfaces.SearchService,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	serverHost string,
	serverPort int,
) *EmailWorker {
	return &EmailWorker{
		mailerService:   mailerService,
		documentStorage: documentStorage,
		searchService:   searchService,
		logger:          logger,
		jobMgr:          jobMgr,
		serverHost:      serverHost,
		serverPort:      serverPort,
	}
}

// GetType returns WorkerTypeEmail for the DefinitionWorker interface
func (w *EmailWorker) GetType() models.WorkerType {
	return models.WorkerTypeEmail
}

// Init performs the initialization/setup phase for an email step.
// This is where we:
//   - Extract and validate configuration (to, subject, body)
//   - Check that mailer is configured
//   - Return metadata for CreateJobs
//
// The Init phase does NOT send the email - it only validates and prepares.
func (w *EmailWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for email")
	}

	// Extract "to" email address (required)
	to, ok := stepConfig["to"].(string)
	if !ok || to == "" {
		return nil, fmt.Errorf("'to' email address is required in step config")
	}

	// Extract subject (optional, has default)
	subject := "Quaero Job Results"
	if subj, ok := stepConfig["subject"].(string); ok && subj != "" {
		subject = subj
	}

	// Check if mailer is configured
	if !w.mailerService.IsConfigured(ctx) {
		return nil, fmt.Errorf("email is not configured. Please configure SMTP settings in Settings > Email")
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Str("to", to).
		Str("subject", subject).
		Msg("Email worker initialized")

	// No child jobs for email - work items represent the single email to send
	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:   "email",
				Name: fmt.Sprintf("Send email to %s", to),
				Type: "email",
				Config: map[string]interface{}{
					"to":      to,
					"subject": subject,
				},
			},
		},
		TotalCount:           1,
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"to":          to,
			"subject":     subject,
			"step_config": stepConfig,
		},
	}, nil
}

// CreateJobs sends the email synchronously - no child jobs are created.
// Returns the step job ID since email executes synchronously.
func (w *EmailWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize email worker: %w", err)
		}
	}

	// Extract metadata from init result
	to, _ := initResult.Metadata["to"].(string)
	subject, _ := initResult.Metadata["subject"].(string)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Check for errors in previous steps
	errorInfo := w.checkJobForErrors(ctx, stepID)
	includeLogsOnError := true // Default to including logs
	if val, ok := stepConfig["include_logs_on_error"].(bool); ok {
		includeLogsOnError = val
	}

	// Get body - can be from step config, or from previous step's output document
	// Also track source document for adding source links
	bodyResult := w.resolveBodyWithSource(ctx, stepConfig)
	body := bodyResult.textBody
	htmlBody := bodyResult.htmlBody

	// Determine if we should switch to error mode:
	// 1. Previous steps have errors, or
	// 2. Body is empty (no data scenario)
	isErrorMode := errorInfo.hasErrors || (body == "" && htmlBody == "")

	if isErrorMode {
		// Switch to error reporting mode
		if onErrorSubject, ok := stepConfig["on_error_subject"].(string); ok && onErrorSubject != "" {
			subject = onErrorSubject
		} else if errorInfo.hasErrors {
			// Append error indicator to subject
			subject = subject + " - Error Occurred"
		}

		// Generate error body
		if includeLogsOnError || body == "" {
			// Get parent job ID for error report
			parentJobID := ""
			if stepJobInterface, err := w.jobMgr.GetJob(ctx, stepID); err == nil {
				if stepJob, ok := stepJobInterface.(*models.QueueJobState); ok && stepJob != nil && stepJob.ParentID != nil {
					parentJobID = *stepJob.ParentID
				}
			}
			body = w.generateErrorEmailBody(errorInfo, parentJobID)
			htmlBody = w.convertMarkdownToHTML(body)
		}

		w.logger.Info().
			Bool("has_errors", errorInfo.hasErrors).
			Int("failed_steps", len(errorInfo.failedSteps)).
			Str("step_id", stepID).
			Msg("Email worker detected errors, switching to error mode")

		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "warn", "Sending error notification email due to job failures or missing data")
		}
	} else if body == "" && htmlBody == "" {
		body = "Job completed. No content was specified for this email."
	}

	// Find and append source document links if we have a source document
	if bodyResult.sourceDoc != nil {
		// Get base URL from step config, or construct from server config
		baseURL := fmt.Sprintf("http://%s:%d", w.serverHost, w.serverPort)
		if configBaseURL, ok := stepConfig["base_url"].(string); ok && configBaseURL != "" {
			baseURL = configBaseURL
		}

		// Find related source documents
		sources := w.findSourceDocuments(ctx, bodyResult.sourceDoc, stepConfig, baseURL)
		if len(sources) > 0 {
			sourceLinksHTML := w.formatSourceLinksHTML(sources, baseURL)

			// Inject source links before the closing </div> of .content
			// The wrapInEmailTemplate structure is: <div class="content">CONTENT</div><div class="footer">...
			// We want to add source links at the end of CONTENT but before closing </div>
			if htmlBody != "" && sourceLinksHTML != "" {
				// Find the position to inject (before </div> of .content)
				insertPos := strings.LastIndex(htmlBody, `</div>
  <div class="footer">`)
				if insertPos > 0 {
					htmlBody = htmlBody[:insertPos] + sourceLinksHTML + htmlBody[insertPos:]
				}
			}

			w.logger.Info().
				Int("source_count", len(sources)).
				Str("step_id", stepID).
				Msg("Added source document links to email")
		}
	}

	// Log job start
	w.logger.Info().
		Str("step_id", stepID).
		Str("step_name", step.Name).
		Str("to", to).
		Str("subject", subject).
		Msg("Sending email")

	// Add job log
	if err := w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Sending email to %s", to)); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to add job log")
	}

	// Send the email
	w.logger.Info().
		Bool("has_html", htmlBody != "").
		Int("html_len", len(htmlBody)).
		Int("text_len", len(body)).
		Msg("Sending email with body")

	// Log HTML conversion result explicitly for test assertion visibility
	if htmlBody != "" {
		if err := w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("HTML email body generated (%d bytes) from markdown content", len(htmlBody))); err != nil {
			w.logger.Warn().Err(err).Msg("Failed to add HTML conversion log")
		}

		// Save HTML body as document for verification and debugging
		// This allows tests to retrieve and verify actual HTML content
		htmlDoc := w.saveHTMLDocument(ctx, stepID, subject, htmlBody)
		if htmlDoc != nil {
			if err := w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Email HTML document saved: %s", htmlDoc.ID)); err != nil {
				w.logger.Warn().Err(err).Msg("Failed to add HTML document log")
			}
		}
	}

	var err error
	if htmlBody != "" {
		w.logger.Debug().Msg("Sending HTML email")
		err = w.mailerService.SendHTMLEmail(ctx, to, subject, htmlBody, body)
	} else {
		w.logger.Debug().Msg("Sending plain text email (no HTML body)")
		err = w.mailerService.SendEmail(ctx, to, subject, body)
	}

	if err != nil {
		w.logger.Error().Err(err).
			Str("step_id", stepID).
			Str("to", to).
			Msg("Failed to send email")

		if logErr := w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed to send email: %v", err)); logErr != nil {
			w.logger.Warn().Err(logErr).Msg("Failed to add job log")
		}

		return "", fmt.Errorf("failed to send email: %w", err)
	}

	// Log success
	if logErr := w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Email sent successfully to %s", to)); logErr != nil {
		w.logger.Warn().Err(logErr).Msg("Failed to add job log")
	}

	w.logger.Info().
		Str("step_id", stepID).
		Str("to", to).
		Msg("Email sent successfully")

	return stepID, nil
}

// ReturnsChildJobs returns false since email executes synchronously
func (w *EmailWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration for email type
func (w *EmailWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("email step requires config")
	}

	// Validate required 'to' field
	to, ok := step.Config["to"].(string)
	if !ok || to == "" {
		return fmt.Errorf("email step requires 'to' in config")
	}

	return nil
}

// emailBodyResult contains the resolved email body and source document info
type emailBodyResult struct {
	textBody   string
	htmlBody   string
	sourceDoc  *models.Document // The document used as email body (if any)
	sourceTags []string         // Tags used to find the source document
}

// jobErrorInfo contains information about job errors for error reporting
type jobErrorInfo struct {
	hasErrors     bool
	failedSteps   []string
	errorMessages []string
	logs          []string
}

// resolveBody determines the email body from step configuration
// Supports:
//   - body (direct text/markdown)
//   - body_html (HTML)
//   - body_from_document (document ID)
//   - body_from_tag (latest document with single tag)
//   - input (tag or array of tags to filter documents - matches output_tags from previous steps)
//
// All text content is converted to HTML for proper email presentation
func (w *EmailWorker) resolveBody(ctx context.Context, stepConfig map[string]interface{}) (textBody, htmlBody string) {
	result := w.resolveBodyWithSource(ctx, stepConfig)
	return result.textBody, result.htmlBody
}

// resolveBodyWithSource determines the email body and returns source document info
func (w *EmailWorker) resolveBodyWithSource(ctx context.Context, stepConfig map[string]interface{}) emailBodyResult {
	var result emailBodyResult

	// Direct body text (markdown is converted to HTML)
	if body, ok := stepConfig["body"].(string); ok && body != "" {
		result.textBody = body
		// Convert markdown to HTML for rich email formatting
		result.htmlBody = w.convertMarkdownToHTML(body)
	}

	// Direct HTML body (overrides conversion from body)
	if html, ok := stepConfig["body_html"].(string); ok && html != "" {
		result.htmlBody = html
	}

	// Body from document ID
	if docID, ok := stepConfig["body_from_document"].(string); ok && docID != "" {
		if doc, err := w.documentStorage.GetDocument(docID); err == nil && doc != nil {
			if doc.ContentMarkdown != "" {
				result.textBody = doc.ContentMarkdown
				// Convert markdown to HTML for rich email formatting
				result.htmlBody = w.convertMarkdownToHTML(doc.ContentMarkdown)
				result.sourceDoc = doc
			}
		} else {
			w.logger.Warn().Str("document_id", docID).Err(err).Msg("Failed to load document for email body")
		}
	}

	// Body from tag (get most recently CREATED document with tag)
	// Order by created_at to get the newest summary, not the most recently updated
	if tag, ok := stepConfig["body_from_tag"].(string); ok && tag != "" {
		w.logger.Debug().Str("tag", tag).Msg("Looking for document by tag for email body")
		opts := interfaces.SearchOptions{
			Tags:     []string{tag},
			Limit:    1,
			OrderBy:  "created_at",
			OrderDir: "desc",
		}
		results, err := w.searchService.Search(ctx, "", opts)
		if err == nil && len(results) > 0 {
			w.logger.Debug().Str("doc_id", results[0].ID).Msg("Found document by tag")
			if doc, err := w.documentStorage.GetDocument(results[0].ID); err == nil && doc != nil {
				if doc.ContentMarkdown != "" {
					w.logger.Debug().Int("markdown_len", len(doc.ContentMarkdown)).Msg("Document has markdown content, converting to HTML")
					result.textBody = doc.ContentMarkdown
					// Convert markdown to HTML for rich email formatting
					result.htmlBody = w.convertMarkdownToHTML(doc.ContentMarkdown)
					result.sourceDoc = doc
					result.sourceTags = []string{tag}
					w.logger.Debug().Int("html_len", len(result.htmlBody)).Msg("HTML body after conversion")
				} else {
					w.logger.Warn().Str("doc_id", results[0].ID).Msg("Document has no markdown content")
				}
			} else {
				w.logger.Warn().Str("doc_id", results[0].ID).Err(err).Msg("Failed to get document by ID")
			}
		} else {
			w.logger.Warn().Str("tag", tag).Err(err).Msg("Failed to find document by tag for email body")
		}
	}

	// Body from input tags (matches output_tags from previous steps)
	// Supports single tag string or array of tags for filtering
	if inputRaw, ok := stepConfig["input"]; ok {
		var inputTags []string

		// Handle single string or array of strings
		switch v := inputRaw.(type) {
		case string:
			if v != "" {
				inputTags = []string{v}
			}
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok && s != "" {
					inputTags = append(inputTags, s)
				}
			}
		case []string:
			inputTags = v
		}

		if len(inputTags) > 0 {
			w.logger.Debug().Strs("input_tags", inputTags).Msg("Looking for document by input tags for email body")
			opts := interfaces.SearchOptions{
				Tags:     inputTags,
				Limit:    1,
				OrderBy:  "created_at",
				OrderDir: "desc",
			}
			results, err := w.searchService.Search(ctx, "", opts)
			if err == nil && len(results) > 0 {
				w.logger.Debug().Str("doc_id", results[0].ID).Msg("Found document by input tags")
				if doc, err := w.documentStorage.GetDocument(results[0].ID); err == nil && doc != nil {
					if doc.ContentMarkdown != "" {
						w.logger.Debug().Int("markdown_len", len(doc.ContentMarkdown)).Msg("Document has markdown content, converting to HTML")
						result.textBody = doc.ContentMarkdown
						result.htmlBody = w.convertMarkdownToHTML(doc.ContentMarkdown)
						result.sourceDoc = doc
						result.sourceTags = inputTags
						w.logger.Debug().Int("html_len", len(result.htmlBody)).Msg("HTML body after conversion")
					} else {
						w.logger.Warn().Str("doc_id", results[0].ID).Msg("Document has no markdown content")
					}
				} else {
					w.logger.Warn().Str("doc_id", results[0].ID).Err(err).Msg("Failed to get document by ID")
				}
			} else {
				w.logger.Warn().Strs("tags", inputTags).Err(err).Msg("Failed to find document by input tags for email body")
			}
		}
	}

	return result
}

// checkJobForErrors checks if the parent job or any previous steps have failed
// Returns error info including failed steps and their logs
func (w *EmailWorker) checkJobForErrors(ctx context.Context, stepID string) jobErrorInfo {
	result := jobErrorInfo{}

	// Get the step job to find the parent manager
	stepJobInterface, err := w.jobMgr.GetJob(ctx, stepID)
	if err != nil {
		w.logger.Debug().Err(err).Str("step_id", stepID).Msg("Could not get step job for error check")
		return result
	}

	stepJob, ok := stepJobInterface.(*models.QueueJobState)
	if !ok || stepJob == nil {
		return result
	}

	// Get the parent manager job
	if stepJob.ParentID == nil || *stepJob.ParentID == "" {
		return result
	}
	parentID := *stepJob.ParentID

	managerJobInterface, err := w.jobMgr.GetJob(ctx, parentID)
	if err != nil {
		w.logger.Debug().Err(err).Str("parent_id", parentID).Msg("Could not get manager job for error check")
		return result
	}

	managerJob, ok := managerJobInterface.(*models.QueueJobState)
	if !ok || managerJob == nil {
		return result
	}

	// Check step_stats in manager metadata for failed steps
	if managerJob.Metadata != nil {
		if stepStats, ok := managerJob.Metadata["step_stats"].([]interface{}); ok {
			for _, statInterface := range stepStats {
				if stat, ok := statInterface.(map[string]interface{}); ok {
					status, _ := stat["status"].(string)
					stepName, _ := stat["step_name"].(string)
					stepJobID, _ := stat["step_id"].(string)

					if status == "failed" || status == "error" {
						result.hasErrors = true
						result.failedSteps = append(result.failedSteps, stepName)

						// Get logs from the failed step
						if stepJobID != "" {
							logs, err := w.jobMgr.GetJobLogs(ctx, stepJobID, 50)
							if err == nil {
								for _, log := range logs {
									if log.Level == "error" || log.Level == "fatal" {
										result.errorMessages = append(result.errorMessages, log.Message)
									}
									result.logs = append(result.logs, fmt.Sprintf("[%s] %s", log.Level, log.Message))
								}
							}
						}
					}
				}
			}
		}
	}

	// Also check manager job error field
	if managerJob.Error != "" {
		result.hasErrors = true
		result.errorMessages = append(result.errorMessages, managerJob.Error)
	}

	return result
}

// generateErrorEmailBody creates the email body for error notifications
func (w *EmailWorker) generateErrorEmailBody(errorInfo jobErrorInfo, jobID string) string {
	var content strings.Builder

	content.WriteString("# An Error Occurred\n\n")
	content.WriteString("The job encountered errors during execution.\n\n")

	if len(errorInfo.failedSteps) > 0 {
		content.WriteString("## Failed Steps\n\n")
		for _, step := range errorInfo.failedSteps {
			content.WriteString(fmt.Sprintf("- %s\n", step))
		}
		content.WriteString("\n")
	}

	if len(errorInfo.errorMessages) > 0 {
		content.WriteString("## Error Details\n\n")
		for _, msg := range errorInfo.errorMessages {
			content.WriteString(fmt.Sprintf("- %s\n", msg))
		}
		content.WriteString("\n")
	}

	if len(errorInfo.logs) > 0 {
		content.WriteString("## Step Logs\n\n")
		content.WriteString("```\n")
		// Limit to last 30 logs
		startIdx := 0
		if len(errorInfo.logs) > 30 {
			startIdx = len(errorInfo.logs) - 30
			content.WriteString("... (earlier logs truncated)\n")
		}
		for _, log := range errorInfo.logs[startIdx:] {
			content.WriteString(log + "\n")
		}
		content.WriteString("```\n\n")
	}

	content.WriteString("---\n\n")
	content.WriteString(fmt.Sprintf("Job ID: %s\n", jobID))

	return content.String()
}

// saveHTMLDocument saves the HTML email body as a document for verification
// This creates a retrievable artifact that tests can use to verify actual HTML content
func (w *EmailWorker) saveHTMLDocument(ctx context.Context, stepID, subject, htmlBody string) *models.Document {
	if htmlBody == "" {
		return nil
	}

	now := time.Now()
	shortStepID := stepID
	if len(stepID) > 8 {
		shortStepID = stepID[:8]
	}

	doc := &models.Document{
		ID:              "doc_" + uuid.New().String(),
		SourceType:      "email_html",
		SourceID:        stepID,
		Title:           fmt.Sprintf("Email HTML: %s", subject),
		ContentMarkdown: htmlBody, // Store HTML in ContentMarkdown for retrieval via API
		DetailLevel:     models.DetailLevelFull,
		Metadata: map[string]interface{}{
			"step_id":    stepID,
			"subject":    subject,
			"html_bytes": len(htmlBody),
		},
		Tags:       []string{"email-html", fmt.Sprintf("email-html-%s", shortStepID)},
		CreatedAt:  now,
		UpdatedAt:  now,
		LastSynced: &now,
	}

	if err := w.documentStorage.SaveDocument(doc); err != nil {
		w.logger.Warn().Err(err).Str("doc_id", doc.ID).Msg("Failed to save HTML email document")
		return nil
	}

	w.logger.Debug().Str("doc_id", doc.ID).Msg("Saved HTML email document for verification")
	return doc
}

// convertMarkdownToHTML converts markdown content to styled HTML for email
// Always returns HTML - uses goldmark for conversion with preprocessing for LLM output
func (w *EmailWorker) convertMarkdownToHTML(markdown string) string {
	if markdown == "" {
		w.logger.Debug().Msg("convertMarkdownToHTML: empty markdown input")
		return ""
	}

	// Strip outer markdown code fences that LLMs often wrap their output in
	// Common patterns: ```markdown\n...\n``` or ```\n...\n```
	markdown = w.stripOuterCodeFences(markdown)

	w.logger.Debug().Int("markdown_len", len(markdown)).Msg("Converting markdown to HTML using goldmark")

	// Create goldmark instance with GitHub Flavored Markdown extensions
	// WithUnsafe() allows raw HTML (like <span style="color:green">) to pass through
	// This is needed for colored indicators (▲/▼) in job outputs
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM, // GitHub Flavored Markdown (tables, strikethrough, etc.)
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(), // Allow raw HTML for colored indicators
		),
	)

	var buf bytes.Buffer
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		w.logger.Error().Err(err).Int("input_len", len(markdown)).Msg("Failed to convert markdown to HTML")
		// Return the markdown wrapped in pre tags as fallback
		return w.wrapInEmailTemplate("<pre>" + escapeHTML(markdown) + "</pre>")
	}

	htmlContent := buf.String()
	w.logger.Debug().Int("html_len", len(htmlContent)).Msg("Markdown converted to HTML successfully")

	// Wrap in styled HTML email template
	result := w.wrapInEmailTemplate(htmlContent)
	w.logger.Debug().Int("final_len", len(result)).Msg("HTML wrapped in email template")

	return result
}

// stripOuterCodeFences removes markdown code fences that wrap the entire content
// LLMs often output their responses wrapped in ```markdown ... ```
// Also handles unclosed code fences (``` at start but no proper closing)
func (w *EmailWorker) stripOuterCodeFences(content string) string {
	content = strings.TrimSpace(content)

	// Check if content starts with a code fence
	if strings.HasPrefix(content, "```") {
		// Find the end of the opening fence line
		firstNewline := strings.Index(content, "\n")
		if firstNewline == -1 {
			return content
		}

		// Get the language hint (e.g., "markdown" from "```markdown")
		openingLine := content[:firstNewline]
		lang := strings.TrimPrefix(openingLine, "```")
		lang = strings.TrimSpace(lang)

		// Check for closing fence - could be "```" or "```\n" or surrounded by whitespace
		trimmedEnd := strings.TrimRight(content, " \t\n\r")

		if strings.HasSuffix(trimmedEnd, "```") {
			// Find the start of the closing fence
			lastFenceStart := strings.LastIndex(trimmedEnd, "\n```")
			if lastFenceStart == -1 {
				// Closing fence is at the very end without newline
				lastFenceStart = len(trimmedEnd) - 3
			}

			// Extract content between fences
			innerContent := content[firstNewline+1 : lastFenceStart]
			innerContent = strings.TrimSpace(innerContent)

			w.logger.Debug().
				Str("lang", lang).
				Int("original_len", len(content)).
				Int("inner_len", len(innerContent)).
				Msg("Stripped outer code fences from markdown")

			return innerContent
		}

		// UNCLOSED code fence - strip the opening fence and process content
		// This handles cases where LLM outputs ```markdown at start but forgets closing ```
		innerContent := content[firstNewline+1:]
		innerContent = strings.TrimSpace(innerContent)

		// Also strip any trailing incomplete fence like "``" or "`"
		innerContent = strings.TrimRight(innerContent, "`")
		innerContent = strings.TrimSpace(innerContent)

		w.logger.Warn().
			Str("lang", lang).
			Int("original_len", len(content)).
			Int("inner_len", len(innerContent)).
			Msg("Stripped UNCLOSED code fence from markdown (no closing ```)")

		return innerContent
	}

	return content
}

// containsRawMarkdown checks if HTML content still contains unconverted markdown patterns
// This is used to detect when goldmark fails to convert content properly
func (w *EmailWorker) containsRawMarkdown(html string) bool {
	// Check for markdown headers (## or # patterns)
	// These should NEVER appear in properly converted HTML
	// Check for patterns like "\n## ", "> ## ", "<p>## " etc.
	if strings.Contains(html, "## ") || strings.Contains(html, "# ") {
		// Verify it's actually a markdown header, not something else
		// Check if it's NOT already inside an HTML tag
		if strings.Contains(html, "\n## ") || strings.Contains(html, "\n# ") ||
			strings.Contains(html, ">## ") || strings.Contains(html, "># ") ||
			strings.HasPrefix(html, "## ") || strings.HasPrefix(html, "# ") {
			w.logger.Debug().Msg("containsRawMarkdown: found markdown headers")
			return true
		}
	}

	// Check for markdown bold (**text**)
	// Even if some <strong> tags exist, if ** is still present, conversion was incomplete
	if strings.Contains(html, "**") {
		w.logger.Debug().Msg("containsRawMarkdown: found markdown bold (**)")
		return true
	}

	// Check for markdown lists (- item or * item at start of lines)
	// These should NEVER appear in properly converted HTML
	if strings.Contains(html, "\n- ") || strings.HasPrefix(html, "- ") ||
		strings.Contains(html, "\n* ") || strings.HasPrefix(html, "* ") {
		// Make sure it's not inside a <style> or <code> block by checking context
		// Simple heuristic: if no <li> exists at all, definitely unconverted
		if !strings.Contains(html, "<li>") {
			w.logger.Debug().Msg("containsRawMarkdown: found markdown lists with no <li>")
			return true
		}
	}

	// Check for markdown code blocks (```)
	if strings.Contains(html, "```") {
		w.logger.Debug().Msg("containsRawMarkdown: found markdown code blocks (```)")
		return true
	}

	return false
}

// simpleMarkdownToHTML performs basic line-by-line markdown to HTML conversion
// This is used as a fallback when goldmark fails to parse malformed markdown from LLMs
func (w *EmailWorker) simpleMarkdownToHTML(markdown string) string {
	var result strings.Builder

	// Normalize line endings
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	markdown = strings.ReplaceAll(markdown, "\r", "\n")

	// Remove BOM if present
	markdown = strings.TrimPrefix(markdown, "\xef\xbb\xbf")

	lines := strings.Split(markdown, "\n")
	inCodeBlock := false
	inList := false
	inTable := false

	for i, line := range lines {
		// Normalize the line - remove non-breaking spaces, zero-width chars, etc.
		trimmed := strings.TrimSpace(line)
		trimmed = strings.ReplaceAll(trimmed, "\u00A0", " ") // Non-breaking space
		trimmed = strings.ReplaceAll(trimmed, "\u200B", "")  // Zero-width space
		trimmed = strings.ReplaceAll(trimmed, "\uFEFF", "")  // BOM

		// Handle code blocks FIRST (highest priority)
		if strings.HasPrefix(trimmed, "```") {
			if inCodeBlock {
				result.WriteString("</code></pre>\n")
				inCodeBlock = false
			} else {
				// Close any open blocks
				if inList {
					result.WriteString("</ul>\n")
					inList = false
				}
				if inTable {
					result.WriteString("</tbody>\n</table>\n")
					inTable = false
				}
				// Extract language hint if present
				lang := strings.TrimPrefix(trimmed, "```")
				if lang != "" {
					result.WriteString(fmt.Sprintf("<pre><code class=\"language-%s\">", escapeHTML(lang)))
				} else {
					result.WriteString("<pre><code>")
				}
				inCodeBlock = true
			}
			continue
		}

		if inCodeBlock {
			result.WriteString(escapeHTML(line))
			result.WriteString("\n")
			continue
		}

		// Handle headers BEFORE tables - a line starting with # is a header, not a table
		// even if it contains | characters
		headerContent, headerLevel := w.parseMarkdownHeader(trimmed)
		if headerLevel > 0 {
			if inList {
				result.WriteString("</ul>\n")
				inList = false
			}
			if inTable {
				result.WriteString("</tbody>\n</table>\n")
				inTable = false
			}
			result.WriteString(fmt.Sprintf("<h%d>", headerLevel))
			result.WriteString(w.processInlineMarkdown(headerContent))
			result.WriteString(fmt.Sprintf("</h%d>\n", headerLevel))
			continue
		}

		// Handle tables - detect by | character at start/in line
		if strings.Contains(trimmed, "|") && strings.Count(trimmed, "|") >= 2 {
			// Check if this is a table separator row (|---|---|)
			isSeparator := strings.Contains(trimmed, "---") || strings.Contains(trimmed, ":--") || strings.Contains(trimmed, "--:")

			if !inTable {
				// Start table - check if this is a header row
				inTable = true
				result.WriteString("<table>\n")

				// Check if next line is separator (indicates this is header)
				isHeader := false
				if i+1 < len(lines) {
					nextLine := strings.TrimSpace(lines[i+1])
					if strings.Contains(nextLine, "|") && (strings.Contains(nextLine, "---") || strings.Contains(nextLine, ":--")) {
						isHeader = true
					}
				}

				if isHeader && !isSeparator {
					result.WriteString("<thead>\n<tr>\n")
					cells := w.parseTableRow(trimmed)
					for _, cell := range cells {
						result.WriteString("<th>")
						result.WriteString(w.processInlineMarkdown(cell))
						result.WriteString("</th>\n")
					}
					result.WriteString("</tr>\n</thead>\n<tbody>\n")
					continue
				}
			}

			if isSeparator {
				// Skip separator rows
				continue
			}

			// Regular table row
			result.WriteString("<tr>\n")
			cells := w.parseTableRow(trimmed)
			for _, cell := range cells {
				result.WriteString("<td>")
				result.WriteString(w.processInlineMarkdown(cell))
				result.WriteString("</td>\n")
			}
			result.WriteString("</tr>\n")
			continue
		} else if inTable {
			// End of table
			result.WriteString("</tbody>\n</table>\n")
			inTable = false
		}

		// Handle horizontal rules
		if trimmed == "---" || trimmed == "***" || trimmed == "___" {
			if inList {
				result.WriteString("</ul>\n")
				inList = false
			}
			result.WriteString("<hr />\n")
			continue
		}

		// Handle unordered list items
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			if !inList {
				result.WriteString("<ul>\n")
				inList = true
			}
			content := strings.TrimPrefix(strings.TrimPrefix(trimmed, "- "), "* ")
			result.WriteString("<li>")
			result.WriteString(w.processInlineMarkdown(content))
			result.WriteString("</li>\n")
			continue
		}

		// Handle numbered list items
		if len(trimmed) > 2 && trimmed[0] >= '0' && trimmed[0] <= '9' {
			dotIdx := strings.Index(trimmed, ". ")
			if dotIdx > 0 && dotIdx < 4 {
				if !inList {
					result.WriteString("<ol>\n")
					inList = true
				}
				content := trimmed[dotIdx+2:]
				result.WriteString("<li>")
				result.WriteString(w.processInlineMarkdown(content))
				result.WriteString("</li>\n")
				continue
			}
		}

		// Close list if we hit a non-list line
		if inList && trimmed != "" {
			result.WriteString("</ul>\n")
			inList = false
		}

		// Handle blockquotes (> text)
		if strings.HasPrefix(trimmed, "> ") || trimmed == ">" {
			content := strings.TrimPrefix(trimmed, "> ")
			content = strings.TrimPrefix(content, ">")
			result.WriteString("<blockquote>")
			result.WriteString(w.processInlineMarkdown(content))
			result.WriteString("</blockquote>\n")
			continue
		}

		// Handle empty lines
		if trimmed == "" {
			result.WriteString("<br />\n")
			continue
		}

		// Handle regular paragraphs
		result.WriteString("<p>")
		result.WriteString(w.processInlineMarkdown(trimmed))
		result.WriteString("</p>\n")
	}

	// Close any open blocks
	if inCodeBlock {
		result.WriteString("</code></pre>\n")
	}
	if inList {
		result.WriteString("</ul>\n")
	}
	if inTable {
		result.WriteString("</tbody>\n</table>\n")
	}

	return result.String()
}

// parseMarkdownHeader parses a markdown header line and returns the content and level (1-6)
// Returns level 0 if not a header
// Handles various formats: "## Header", "##Header", "##  Header", etc.
func (w *EmailWorker) parseMarkdownHeader(line string) (content string, level int) {
	// Count leading # characters
	level = 0
	for i, c := range line {
		if c == '#' {
			level++
		} else {
			// Found end of # sequence
			if level > 0 && level <= 6 {
				// Extract content after the # symbols
				content = strings.TrimSpace(line[i:])
				// If content is empty and line only has #, not a header
				if content == "" && i == len(line) {
					return "", 0
				}
				return content, level
			}
			return "", 0
		}
	}
	// Line was all # characters - not a valid header
	return "", 0
}

// parseTableRow extracts cells from a markdown table row
func (w *EmailWorker) parseTableRow(row string) []string {
	// Remove leading/trailing | if present
	row = strings.TrimSpace(row)
	if strings.HasPrefix(row, "|") {
		row = row[1:]
	}
	if strings.HasSuffix(row, "|") {
		row = row[:len(row)-1]
	}

	// Split by | and trim each cell
	parts := strings.Split(row, "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}

// processInlineMarkdown handles inline markdown formatting (bold, italic, code, links)
func (w *EmailWorker) processInlineMarkdown(text string) string {
	// First escape HTML
	text = escapeHTML(text)

	// Process inline code (must be before bold/italic to avoid conflicts)
	// Match `code` patterns
	for {
		start := strings.Index(text, "`")
		if start == -1 {
			break
		}
		end := strings.Index(text[start+1:], "`")
		if end == -1 {
			// Unmatched backtick - remove it to avoid raw markdown in output
			text = text[:start] + text[start+1:]
			continue
		}
		end += start + 1
		code := text[start+1 : end]
		text = text[:start] + "<code>" + code + "</code>" + text[end+1:]
	}

	// Process bold (**text** or __text__)
	maxIterations := 100 // Prevent infinite loops
	for i := 0; i < maxIterations; i++ {
		start := strings.Index(text, "**")
		if start == -1 {
			start = strings.Index(text, "__")
		}
		if start == -1 {
			break
		}
		marker := text[start : start+2]
		end := strings.Index(text[start+2:], marker)
		if end == -1 {
			// Unmatched marker - remove it to avoid raw markdown in output
			text = text[:start] + text[start+2:]
			continue
		}
		end += start + 2
		bold := text[start+2 : end]
		text = text[:start] + "<strong>" + bold + "</strong>" + text[end+2:]
	}

	// Process italic (*text* or _text_) - be careful not to match ** or __
	for i := 0; i < maxIterations; i++ {
		// Find single * not followed by another *
		start := -1
		for j := 0; j < len(text); j++ {
			if text[j] == '*' || text[j] == '_' {
				// Check it's not part of ** or __
				if j > 0 && (text[j-1] == '*' || text[j-1] == '_') {
					continue
				}
				if j < len(text)-1 && (text[j+1] == '*' || text[j+1] == '_') {
					continue
				}
				start = j
				break
			}
		}
		if start == -1 {
			break
		}
		marker := string(text[start])
		// Find closing marker
		end := -1
		for j := start + 1; j < len(text); j++ {
			if string(text[j]) == marker {
				// Check it's not part of ** or __
				if j > 0 && (text[j-1] == '*' || text[j-1] == '_') {
					continue
				}
				if j < len(text)-1 && (text[j+1] == '*' || text[j+1] == '_') {
					continue
				}
				end = j
				break
			}
		}
		if end == -1 {
			// Unmatched marker - remove it to avoid raw markdown in output
			text = text[:start] + text[start+1:]
			continue
		}
		italic := text[start+1 : end]
		text = text[:start] + "<em>" + italic + "</em>" + text[end+1:]
	}

	return text
}

// escapeHTML escapes HTML special characters for safe embedding
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// wrapInEmailTemplate wraps HTML content in a styled email template
func (w *EmailWorker) wrapInEmailTemplate(content string) string {
	return `<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
      line-height: 1.6;
      color: #333;
      max-width: 800px;
      margin: 0 auto;
      padding: 20px;
      background-color: #f9f9f9;
    }
    .content {
      background-color: #fff;
      padding: 30px;
      border-radius: 8px;
      box-shadow: 0 1px 3px rgba(0,0,0,0.1);
    }
    h1 { color: #1a1a1a; font-size: 24px; margin-top: 0; border-bottom: 2px solid #eee; padding-bottom: 10px; }
    h2 { color: #2a2a2a; font-size: 20px; margin-top: 24px; }
    h3 { color: #3a3a3a; font-size: 16px; margin-top: 20px; }
    p { margin: 12px 0; }
    ul, ol { padding-left: 24px; margin: 12px 0; }
    li { margin: 6px 0; }
    strong { color: #1a1a1a; }
    code { background: #f4f4f4; padding: 2px 6px; border-radius: 3px; font-family: 'SF Mono', Monaco, 'Courier New', monospace; font-size: 14px; }
    pre { background: #f4f4f4; padding: 16px; border-radius: 6px; overflow-x: auto; font-family: 'SF Mono', Monaco, 'Courier New', monospace; font-size: 13px; }
    pre code { background: none; padding: 0; }
    blockquote { border-left: 4px solid #ddd; margin: 16px 0; padding-left: 16px; color: #666; }
    table { border-collapse: collapse; width: 100%; margin: 16px 0; }
    th, td { border: 1px solid #ddd; padding: 8px 12px; text-align: left; }
    th { background: #f4f4f4; font-weight: 600; }
    hr { border: none; border-top: 1px solid #eee; margin: 24px 0; }
    a { color: #0066cc; text-decoration: none; }
    a:hover { text-decoration: underline; }
    .footer { margin-top: 30px; padding-top: 20px; border-top: 1px solid #eee; font-size: 12px; color: #888; }
  </style>
</head>
<body>
  <div class="content">
    ` + content + `
  </div>
  <div class="footer">
    <p>This email was automatically generated by Quaero.</p>
  </div>
</body>
</html>`
}

// sourceDocInfo holds information about a source document for linking
type sourceDocInfo struct {
	ID             string
	Title          string
	SourceType     string
	SourceCategory string // "local", "data", "web"
	URL            string // External URL for data/web sources
}

// findSourceDocuments discovers related source documents based on the email body document's tags
// It looks for documents with tags like "asx-stock-data", "stock-recommendation", "stock-review"
// that share common stock ticker tags with the source document
func (w *EmailWorker) findSourceDocuments(ctx context.Context, sourceDoc *models.Document, stepConfig map[string]interface{}, baseURL string) []sourceDocInfo {
	var sources []sourceDocInfo

	if sourceDoc == nil {
		return sources
	}

	// Extract stock ticker tags from source document (typically 3-4 letter lowercase codes)
	var tickerTags []string
	for _, tag := range sourceDoc.Tags {
		// Skip non-ticker tags
		if tag == "smsf-portfolio-review" || tag == "smsf-portfolio" || tag == "portfolio-summary" ||
			tag == "stock-recommendation" || tag == "stock-portfolio" || tag == "asx-stock-data" ||
			tag == "stock-review" || tag == "summary" || strings.HasPrefix(tag, "date:") ||
			strings.HasPrefix(tag, "email") || strings.HasPrefix(tag, "job-") {
			continue
		}
		// Ticker tags are typically 2-5 lowercase letters
		if len(tag) >= 2 && len(tag) <= 6 && tag == strings.ToLower(tag) {
			tickerTags = append(tickerTags, tag)
		}
	}

	w.logger.Debug().
		Strs("source_tags", sourceDoc.Tags).
		Strs("ticker_tags", tickerTags).
		Msg("Finding source documents for email")

	// Source document types to find for each ticker
	sourceTypes := []string{"asx-stock-data", "stock-recommendation", "asx-announcement-summary"}

	// For each ticker, find related source documents
	for _, ticker := range tickerTags {
		for _, sourceType := range sourceTypes {
			opts := interfaces.SearchOptions{
				Tags:     []string{sourceType, ticker},
				Limit:    1,
				OrderBy:  "created_at",
				OrderDir: "desc",
			}
			results, err := w.searchService.Search(ctx, "", opts)
			if err != nil {
				w.logger.Debug().Err(err).Str("ticker", ticker).Str("source_type", sourceType).Msg("Error searching for source document")
				continue
			}
			if len(results) > 0 {
				doc := results[0]
				sources = append(sources, sourceDocInfo{
					ID:             doc.ID,
					Title:          fmt.Sprintf("ASX:%s %s", strings.ToUpper(ticker), sourceType),
					SourceType:     sourceType,
					SourceCategory: "local",
				})
				w.logger.Debug().Str("doc_id", doc.ID).Str("ticker", ticker).Str("source_type", sourceType).Msg("Found source document")

				// Extract API endpoints from document metadata for data sources
				if doc.Metadata != nil {
					if debugMeta, ok := doc.Metadata["debug_metadata"].(map[string]interface{}); ok {
						if endpoints, ok := debugMeta["api_endpoints"].([]interface{}); ok {
							for _, ep := range endpoints {
								if epMap, ok := ep.(map[string]interface{}); ok {
									if endpoint, ok := epMap["endpoint"].(string); ok {
										// Extract base domain from endpoint URL
										domain := extractBaseDomain(endpoint)
										if domain != "" {
											sources = append(sources, sourceDocInfo{
												Title:          domain,
												SourceType:     "api",
												SourceCategory: "data",
												URL:            "https://" + domain,
											})
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Also add the main body document itself
	if sourceDoc.ID != "" {
		sources = append(sources, sourceDocInfo{
			ID:             sourceDoc.ID,
			Title:          sourceDoc.Title,
			SourceType:     "portfolio-summary",
			SourceCategory: "local",
		})
	}

	// Find web search documents and extract their sources
	for _, ticker := range tickerTags {
		opts := interfaces.SearchOptions{
			Tags:     []string{"web-search", ticker},
			Limit:    5,
			OrderBy:  "created_at",
			OrderDir: "desc",
		}
		results, err := w.searchService.Search(ctx, "", opts)
		if err != nil {
			w.logger.Debug().Err(err).Str("ticker", ticker).Msg("Error searching for web search documents")
			continue
		}

		for _, doc := range results {
			// Extract web search sources from document metadata
			if doc.Metadata != nil {
				if webSources, ok := doc.Metadata["sources"].([]interface{}); ok {
					for _, ws := range webSources {
						if wsMap, ok := ws.(map[string]interface{}); ok {
							url, _ := wsMap["url"].(string)
							title, _ := wsMap["title"].(string)
							if url != "" {
								if title == "" {
									title = extractBaseDomain(url)
								}
								sources = append(sources, sourceDocInfo{
									Title:          title,
									SourceType:     "web-search",
									SourceCategory: "web",
									URL:            url,
								})
							}
						}
					}
				}
			}
		}
	}

	// Deduplicate data and web sources
	sources = deduplicateDataSources(sources)

	return sources
}

// extractBaseDomain extracts the base domain from a URL
func extractBaseDomain(urlStr string) string {
	// Handle URLs with or without scheme
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "https://" + urlStr
	}

	// Find the host part
	start := strings.Index(urlStr, "://")
	if start == -1 {
		return ""
	}
	rest := urlStr[start+3:]

	// Find end of host (before path, query, or fragment)
	end := len(rest)
	for _, sep := range []string{"/", "?", "#"} {
		if idx := strings.Index(rest, sep); idx != -1 && idx < end {
			end = idx
		}
	}

	host := rest[:end]

	// Remove port if present
	if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
		host = host[:colonIdx]
	}

	return host
}

// deduplicateDataSources removes duplicate data/web sources by URL
func deduplicateDataSources(sources []sourceDocInfo) []sourceDocInfo {
	seen := make(map[string]bool)
	result := make([]sourceDocInfo, 0, len(sources))

	for _, src := range sources {
		// Always keep local sources
		if src.SourceCategory == "local" {
			result = append(result, src)
			continue
		}

		// Deduplicate data/web sources by URL or title
		key := src.URL
		if key == "" {
			key = src.Title
		}
		if !seen[key] {
			seen[key] = true
			result = append(result, src)
		}
	}

	return result
}

// formatSourceLinksHTML creates an HTML section with clickable links to source documents
// Sources are grouped by category: Local (Quaero documents), Data (API sources), Web (search results)
func (w *EmailWorker) formatSourceLinksHTML(sources []sourceDocInfo, baseURL string) string {
	if len(sources) == 0 {
		return ""
	}

	// Group sources by category
	categoryGroups := make(map[string][]sourceDocInfo)
	for _, src := range sources {
		cat := src.SourceCategory
		if cat == "" {
			cat = "local" // Default to local
		}
		categoryGroups[cat] = append(categoryGroups[cat], src)
	}

	var sb strings.Builder
	sb.WriteString(`<div style="margin-top: 30px; padding-top: 20px; border-top: 2px solid #e0e0e0;">`)
	sb.WriteString(`<h3 style="color: #666; font-size: 14px; margin-bottom: 12px;">📎 Sources</h3>`)

	// Category order and labels
	categoryOrder := []struct {
		key   string
		label string
		desc  string
	}{
		{"local", "📁 Local", "Quaero documents"},
		{"data", "📊 Data", "External API sources"},
		{"web", "🌐 Web", "Web search results"},
	}

	for _, cat := range categoryOrder {
		srcs := categoryGroups[cat.key]
		if len(srcs) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf(`<div style="margin-bottom: 12px;"><strong style="color: #444; font-size: 13px;">%s</strong>`, cat.label))
		sb.WriteString(fmt.Sprintf(`<span style="color: #888; font-size: 11px; margin-left: 8px;">(%s)</span>`, cat.desc))
		sb.WriteString(`<ul style="list-style: none; padding: 0; margin: 4px 0 0 0;">`)

		if cat.key == "local" {
			// Group local sources by type for better organization
			typeGroups := make(map[string][]sourceDocInfo)
			typeOrder := []string{"asx-stock-data", "stock-recommendation", "asx-announcement-summary", "portfolio-summary"}
			for _, src := range srcs {
				typeGroups[src.SourceType] = append(typeGroups[src.SourceType], src)
			}

			for _, srcType := range typeOrder {
				docs := typeGroups[srcType]
				if len(docs) == 0 {
					continue
				}

				typeLabel := srcType
				switch srcType {
				case "asx-stock-data":
					typeLabel = "Stock Data"
				case "stock-recommendation":
					typeLabel = "Analysis & Recommendations"
				case "asx-announcement-summary":
					typeLabel = "ASX Announcements"
				case "portfolio-summary":
					typeLabel = "Portfolio Summary"
				}

				sb.WriteString(fmt.Sprintf(`<li style="margin: 6px 0;"><span style="color: #555;">%s:</span> `, typeLabel))
				for i, doc := range docs {
					if i > 0 {
						sb.WriteString(", ")
					}
					link := fmt.Sprintf("%s/documents?document_id=%s", baseURL, doc.ID)
					sb.WriteString(fmt.Sprintf(`<a href="%s" style="color: #0066cc;">%s</a>`, link, doc.Title))
				}
				sb.WriteString(`</li>`)
			}
		} else {
			// For data/web sources, just list them
			for _, src := range srcs {
				link := src.URL
				if link == "" {
					continue
				}
				sb.WriteString(fmt.Sprintf(`<li style="margin: 4px 0;"><a href="%s" style="color: #0066cc;">%s</a></li>`, link, src.Title))
			}
		}

		sb.WriteString(`</ul></div>`)
	}

	sb.WriteString(`</div>`)
	return sb.String()
}

// saveEmailToDir saves email content to a directory for archiving and audit purposes.
// Creates both markdown (.md) and HTML (.html) versions of the email content.
// Directory structure: save_dir/YYYY-MM-DD/job-name_timestamp.md
func (w *EmailWorker) saveEmailToDir(saveDir, stepID, subject, textBody, htmlBody, to string, jobDef *models.JobDefinition) error {
	// Create date-based subdirectory
	dateDir := time.Now().Format("2006-01-02")
	fullDir := filepath.Join(saveDir, dateDir)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(fullDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", fullDir, err)
	}

	// Build filename from job name and timestamp
	timestamp := time.Now().Format("15-04-05")
	safeName := "email"
	if jobDef != nil && jobDef.Name != "" {
		safeName = strings.ReplaceAll(jobDef.Name, " ", "-")
		safeName = strings.ReplaceAll(safeName, "/", "-")
		safeName = strings.ReplaceAll(safeName, ":", "-")
		safeName = strings.ToLower(safeName)
	}

	// Save markdown version (primary content)
	mdFilename := fmt.Sprintf("%s_%s.md", safeName, timestamp)
	mdPath := filepath.Join(fullDir, mdFilename)

	mdContent := fmt.Sprintf(`# %s

**To:** %s
**Date:** %s
**Step ID:** %s

---

%s
`, subject, to, time.Now().Format("January 2, 2006 at 15:04:05"), stepID, textBody)

	if err := os.WriteFile(mdPath, []byte(mdContent), 0644); err != nil {
		return fmt.Errorf("failed to write markdown file: %w", err)
	}

	w.logger.Info().
		Str("path", mdPath).
		Int("bytes", len(mdContent)).
		Msg("Saved email markdown to file")

	// Save HTML version if available
	if htmlBody != "" {
		htmlFilename := fmt.Sprintf("%s_%s.html", safeName, timestamp)
		htmlPath := filepath.Join(fullDir, htmlFilename)

		if err := os.WriteFile(htmlPath, []byte(htmlBody), 0644); err != nil {
			w.logger.Warn().Err(err).Str("path", htmlPath).Msg("Failed to save HTML version")
			// Don't fail the whole operation if HTML save fails
		} else {
			w.logger.Info().
				Str("path", htmlPath).
				Int("bytes", len(htmlBody)).
				Msg("Saved email HTML to file")
		}
	}

	return nil
}
