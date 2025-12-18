// -----------------------------------------------------------------------
// EmailWorker - Sends email notifications with job results
// Used as a step in job definitions to email results/summaries to users
// -----------------------------------------------------------------------

package workers

import (
	"bytes"
	"context"
	"fmt"

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
) *EmailWorker {
	return &EmailWorker{
		mailerService:   mailerService,
		documentStorage: documentStorage,
		searchService:   searchService,
		logger:          logger,
		jobMgr:          jobMgr,
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

	// Get body - can be from step config, or from previous step's output document
	body, htmlBody := w.resolveBody(ctx, stepConfig)

	if body == "" && htmlBody == "" {
		body = "Job completed. No content was specified for this email."
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
	var err error
	if htmlBody != "" {
		err = w.mailerService.SendHTMLEmail(ctx, to, subject, htmlBody, body)
	} else {
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

// resolveBody determines the email body from step configuration
// Supports: body (direct text), body_html (HTML), body_from_document (document ID), body_from_tag (latest document with tag)
func (w *EmailWorker) resolveBody(ctx context.Context, stepConfig map[string]interface{}) (textBody, htmlBody string) {
	// Direct body text
	if body, ok := stepConfig["body"].(string); ok && body != "" {
		textBody = body
	}

	// Direct HTML body
	if html, ok := stepConfig["body_html"].(string); ok && html != "" {
		htmlBody = html
	}

	// Body from document ID
	if docID, ok := stepConfig["body_from_document"].(string); ok && docID != "" {
		if doc, err := w.documentStorage.GetDocument(docID); err == nil && doc != nil {
			if doc.ContentMarkdown != "" {
				textBody = doc.ContentMarkdown
				// Convert markdown to HTML for rich email formatting
				htmlBody = w.convertMarkdownToHTML(doc.ContentMarkdown)
			}
		} else {
			w.logger.Warn().Str("document_id", docID).Err(err).Msg("Failed to load document for email body")
		}
	}

	// Body from tag (get latest document with tag)
	if tag, ok := stepConfig["body_from_tag"].(string); ok && tag != "" {
		opts := interfaces.SearchOptions{
			Tags:  []string{tag},
			Limit: 1,
		}
		results, err := w.searchService.Search(ctx, "", opts)
		if err == nil && len(results) > 0 {
			if doc, err := w.documentStorage.GetDocument(results[0].ID); err == nil && doc != nil {
				if doc.ContentMarkdown != "" {
					textBody = doc.ContentMarkdown
					// Convert markdown to HTML for rich email formatting
					htmlBody = w.convertMarkdownToHTML(doc.ContentMarkdown)
				}
			}
		} else {
			w.logger.Warn().Str("tag", tag).Err(err).Msg("Failed to find document by tag for email body")
		}
	}

	return textBody, htmlBody
}

// convertMarkdownToHTML converts markdown content to styled HTML for email
func (w *EmailWorker) convertMarkdownToHTML(markdown string) string {
	if markdown == "" {
		return ""
	}

	// Create goldmark instance with common extensions
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM, // GitHub Flavored Markdown (tables, strikethrough, etc.)
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
		),
	)

	var buf bytes.Buffer
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to convert markdown to HTML, using plain text")
		return ""
	}

	// Wrap in styled HTML email template
	return w.wrapInEmailTemplate(buf.String())
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
