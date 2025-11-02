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
	JobManager interfaces.JobManager // Changed from JobStorage to use cascade deletion logic
	LogService interfaces.LogService
	// NOTE: CleanupJob uses JobManager instead of JobStorage for deletion.
	// This is intentional because:
	//   1. JobManager.DeleteJob() handles cascade deletion of children
	//   2. JobManager.DeleteJob() validates business rules (e.g., cannot delete running jobs)
	//   3. JobStorage.DeleteJob() is a low-level operation without validation
	//
	// Using JobManager ensures cleanup jobs follow the same deletion logic as manual deletions.
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
	startTime := time.Now()

	// Validate message
	if err := c.Validate(msg); err != nil {
		c.logger.LogJobError(err, fmt.Sprintf("Validation failed for age_threshold=%v, status_filter=%v", msg.Config["age_threshold_days"], msg.Config["status_filter"]))
		// Note: CleanupJob doesn't have JobStorage dependency to update status
		// Status update would require adding JobStorage to CleanupJobDeps
		return fmt.Errorf("invalid message: %w", err)
	}

	// TODO: Add JobStorage to CleanupJobDeps to enable status updates on validation failure
	// This would allow consistent error handling across all job types

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

	// Log job start using JobLogger
	c.logger.LogJobStart(
		fmt.Sprintf("Cleanup: age_threshold=%d days, status=%s, dry_run=%v", ageThreshold, statusFilter, dryRun),
		"maintenance",
		msg.Config,
	)

	// Calculate cleanup cutoff time
	cleanupTime := time.Now().Add(-time.Duration(ageThreshold) * 24 * time.Hour)

	// Query jobs matching criteria
	// Build filter for status
	var statuses []string
	if statusFilter == "all" {
		statuses = []string{"completed", "failed", "cancelled"}
	} else {
		statuses = []string{statusFilter}
	}

	// Query and collect eligible job IDs across all statuses
	jobsToClean := []string{}
	for _, status := range statuses {
		opts := &interfaces.JobListOptions{
			Status:  status,
			Limit:   100, // Process in batches
			Offset:  0,
			OrderBy: "updated_at",
		}

		for {
			jobs, err := c.deps.JobManager.ListJobs(ctx, opts)
			if err != nil {
				c.logger.LogJobError(err, fmt.Sprintf("Failed to list jobs: status=%s", status))
				break // Continue with other statuses
			}

			if len(jobs) == 0 {
				break
			}

			// Filter jobs by age
			for _, job := range jobs {
				// Access job fields directly (jobs are already typed as []*models.CrawlJob)
				// Use CompletedAt/StartedAt/CreatedAt as the timestamp to check
				// Determine which timestamp to check
				var checkTime time.Time
				if !job.CompletedAt.IsZero() {
					checkTime = job.CompletedAt
				} else if !job.StartedAt.IsZero() {
					checkTime = job.StartedAt
				} else {
					checkTime = job.CreatedAt
				}

				// Check if job is old enough
				if !checkTime.IsZero() && checkTime.Before(cleanupTime) {
					if job.ID != "" {
						jobsToClean = append(jobsToClean, job.ID)
					}
				}
			}

			opts.Offset += opts.Limit
		}
	}

	c.logger.LogJobProgress(len(jobsToClean), len(jobsToClean), fmt.Sprintf("Found %d jobs to clean", len(jobsToClean)))

	logsDeleted := 0
	jobsDeleted := 0

	// Perform cleanup if not dry run
	if !dryRun && len(jobsToClean) > 0 {
		for i, jobID := range jobsToClean {
			// Delete job (children and logs are cascade deleted with the job)
			if _, err := c.deps.JobManager.DeleteJob(ctx, jobID); err != nil {
				c.logger.LogJobError(err, fmt.Sprintf("Failed to delete job: status=%s, job_id=%s", statusFilter, jobID))
				continue
			}

			jobsDeleted++
			logsDeleted++ // Assume logs deleted with job

			// Log progress
			c.logger.LogJobProgress(i+1, len(jobsToClean), fmt.Sprintf("Deleted %d/%d jobs", i+1, len(jobsToClean)))
		}

		c.logger.Info().
			Int("jobs_deleted", jobsDeleted).
			Int("logs_deleted", logsDeleted).
			Msg("Cleanup completed")
	}

	// Log cleanup summary using JobLogger
	c.logger.LogJobComplete(
		time.Since(startTime),
		jobsDeleted,
	)

	return nil
}

// Validate validates the cleanup message
func (c *CleanupJob) Validate(msg *queue.JobMessage) error {
	if msg.Config == nil {
		return fmt.Errorf("config is required")
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
