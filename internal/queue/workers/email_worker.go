// -----------------------------------------------------------------------
// EmailWorker - Sends email notifications with job results
// Used as a step in job definitions to email results/summaries to users
// -----------------------------------------------------------------------

package workers

import (
	"bytes"
	"context"
	"fmt"
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

// resolveBody determines the email body from step configuration
// Supports: body (direct text/markdown), body_html (HTML), body_from_document (document ID), body_from_tag (latest document with tag)
// All text content is converted to HTML for proper email presentation
func (w *EmailWorker) resolveBody(ctx context.Context, stepConfig map[string]interface{}) (textBody, htmlBody string) {
	// Direct body text (markdown is converted to HTML)
	if body, ok := stepConfig["body"].(string); ok && body != "" {
		textBody = body
		// Convert markdown to HTML for rich email formatting
		htmlBody = w.convertMarkdownToHTML(body)
	}

	// Direct HTML body (overrides conversion from body)
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
					textBody = doc.ContentMarkdown
					// Convert markdown to HTML for rich email formatting
					htmlBody = w.convertMarkdownToHTML(doc.ContentMarkdown)
					w.logger.Debug().Int("html_len", len(htmlBody)).Msg("HTML body after conversion")
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

	return textBody, htmlBody
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
