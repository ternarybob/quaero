// -----------------------------------------------------------------------
// EmailWatcherWorker - Monitors email inbox for job execution commands
// Reads IMAP emails with subject containing 'quaero' and executes named jobs
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/services/imap"
)

// EmailWatcherWorker monitors email inbox for job execution commands
type EmailWatcherWorker struct {
	imapService   *imap.Service
	jobDefStorage interfaces.JobDefinitionStorage
	orchestrator  *queue.Orchestrator
	logger        arbor.ILogger
	jobMgr        *queue.Manager
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*EmailWatcherWorker)(nil)

// NewEmailWatcherWorker creates a new email watcher worker
func NewEmailWatcherWorker(
	imapService *imap.Service,
	jobDefStorage interfaces.JobDefinitionStorage,
	orchestrator *queue.Orchestrator,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *EmailWatcherWorker {
	return &EmailWatcherWorker{
		imapService:   imapService,
		jobDefStorage: jobDefStorage,
		orchestrator:  orchestrator,
		logger:        logger,
		jobMgr:        jobMgr,
	}
}

// GetType returns WorkerTypeEmailWatcher
func (w *EmailWatcherWorker) GetType() models.WorkerType {
	return models.WorkerTypeEmailWatcher
}

// Init initializes the email watcher worker
func (w *EmailWatcherWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	// Check if IMAP is configured
	if !w.imapService.IsConfigured(ctx) {
		return nil, fmt.Errorf("IMAP is not configured. Please configure IMAP settings in Settings")
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Msg("Email watcher worker initialized")

	// Single work item - check emails
	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:   "email_check",
				Name: "Check emails for job execution commands",
				Type: "email_watcher",
			},
		},
		TotalCount:           1,
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
	}, nil
}

// CreateJobs checks emails and executes jobs inline
func (w *EmailWatcherWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize email watcher worker: %w", err)
		}
	}

	w.logger.Info().
		Str("phase", "run").
		Str("step_name", step.Name).
		Msg("Checking emails for job execution commands")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", "Checking IMAP inbox for emails with subject 'quaero'")
	}

	// Fetch unread emails with subject containing 'quaero'
	emails, err := w.imapService.FetchUnreadEmails(ctx, "quaero")
	if err != nil {
		w.logger.Error().Err(err).Msg("Failed to fetch emails")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed to fetch emails: %v", err))
		}
		return "", fmt.Errorf("failed to fetch emails: %w", err)
	}

	if len(emails) == 0 {
		w.logger.Debug().Msg("No unread emails with subject 'quaero'")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "info", "No unread emails found with subject 'quaero'")
		}
		return stepID, nil
	}

	w.logger.Info().Int("count", len(emails)).Msg("Found unread emails with subject 'quaero'")
	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Found %d unread email(s) with subject 'quaero'", len(emails)))
	}

	// Process each email
	for _, email := range emails {
		w.logger.Debug().
			Str("from", email.From).
			Str("subject", email.Subject).
			Msg("Processing email")

		// Extract job name from email body
		jobName := extractJobName(email.Body)
		if jobName == "" {
			w.logger.Warn().
				Str("from", email.From).
				Str("subject", email.Subject).
				Msg("Email body does not contain valid job name format")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "warn", fmt.Sprintf("Email from %s has no valid job name (expected format: 'execute: <job-name>')", email.From))
			}

			// Mark as read even if invalid
			if err := w.imapService.MarkAsRead(ctx, email.ID); err != nil {
				w.logger.Warn().Err(err).Int("email_id", int(email.ID)).Msg("Failed to mark email as read")
			}
			continue
		}

		w.logger.Info().
			Str("from", email.From).
			Str("job_name", jobName).
			Msg("Extracted job name from email")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Email from %s requests execution of job: %s", email.From, jobName))
		}

		// Look up job definition by name
		jobDefToExecute, err := w.findJobDefinitionByName(ctx, jobName)
		if err != nil {
			w.logger.Error().
				Err(err).
				Str("job_name", jobName).
				Msg("Failed to find job definition")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Job '%s' not found: %v", jobName, err))
			}

			// Mark as read
			if err := w.imapService.MarkAsRead(ctx, email.ID); err != nil {
				w.logger.Warn().Err(err).Int("email_id", int(email.ID)).Msg("Failed to mark email as read")
			}
			continue
		}

		w.logger.Info().
			Str("job_name", jobName).
			Str("job_id", jobDefToExecute.ID).
			Msg("Found job definition, executing")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Executing job definition: %s (ID: %s)", jobDefToExecute.Name, jobDefToExecute.ID))
		}

		// Execute job definition
		managerID, err := w.orchestrator.ExecuteJobDefinition(ctx, jobDefToExecute, nil, nil)
		if err != nil {
			w.logger.Error().
				Err(err).
				Str("job_name", jobName).
				Msg("Failed to execute job definition")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed to execute job '%s': %v", jobName, err))
			}

			// Mark as read even if execution failed
			if err := w.imapService.MarkAsRead(ctx, email.ID); err != nil {
				w.logger.Warn().Err(err).Int("email_id", int(email.ID)).Msg("Failed to mark email as read")
			}
			continue
		}

		w.logger.Info().
			Str("job_name", jobName).
			Str("manager_id", managerID).
			Msg("Job execution started successfully")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Job '%s' started successfully (Manager ID: %s)", jobName, managerID))
		}

		// Mark email as read
		if err := w.imapService.MarkAsRead(ctx, email.ID); err != nil {
			w.logger.Warn().Err(err).Int("email_id", int(email.ID)).Msg("Failed to mark email as read")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "warn", fmt.Sprintf("Failed to mark email as read: %v", err))
			}
		} else {
			w.logger.Debug().Int("email_id", int(email.ID)).Msg("Marked email as read")
		}
	}

	w.logger.Info().Int("processed", len(emails)).Msg("Email processing complete")
	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Processed %d email(s)", len(emails)))
	}

	return stepID, nil
}

// ReturnsChildJobs returns false since email watcher executes inline
func (w *EmailWatcherWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration
func (w *EmailWatcherWorker) ValidateConfig(step models.JobStep) error {
	// No specific config required for email watcher
	return nil
}

// extractJobName extracts job name from email body
// Expected format: "execute: <job-name>" (case-insensitive)
func extractJobName(body string) string {
	// Try different patterns
	patterns := []string{
		`(?i)execute:\s*([a-zA-Z0-9_-]+)`, // execute: job-name
		`(?i)run:\s*([a-zA-Z0-9_-]+)`,     // run: job-name
		`(?i)job:\s*([a-zA-Z0-9_-]+)`,     // job: job-name
		`(?i)trigger:\s*([a-zA-Z0-9_-]+)`, // trigger: job-name
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(body)
		if len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}

	return ""
}

// findJobDefinitionByName finds a job definition by name
func (w *EmailWatcherWorker) findJobDefinitionByName(ctx context.Context, name string) (*models.JobDefinition, error) {
	// List all job definitions
	jobDefs, err := w.jobDefStorage.ListJobDefinitions(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list job definitions: %w", err)
	}

	// Find by name (case-insensitive)
	nameLower := strings.ToLower(name)
	for _, jobDef := range jobDefs {
		if strings.ToLower(jobDef.Name) == nameLower {
			return jobDef, nil
		}
	}

	return nil, fmt.Errorf("job definition '%s' not found", name)
}
