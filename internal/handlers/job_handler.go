// -----------------------------------------------------------------------
// Last Modified: Thursday, 23rd October 2025 8:03:36 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/services/crawler"
	"github.com/ternarybob/quaero/internal/services/sources"
	"github.com/ternarybob/quaero/internal/storage/sqlite"
)

// JobHandler handles job-related API requests
type JobHandler struct {
	crawlerService   *crawler.Service
	jobStorage       interfaces.JobStorage
	sourceService    *sources.Service
	authStorage      interfaces.AuthStorage
	schedulerService interfaces.SchedulerService
	config           *common.Config
	logger           arbor.ILogger
}

// NewJobHandler creates a new job handler
func NewJobHandler(crawlerService *crawler.Service, jobStorage interfaces.JobStorage, sourceService *sources.Service, authStorage interfaces.AuthStorage, schedulerService interfaces.SchedulerService, config *common.Config, logger arbor.ILogger) *JobHandler {
	return &JobHandler{
		crawlerService:   crawlerService,
		jobStorage:       jobStorage,
		sourceService:    sourceService,
		authStorage:      authStorage,
		schedulerService: schedulerService,
		config:           config,
		logger:           logger,
	}
}

// ListJobsHandler returns a paginated list of jobs
// GET /api/jobs?limit=50&offset=0&status=completed&source=jira&entity=project
// Note: status parameter supports comma-separated values (e.g., "pending,running")
func (h *JobHandler) ListJobsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	status := r.URL.Query().Get("status") // Supports comma-separated values (e.g., "pending,running"); parsing handled by storage layer
	sourceType := r.URL.Query().Get("source")
	entityType := r.URL.Query().Get("entity")
	orderBy := r.URL.Query().Get("order_by")
	orderDir := r.URL.Query().Get("order_dir")

	// Set defaults
	limit := 50
	offset := 0

	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil {
			limit = parsed
		}
	}

	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsed
		}
	}

	if orderBy == "" {
		orderBy = "created_at"
	}
	if orderDir == "" {
		orderDir = "DESC"
	}

	opts := &interfaces.ListOptions{
		Limit:      limit,
		Offset:     offset,
		Status:     status,
		SourceType: sourceType,
		EntityType: entityType,
		OrderBy:    orderBy,
		OrderDir:   orderDir,
	}

	jobsInterface, err := h.crawlerService.ListJobs(ctx, opts)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to list jobs")
		http.Error(w, "Failed to list jobs", http.StatusInternalServerError)
		return
	}

	// Type assert to []*crawler.CrawlJob
	jobs, ok := jobsInterface.([]*crawler.CrawlJob)
	if !ok {
		h.logger.Error().Msg("Unexpected result type from ListJobs")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Mask sensitive data in jobs
	maskedJobs := make([]*crawler.CrawlJob, len(jobs))
	for i, job := range jobs {
		maskedJobs[i] = job.MaskSensitiveData()
	}

	// Get total count - use filtered count when filters are present
	totalCount := 0
	hasFilters := opts != nil && (opts.Status != "" || opts.SourceType != "" || opts.EntityType != "")

	if hasFilters {
		// Get filtered count matching the query criteria
		filteredCount, err := h.jobStorage.CountJobsWithFilters(ctx, opts)
		if err != nil {
			h.logger.Warn().Err(err).Msg("Failed to count filtered jobs, falling back to global count")
			totalCount, _ = h.jobStorage.CountJobs(ctx)
		} else {
			totalCount = filteredCount
		}
	} else {
		// No filters: use global count
		globalCount, err := h.jobStorage.CountJobs(ctx)
		if err != nil {
			h.logger.Warn().Err(err).Msg("Failed to count jobs")
			totalCount = len(jobs)
		} else {
			totalCount = globalCount
		}
	}

	response := map[string]interface{}{
		"jobs":        maskedJobs,
		"total_count": totalCount,
		"limit":       limit,
		"offset":      offset,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetJobHandler returns a single job by ID
// GET /api/jobs/{id}
func (h *JobHandler) GetJobHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract job ID from path: /api/jobs/{id}
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}
	jobID := pathParts[2]

	if jobID == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	// Try to get job from active jobs first
	jobInterface, err := h.crawlerService.GetJobStatus(jobID)

	// If job is not in active jobs or there was an error, try to get it from database
	if err != nil || jobInterface == nil {
		if err != nil {
			h.logger.Debug().Err(err).Str("job_id", jobID).Msg("Job not in active jobs, checking storage")
		}

		jobInterface, storageErr := h.jobStorage.GetJob(ctx, jobID)
		if storageErr != nil {
			h.logger.Error().Err(storageErr).Str("job_id", jobID).Msg("Failed to get job from storage")
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}

		job, ok := jobInterface.(*crawler.CrawlJob)
		if !ok {
			http.Error(w, "Invalid job type", http.StatusInternalServerError)
			return
		}

		// Mask sensitive data before returning
		masked := job.MaskSensitiveData()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(masked)
		return
	}

	// Type assert the job from active jobs
	job, ok := jobInterface.(*crawler.CrawlJob)
	if !ok {
		http.Error(w, "Invalid job type", http.StatusInternalServerError)
		return
	}

	// Mask sensitive data before returning
	masked := job.MaskSensitiveData()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(masked)
}

// GetJobResultsHandler returns the results of a completed job
// GET /api/jobs/{id}/results
func (h *JobHandler) GetJobResultsHandler(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path: /api/jobs/{id}/results
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}
	jobID := pathParts[2]

	if jobID == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	resultsInterface, err := h.crawlerService.GetJobResults(jobID)
	if err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job results")
		http.Error(w, "Failed to get job results", http.StatusInternalServerError)
		return
	}

	// Type assert to []*crawler.CrawlResult
	results, ok := resultsInterface.([]*crawler.CrawlResult)
	if !ok {
		h.logger.Error().Str("job_id", jobID).Msg("Unexpected result type from GetJobResults")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_id":  jobID,
		"results": results,
		"count":   len(results),
	})
}

// GetJobLogsHandler returns the logs of a job
// GET /api/jobs/{id}/logs
func (h *JobHandler) GetJobLogsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract job ID from path: /api/jobs/{id}/logs
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}
	jobID := pathParts[2]

	if jobID == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	logs, err := h.jobStorage.GetJobLogs(ctx, jobID)
	if err != nil {
		if errors.Is(err, sqlite.ErrJobNotFound) {
			h.logger.Debug().Err(err).Str("job_id", jobID).Msg("Job not found when retrieving logs")
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job logs")
		http.Error(w, "Failed to get job logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_id": jobID,
		"logs":   logs,
		"count":  len(logs),
	})
}

// RerunJobHandler re-executes a previous job
// POST /api/jobs/{id}/rerun
func (h *JobHandler) RerunJobHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Log the incoming request for debugging
	h.logger.Debug().
		Str("method", r.Method).
		Str("url_path", r.URL.Path).
		Str("raw_path", r.URL.RawPath).
		Msg("Rerun job request received")

	// Extract job ID from path: /api/jobs/{id}/rerun
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	h.logger.Debug().
		Strs("path_parts", pathParts).
		Int("parts_length", len(pathParts)).
		Msg("Path parts after split")

	if len(pathParts) < 3 {
		h.logger.Warn().
			Str("url_path", r.URL.Path).
			Int("parts_length", len(pathParts)).
			Msg("Invalid path: not enough segments")
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}
	jobID := pathParts[2]

	h.logger.Debug().
		Str("extracted_job_id", jobID).
		Msg("Extracted job ID from path")

	if jobID == "" {
		h.logger.Warn().Msg("Job ID is empty after extraction")
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	// Check if job is currently running in memory - prevent rerun of active jobs
	jobInterface, err := h.crawlerService.GetJobStatus(jobID)
	if err == nil && jobInterface != nil {
		job, ok := jobInterface.(*crawler.CrawlJob)
		if !ok {
			http.Error(w, "Invalid job type", http.StatusInternalServerError)
			return
		}
		if job.Status == crawler.JobStatusRunning {
			h.logger.Warn().Str("job_id", jobID).Msg("Cannot rerun active job")
			http.Error(w, "Cannot rerun an active job. Wait for it to complete or cancel it first.", http.StatusBadRequest)
			return
		}
	}

	// Check if job is marked as running in storage
	jobFromStorage, err := h.jobStorage.GetJob(ctx, jobID)
	if err == nil && jobFromStorage != nil {
		if crawlJob, ok := jobFromStorage.(*crawler.CrawlJob); ok {
			if crawlJob.Status == crawler.JobStatusRunning {
				h.logger.Warn().Str("job_id", jobID).Msg("Cannot rerun job marked as running in database")
				http.Error(w, "Cannot rerun an active job. Wait for it to complete or cancel it first.", http.StatusBadRequest)
				return
			}
		}
	}

	// Parse optional config update from request body
	var updateConfig *crawler.CrawlConfig
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&updateConfig); err != nil {
			// Ignore parse errors - updateConfig will be nil and original config will be used
			h.logger.Debug().Err(err).Msg("No config update provided, using original")
		}
	}

	newJobID, err := h.crawlerService.RerunJob(ctx, jobID, updateConfig)
	if err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to rerun job")
		// Return detailed error for debugging
		http.Error(w, "Failed to rerun job: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.logger.Info().Str("original_job_id", jobID).Str("new_job_id", newJobID).Msg("Job copied and queued")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"original_job_id": jobID,
		"new_job_id":      newJobID,
		"message":         "Job copied and added to queue successfully",
	})
}

// CancelJobHandler cancels a running job
// POST /api/jobs/{id}/cancel
func (h *JobHandler) CancelJobHandler(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path: /api/jobs/{id}/cancel
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}
	jobID := pathParts[2]

	if jobID == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	err := h.crawlerService.CancelJob(jobID)
	if err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to cancel job")
		http.Error(w, "Failed to cancel job", http.StatusInternalServerError)
		return
	}

	h.logger.Info().Str("job_id", jobID).Msg("Job cancelled")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_id":  jobID,
		"message": "Job cancelled successfully",
	})
}

// DeleteJobHandler deletes a job from the database
// DELETE /api/jobs/{id}
func (h *JobHandler) DeleteJobHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract job ID from path: /api/jobs/{id}
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}
	jobID := pathParts[2]

	if jobID == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	// Check if job is still running
	jobInterface, err := h.crawlerService.GetJobStatus(jobID)
	if err == nil && jobInterface != nil {
		job, ok := jobInterface.(*crawler.CrawlJob)
		if !ok {
			http.Error(w, "Invalid job type", http.StatusInternalServerError)
			return
		}
		if job.Status == crawler.JobStatusRunning {
			http.Error(w, "Cannot delete a running job. Cancel it first.", http.StatusBadRequest)
			return
		}
	}

	err = h.jobStorage.DeleteJob(ctx, jobID)
	if err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to delete job")
		http.Error(w, "Failed to delete job", http.StatusInternalServerError)
		return
	}

	h.logger.Info().Str("job_id", jobID).Msg("Job deleted")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_id":  jobID,
		"message": "Job deleted successfully",
	})
}

// GetJobStatsHandler returns statistics about jobs
// GET /api/jobs/stats
func (h *JobHandler) GetJobStatsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	totalCount, err := h.jobStorage.CountJobs(ctx)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to count total jobs")
		totalCount = 0
	}

	pendingCount, _ := h.jobStorage.CountJobsByStatus(ctx, string(crawler.JobStatusPending))
	runningCount, _ := h.jobStorage.CountJobsByStatus(ctx, string(crawler.JobStatusRunning))
	completedCount, _ := h.jobStorage.CountJobsByStatus(ctx, string(crawler.JobStatusCompleted))
	failedCount, _ := h.jobStorage.CountJobsByStatus(ctx, string(crawler.JobStatusFailed))
	cancelledCount, _ := h.jobStorage.CountJobsByStatus(ctx, string(crawler.JobStatusCancelled))

	stats := map[string]interface{}{
		"total_jobs":     totalCount,
		"pending_jobs":   pendingCount,
		"running_jobs":   runningCount,
		"completed_jobs": completedCount,
		"failed_jobs":    failedCount,
		"cancelled_jobs": cancelledCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// ConfigOverrides allows selective overriding of crawl configuration
// Using pointers to detect presence of fields
// TODO: Filtering configuration will be handled at the job definition level in subsequent phases
type ConfigOverrides struct {
	MaxDepth    *int  `json:"max_depth,omitempty"`
	MaxPages    *int  `json:"max_pages,omitempty"`
	Concurrency *int  `json:"concurrency,omitempty"`
	RateLimit   *int  `json:"rate_limit,omitempty"` // milliseconds
	FollowLinks *bool `json:"follow_links,omitempty"`
}

// CreateJobHandler creates a new job from a source configuration
// POST /api/jobs/create
// DEPRECATED: Direct job creation from sources is deprecated. Use Job Definitions to specify start URLs.
func (h *JobHandler) CreateJobHandler(w http.ResponseWriter, r *http.Request) {
	h.logger.Warn().Msg("CreateJobHandler called - endpoint is deprecated")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   "Not Implemented",
		"message": "Direct job creation from sources is deprecated. Please use Job Definitions to specify start URLs and crawl parameters.",
	})
}

// GetJobQueueHandler returns jobs in pending or running status
// GET /api/jobs/queue
func (h *JobHandler) GetJobQueueHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Fetch pending jobs
	pendingJobsInterface, err := h.jobStorage.GetJobsByStatus(ctx, string(crawler.JobStatusPending))
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch pending jobs")
		http.Error(w, "Failed to fetch job queue", http.StatusInternalServerError)
		return
	}

	// Fetch running jobs
	runningJobsInterface, err := h.jobStorage.GetJobsByStatus(ctx, string(crawler.JobStatusRunning))
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch running jobs")
		http.Error(w, "Failed to fetch job queue", http.StatusInternalServerError)
		return
	}

	// Convert to CrawlJob slices and mask sensitive data
	pendingJobs := make([]*crawler.CrawlJob, 0)
	for _, jobInterface := range pendingJobsInterface {
		if job, ok := jobInterface.(*crawler.CrawlJob); ok {
			pendingJobs = append(pendingJobs, job.MaskSensitiveData())
		}
	}

	runningJobs := make([]*crawler.CrawlJob, 0)
	for _, jobInterface := range runningJobsInterface {
		if job, ok := jobInterface.(*crawler.CrawlJob); ok {
			runningJobs = append(runningJobs, job.MaskSensitiveData())
		}
	}

	totalCount := len(pendingJobs) + len(runningJobs)

	response := map[string]interface{}{
		"pending": pendingJobs,
		"running": runningJobs,
		"total":   totalCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateJobHandler updates job metadata like name and description
// PUT /api/jobs/{id}
func (h *JobHandler) UpdateJobHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract job ID from path: /api/jobs/{id}
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}
	jobID := pathParts[2]

	if jobID == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	// Parse request body
	var req struct {
		Name        *string `json:"name,omitempty"`
		Description *string `json:"description,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error().Err(err).Msg("Failed to parse request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate at least one field is provided
	if req.Name == nil && req.Description == nil {
		http.Error(w, "At least one field (name or description) must be provided", http.StatusBadRequest)
		return
	}

	// Get existing job from storage
	jobInterface, err := h.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job from storage")
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	job, ok := jobInterface.(*crawler.CrawlJob)
	if !ok {
		h.logger.Error().Str("job_id", jobID).Msg("Invalid job type")
		http.Error(w, "Invalid job type", http.StatusInternalServerError)
		return
	}

	// Update fields if provided
	if req.Name != nil {
		job.Name = *req.Name
	}
	if req.Description != nil {
		job.Description = *req.Description
	}

	// Update job in storage
	if err := h.jobStorage.UpdateJob(ctx, job); err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to update job")
		http.Error(w, "Failed to update job", http.StatusInternalServerError)
		return
	}

	h.logger.Info().
		Str("job_id", jobID).
		Str("name", job.Name).
		Str("description", job.Description).
		Msg("Job updated successfully")

	// Return updated job (masked)
	masked := job.MaskSensitiveData()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job":     masked,
		"message": "Job updated successfully",
	})
}
