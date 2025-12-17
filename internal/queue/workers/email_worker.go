// -----------------------------------------------------------------------
// EmailWorker - Sends email notifications with job results
// Used as a step in job definitions to email results/summaries to users
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"fmt"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/services/mailer"
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
			}
			// Check if content looks like HTML
			if strings.Contains(textBody, "<html") || strings.Contains(textBody, "<body") || strings.Contains(textBody, "<p>") {
				htmlBody = textBody
				textBody = "" // Will be derived from HTML or empty
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
				}
				// Check if content looks like HTML
				if strings.Contains(textBody, "<html") || strings.Contains(textBody, "<body") || strings.Contains(textBody, "<p>") {
					htmlBody = textBody
					textBody = ""
				}
			}
		} else {
			w.logger.Warn().Str("tag", tag).Err(err).Msg("Failed to find document by tag for email body")
		}
	}

	return textBody, htmlBody
}
