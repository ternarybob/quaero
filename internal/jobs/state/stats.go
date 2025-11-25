package state

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// JobTreeStatus represents aggregated status for a job tree (parent + children)
type JobTreeStatus struct {
	ParentJob       interface{} `json:"parent_job"` // This will be *queue.Job in practice
	TotalChildren   int         `json:"total_children"`
	CompletedCount  int         `json:"completed_count"`
	FailedCount     int         `json:"failed_count"`
	RunningCount    int         `json:"running_count"`
	PendingCount    int         `json:"pending_count"`
	CancelledCount  int         `json:"cancelled_count"`
	OverallProgress float64     `json:"overall_progress"`            // 0.0 to 1.0
	EstimatedTime   *int64      `json:"estimated_time_ms,omitempty"` // Estimated milliseconds to completion
}

// GetJobTreeStatus retrieves aggregated status for a parent job and all its children
// This provides efficient status reporting for hierarchical job execution
// NOTE: This method needs to access queue.Job but to avoid circular imports,
// it should be called from the queue package with the parent job passed in
func (m *Manager) GetJobTreeStatus(ctx context.Context, parentJobID string, getJobFunc func(context.Context, string) (interface{}, error)) (*JobTreeStatus, error) {
	// Get parent job using provided function
	parentJobInternal, err := getJobFunc(ctx, parentJobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent job: %w", err)
	}

	// Extract progress info from parent job
	var progressCurrent, progressTotal int
	var startedAt *time.Time

	// Type assert to access fields (this is safe as getJobFunc returns queue.Job)
	if job, ok := parentJobInternal.(interface {
		GetProgressCurrent() int
		GetProgressTotal() int
		GetStartedAt() *time.Time
	}); ok {
		progressCurrent = job.GetProgressCurrent()
		progressTotal = job.GetProgressTotal()
		startedAt = job.GetStartedAt()
	}

	// Aggregate child job statuses
	childStats, err := m.jobStorage.GetJobChildStats(ctx, []string{parentJobID})
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate child statuses: %w", err)
	}

	stats := childStats[parentJobID]
	if stats == nil {
		stats = &interfaces.JobChildStats{}
	}

	// Calculate overall progress
	// Progress based on completed + failed (terminal states) vs total
	var overallProgress float64
	if stats.ChildCount > 0 {
		terminalCount := stats.CompletedChildren + stats.FailedChildren + stats.CancelledChildren
		overallProgress = float64(terminalCount) / float64(stats.ChildCount)
	} else {
		// No children yet, use parent job progress if available
		if progressTotal > 0 {
			overallProgress = float64(progressCurrent) / float64(progressTotal)
		}
	}

	// Estimate time to completion (simple linear extrapolation)
	var estimatedTime *int64
	if stats.RunningChildren > 0 && startedAt != nil {
		elapsed := time.Since(*startedAt)
		if overallProgress > 0 && overallProgress < 1.0 {
			totalEstimated := float64(elapsed) / overallProgress
			remaining := totalEstimated - float64(elapsed)
			remainingMS := int64(time.Duration(remaining) / time.Millisecond)
			estimatedTime = &remainingMS
		}
	}

	status := &JobTreeStatus{
		ParentJob:       parentJobInternal,
		TotalChildren:   stats.ChildCount,
		CompletedCount:  stats.CompletedChildren,
		FailedCount:     stats.FailedChildren,
		RunningCount:    stats.RunningChildren,
		PendingCount:    stats.PendingChildren,
		CancelledCount:  stats.CancelledChildren,
		OverallProgress: overallProgress,
		EstimatedTime:   estimatedTime,
	}

	return status, nil
}

// GetFailedChildCount returns the count of failed child jobs for a parent job
func (m *Manager) GetFailedChildCount(ctx context.Context, parentJobID string) (int, error) {
	childStats, err := m.jobStorage.GetJobChildStats(ctx, []string{parentJobID})
	if err != nil {
		return 0, err
	}

	if stats, ok := childStats[parentJobID]; ok {
		return stats.FailedChildren, nil
	}

	return 0, nil
}

// CrawlerProgressStats represents comprehensive progress statistics for crawler jobs
type CrawlerProgressStats struct {
	// Basic job information
	JobID    string `json:"job_id"`
	ParentID string `json:"parent_id,omitempty"`
	Status   string `json:"status"`
	JobType  string `json:"job_type"`

	// Child job statistics
	TotalChildren     int `json:"total_children"`
	CompletedChildren int `json:"completed_children"`
	FailedChildren    int `json:"failed_children"`
	RunningChildren   int `json:"running_children"`
	PendingChildren   int `json:"pending_children"`
	CancelledChildren int `json:"cancelled_children"`

	// Progress calculation
	OverallProgress float64 `json:"overall_progress"` // 0.0 to 1.0
	ProgressText    string  `json:"progress_text"`    // Human-readable progress

	// Link following statistics (crawler-specific)
	LinksFound    int `json:"links_found"`
	LinksFiltered int `json:"links_filtered"`
	LinksFollowed int `json:"links_followed"`
	LinksSkipped  int `json:"links_skipped"`

	// Timing information
	StartedAt    *time.Time `json:"started_at,omitempty"`
	EstimatedEnd *time.Time `json:"estimated_end,omitempty"`
	Duration     *float64   `json:"duration_seconds,omitempty"`

	// Error information
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// GetCrawlerProgressStats retrieves comprehensive progress statistics for a crawler job
// This method calculates parent job progress from child job statistics and includes
// link following metrics for real-time monitoring
// NOTE: This needs queue.Job but to avoid circular imports, job data is passed via interface{}
func (m *Manager) GetCrawlerProgressStats(ctx context.Context, jobID string, getJobFunc func(context.Context, string) (interface{}, error)) (*CrawlerProgressStats, error) {
	// Get the job details
	jobInterface, err := getJobFunc(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	// Extract job fields (using type assertion with interface methods)
	stats := &CrawlerProgressStats{
		JobID: jobID,
	}

	// Type assert to extract job information
	if job, ok := jobInterface.(interface {
		GetID() string
		GetStatus() string
		GetType() string
		GetParentID() *string
		GetStartedAt() *time.Time
		GetCompletedAt() *time.Time
		GetError() *string
		GetProgressCurrent() int
		GetProgressTotal() int
	}); ok {
		stats.Status = job.GetStatus()
		stats.JobType = job.GetType()
		stats.StartedAt = job.GetStartedAt()

		if parentID := job.GetParentID(); parentID != nil {
			stats.ParentID = *parentID
		}

		// Calculate duration if job has started
		if job.GetStartedAt() != nil {
			var endTime time.Time
			if job.GetCompletedAt() != nil {
				endTime = *job.GetCompletedAt()
			} else {
				endTime = time.Now()
			}
			duration := endTime.Sub(*job.GetStartedAt()).Seconds()
			stats.Duration = &duration
		}

		// Extract errors and warnings
		if job.GetError() != nil && *job.GetError() != "" {
			stats.Errors = []string{*job.GetError()}
		}
	}

	// Get child job statistics
	childStats, err := m.getChildJobStatistics(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get child statistics: %w", err)
	}

	stats.TotalChildren = childStats.TotalChildren
	stats.CompletedChildren = childStats.CompletedChildren
	stats.FailedChildren = childStats.FailedChildren
	stats.RunningChildren = childStats.RunningChildren
	stats.PendingChildren = childStats.PendingChildren
	stats.CancelledChildren = childStats.CancelledChildren

	// Calculate overall progress
	if stats.TotalChildren > 0 {
		terminalCount := stats.CompletedChildren + stats.FailedChildren + stats.CancelledChildren
		stats.OverallProgress = float64(terminalCount) / float64(stats.TotalChildren)
	} else {
		// No children yet, use parent job progress if available
		if job, ok := jobInterface.(interface {
			GetProgressCurrent() int
			GetProgressTotal() int
		}); ok {
			if job.GetProgressTotal() > 0 {
				stats.OverallProgress = float64(job.GetProgressCurrent()) / float64(job.GetProgressTotal())
			}
		}
	}

	// Generate progress text
	stats.ProgressText = m.generateProgressText(stats)

	// Get link following statistics from crawler metadata
	linkStats, err := m.getLinkFollowingStats(ctx, jobID)
	if err == nil {
		stats.LinksFound = linkStats.LinksFound
		stats.LinksFiltered = linkStats.LinksFiltered
		stats.LinksFollowed = linkStats.LinksFollowed
		stats.LinksSkipped = linkStats.LinksSkipped
	}

	// Estimate completion time
	if stats.OverallProgress > 0 && stats.OverallProgress < 1.0 && stats.StartedAt != nil && stats.RunningChildren > 0 {
		elapsed := time.Since(*stats.StartedAt)
		totalEstimated := float64(elapsed) / stats.OverallProgress
		remaining := totalEstimated - float64(elapsed)
		estimatedEnd := time.Now().Add(time.Duration(remaining))
		stats.EstimatedEnd = &estimatedEnd
	}

	return stats, nil
}

// childJobStatistics holds detailed child job statistics
type childJobStatistics struct {
	TotalChildren     int
	CompletedChildren int
	FailedChildren    int
	RunningChildren   int
	PendingChildren   int
	CancelledChildren int
}

// getChildJobStatistics retrieves detailed child job statistics
func (m *Manager) getChildJobStatistics(ctx context.Context, parentJobID string) (*childJobStatistics, error) {
	statsMap, err := m.jobStorage.GetJobChildStats(ctx, []string{parentJobID})
	if err != nil {
		return nil, err
	}

	stats := &childJobStatistics{}
	if s, ok := statsMap[parentJobID]; ok {
		stats.TotalChildren = s.ChildCount
		stats.CompletedChildren = s.CompletedChildren
		stats.FailedChildren = s.FailedChildren
		stats.RunningChildren = s.RunningChildren
		stats.PendingChildren = s.PendingChildren
		stats.CancelledChildren = s.CancelledChildren
	}

	return stats, nil
}

// linkFollowingStats holds link following statistics
type linkFollowingStats struct {
	LinksFound    int
	LinksFiltered int
	LinksFollowed int
	LinksSkipped  int
}

// getLinkFollowingStats retrieves link following statistics from crawler metadata
// This aggregates link statistics across all child jobs for a parent crawler job
func (m *Manager) getLinkFollowingStats(ctx context.Context, jobID string) (*linkFollowingStats, error) {
	// For now, return empty stats as this would require parsing crawler metadata
	// In a full implementation, this would query the documents table or job metadata
	// to aggregate link statistics from all child jobs
	return &linkFollowingStats{}, nil
}

// generateProgressText creates human-readable progress text
func (m *Manager) generateProgressText(stats *CrawlerProgressStats) string {
	if stats.TotalChildren == 0 {
		return "No child jobs spawned yet"
	}

	return fmt.Sprintf("%d URLs (%d completed, %d failed, %d running, %d pending)",
		stats.TotalChildren,
		stats.CompletedChildren,
		stats.FailedChildren,
		stats.RunningChildren,
		stats.PendingChildren,
	)
}

// GetJobTreeProgressStats retrieves progress statistics for multiple parent jobs
// This is optimized for bulk operations when displaying multiple jobs in the UI
func (m *Manager) GetJobTreeProgressStats(ctx context.Context, parentJobIDs []string) (map[string]*CrawlerProgressStats, error) {
	if len(parentJobIDs) == 0 {
		return make(map[string]*CrawlerProgressStats), nil
	}

	result := make(map[string]*CrawlerProgressStats)

	// Get all parent jobs
	// We have to loop because ListJobs doesn't support IN clause for IDs
	// Or we can use ListJobs with no filter and filter in memory if list is small?
	// Better to loop GetJob for now or add GetJobs(ids) to interface.
	// Since interface is fixed, we loop.

	for _, id := range parentJobIDs {
		jobEntityInterface, err := m.jobStorage.GetJob(ctx, id)
		if err != nil {
			continue
		}
		jobState := jobEntityInterface.(*models.QueueJobState)

		stats := &CrawlerProgressStats{
			JobID:   jobState.ID,
			Status:  string(jobState.Status),
			JobType: jobState.Type,
		}

		if jobState.ParentID != nil {
			stats.ParentID = *jobState.ParentID
		}

		if jobState.StartedAt != nil {
			stats.StartedAt = jobState.StartedAt
		}

		if jobState.Error != "" {
			stats.Errors = []string{jobState.Error}
		}

		// Get child statistics for this parent
		childStats, err := m.getChildJobStatistics(ctx, id)
		if err == nil {
			stats.TotalChildren = childStats.TotalChildren
			stats.CompletedChildren = childStats.CompletedChildren
			stats.FailedChildren = childStats.FailedChildren
			stats.RunningChildren = childStats.RunningChildren
			stats.PendingChildren = childStats.PendingChildren
			stats.CancelledChildren = childStats.CancelledChildren

			// Calculate progress
			if stats.TotalChildren > 0 {
				terminalCount := stats.CompletedChildren + stats.FailedChildren + stats.CancelledChildren
				stats.OverallProgress = float64(terminalCount) / float64(stats.TotalChildren)
			}

			stats.ProgressText = m.generateProgressText(stats)
		}

		result[id] = stats
	}

	return result, nil
}

// ChildJobStats represents statistics for child jobs of a parent job
type ChildJobStats struct {
	TotalChildren     int `json:"total_children"`
	CompletedChildren int `json:"completed_children"`
	FailedChildren    int `json:"failed_children"`
	CancelledChildren int `json:"cancelled_children"`
	RunningChildren   int `json:"running_children"`
	PendingChildren   int `json:"pending_children"`
}

// GetChildJobStats retrieves child job statistics for a single parent job
// This is used by the JobMonitor to monitor child job progress
func (m *Manager) GetChildJobStats(ctx context.Context, parentJobID string) (*ChildJobStats, error) {
	statsMap, err := m.jobStorage.GetJobChildStats(ctx, []string{parentJobID})
	if err != nil {
		return nil, err
	}

	s := statsMap[parentJobID]
	if s == nil {
		return &ChildJobStats{}, nil
	}

	return &ChildJobStats{
		TotalChildren:     s.ChildCount,
		CompletedChildren: s.CompletedChildren,
		FailedChildren:    s.FailedChildren,
		CancelledChildren: s.CancelledChildren,
		RunningChildren:   s.RunningChildren,
		PendingChildren:   s.PendingChildren,
	}, nil
}

// GetJobChildStats implements interfaces.JobManager.GetJobChildStats
func (m *Manager) GetJobChildStats(ctx context.Context, parentIDs []string) (map[string]*interfaces.JobChildStats, error) {
	return m.jobStorage.GetJobChildStats(ctx, parentIDs)
}
