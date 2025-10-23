package types

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/queue"
)

// CleanupJobDeps holds dependencies for cleanup jobs
type CleanupJobDeps struct {
	JobStorage interfaces.JobStorage
	LogService interfaces.LogService
}

// CleanupJob handles job and log cleanup operations
type CleanupJob struct {
	*BaseJob
	deps *CleanupJobDeps
}

// NewCleanupJob creates a new cleanup job
func NewCleanupJob(base *BaseJob, deps *CleanupJobDeps) *CleanupJob {
	return &CleanupJob{
		BaseJob: base,
		deps:    deps,
	}
}

// Execute processes a cleanup job
func (c *CleanupJob) Execute(ctx context.Context, msg *queue.JobMessage) error {
	c.logger.Info().
		Str("message_id", msg.ID).
		Msg("Processing cleanup job")

	// Validate message
	if err := c.Validate(msg); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	// Extract cleanup criteria from config
	ageThreshold := 30 // Default: 30 days
	if age, ok := msg.Config["age_threshold_days"].(float64); ok {
		ageThreshold = int(age)
	}

	// Enforce minimum age threshold for safety (7 days to prevent accidental deletion of recent jobs)
	minAge := 7
	if ageThreshold < minAge {
		c.logger.Warn().
			Int("requested_age", ageThreshold).
			Int("enforced_age", minAge).
			Msg("Age threshold too low, enforcing minimum")
		ageThreshold = minAge
	}

	statusFilter := "completed" // Default: only clean completed jobs
	if status, ok := msg.Config["status_filter"].(string); ok {
		statusFilter = status
	}

	dryRun := false // Default: actually delete
	if dry, ok := msg.Config["dry_run"].(bool); ok {
		dryRun = dry
	}

	// Log job start
	if err := c.LogJobEvent(ctx, msg.ParentID, "info",
		fmt.Sprintf("Starting cleanup: age_threshold=%d days, status=%s, dry_run=%v",
			ageThreshold, statusFilter, dryRun)); err != nil {
		c.logger.Warn().Err(err).Msg("Failed to log job start event")
	}

	c.logger.Info().
		Int("age_threshold_days", ageThreshold).
		Str("status_filter", statusFilter).
		Bool("dry_run", dryRun).
		Msg("Starting cleanup with criteria")

	// Calculate cleanup cutoff time
	cleanupTime := time.Now().Add(-time.Duration(ageThreshold) * 24 * time.Hour)
	c.logger.Info().
		Str("cleanup_before", cleanupTime.Format(time.RFC3339)).
		Msg("Cleanup cutoff time calculated")

	// Query jobs matching criteria
	// Build filter for status
	var statuses []string
	if statusFilter == "all" {
		statuses = []string{"completed", "failed", "cancelled"}
	} else {
		statuses = []string{statusFilter}
	}

	c.logger.Info().
		Str("cutoff", cleanupTime.Format(time.RFC3339)).
		Strs("statuses", statuses).
		Msg("Querying jobs for cleanup")

	// Query and collect eligible job IDs across all statuses
	jobsToClean := []string{}
	for _, status := range statuses {
		opts := &interfaces.ListOptions{
			Status:  status,
			Limit:   100, // Process in batches
			Offset:  0,
			OrderBy: "updated_at",
		}

		for {
			jobs, err := c.deps.JobStorage.ListJobs(ctx, opts)
			if err != nil {
				c.logger.Error().
					Err(err).
					Str("status", status).
					Msg("Failed to list jobs")
				break // Continue with other statuses
			}

			if len(jobs) == 0 {
				break
			}

			// Filter jobs by age
			for _, jobInterface := range jobs {
				// Type assert to access job fields
				// Jobs are stored as interface{}, need to access via reflection or type assertion
				// Since we don't know the exact type, we'll use type assertion to a map
				if jobMap, ok := jobInterface.(map[string]interface{}); ok {
					// Extract updated_at timestamp
					var updatedAt time.Time
					if updatedAtStr, ok := jobMap["updated_at"].(string); ok {
						if parsed, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
							updatedAt = parsed
						}
					} else if updatedAtTime, ok := jobMap["updated_at"].(time.Time); ok {
						updatedAt = updatedAtTime
					}

					// Check if job is old enough
					if !updatedAt.IsZero() && updatedAt.Before(cleanupTime) {
						if jobID, ok := jobMap["id"].(string); ok && jobID != "" {
							jobsToClean = append(jobsToClean, jobID)
						}
					}
				}
			}

			opts.Offset += opts.Limit
		}
	}

	c.logger.Info().
		Int("jobs_found", len(jobsToClean)).
		Bool("dry_run", dryRun).
		Msg("Jobs identified for cleanup")

	logsDeleted := 0
	jobsDeleted := 0

	// Perform cleanup if not dry run
	if !dryRun && len(jobsToClean) > 0 {
		c.logger.Info().
			Int("jobs_to_clean", len(jobsToClean)).
			Msg("Starting job deletion")

		for i, jobID := range jobsToClean {
			// Delete job (logs are cascade deleted with the job)
			if err := c.deps.JobStorage.DeleteJob(ctx, jobID); err != nil {
				c.logger.Error().
					Err(err).
					Str("job_id", jobID).
					Msg("Failed to delete job")
				continue
			}

			jobsDeleted++
			logsDeleted++ // Assume logs deleted with job

			// Log progress every 10 deletions
			if (i+1)%10 == 0 {
				c.logger.Info().
					Int("progress", i+1).
					Int("total", len(jobsToClean)).
					Msg("Deletion progress")
			}
		}

		c.logger.Info().
			Int("jobs_deleted", jobsDeleted).
			Msg("Job deletion completed")
	} else if dryRun {
		c.logger.Info().
			Int("jobs_found", len(jobsToClean)).
			Msg("Dry run mode - no actual deletion performed")
	}

	// Log cleanup summary
	summaryMsg := fmt.Sprintf("Cleanup completed: jobs_deleted=%d, logs_deleted=%d, dry_run=%v",
		jobsDeleted, logsDeleted, dryRun)

	if err := c.LogJobEvent(ctx, msg.ParentID, "info", summaryMsg); err != nil {
		c.logger.Warn().Err(err).Msg("Failed to log job completion event")
	}

	c.logger.Info().
		Str("message_id", msg.ID).
		Int("jobs_deleted", jobsDeleted).
		Int("logs_deleted", logsDeleted).
		Bool("dry_run", dryRun).
		Msg("Cleanup job completed successfully")

	return nil
}

// Validate validates the cleanup message
func (c *CleanupJob) Validate(msg *queue.JobMessage) error {
	if msg.Config == nil {
		return fmt.Errorf("config is required")
	}

	// Validate ParentID is present (required for logging)
	if msg.ParentID == "" {
		return fmt.Errorf("parent_id is required for logging job events")
	}

	// Validate age threshold if present
	if age, ok := msg.Config["age_threshold_days"].(float64); ok {
		if age < 0 {
			return fmt.Errorf("age_threshold_days must be non-negative, got: %v", age)
		}
	}

	// Validate status filter if present
	if status, ok := msg.Config["status_filter"].(string); ok {
		validStatuses := map[string]bool{
			"completed": true,
			"failed":    true,
			"cancelled": true,
			"all":       true, // Special value to clean all terminal states
		}
		if !validStatuses[status] {
			return fmt.Errorf("invalid status_filter: %s (must be completed, failed, cancelled, or all)", status)
		}

		// Safety check: prevent cleaning running jobs
		if status == "running" || status == "pending" {
			return fmt.Errorf("cannot clean jobs with status: %s (running and pending jobs cannot be cleaned)", status)
		}
	}

	return nil
}

// GetType returns the job type
func (c *CleanupJob) GetType() string {
	return "cleanup"
}
