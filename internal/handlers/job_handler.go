// -----------------------------------------------------------------------
// Last Modified: Thursday, 23rd October 2025 8:03:36 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/crawler"
	"github.com/ternarybob/quaero/internal/services/sources"
)

// JobGroup represents a parent job with its children
type JobGroup struct {
	Parent   *models.CrawlJob
	Children []*models.CrawlJob
}

// JobHandler handles job-related API requests
type JobHandler struct {
	crawlerService   *crawler.Service
	jobStorage       interfaces.JobStorage
	sourceService    *sources.Service
	authStorage      interfaces.AuthStorage
	schedulerService interfaces.SchedulerService
	logService       interfaces.LogService
	jobManager       interfaces.JobManager
	config           *common.Config
	logger           arbor.ILogger
}

// NewJobHandler creates a new job handler
func NewJobHandler(crawlerService *crawler.Service, jobStorage interfaces.JobStorage, sourceService *sources.Service, authStorage interfaces.AuthStorage, schedulerService interfaces.SchedulerService, logService interfaces.LogService, jobManager interfaces.JobManager, config *common.Config, logger arbor.ILogger) *JobHandler {
	return &JobHandler{
		crawlerService:   crawlerService,
		jobStorage:       jobStorage,
		sourceService:    sourceService,
		authStorage:      authStorage,
		schedulerService: schedulerService,
		logService:       logService,
		jobManager:       jobManager,
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
	parentID := r.URL.Query().Get("parent_id")
	groupedStr := r.URL.Query().Get("grouped")
	grouped := false
	if groupedStr == "true" {
		grouped = true
	}
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

	opts := &interfaces.JobListOptions{
		Limit:      limit,
		Offset:     offset,
		Status:     status,
		SourceType: sourceType,
		EntityType: entityType,
		ParentID:   parentID, // NEW
		Grouped:    grouped,  // NEW
		OrderBy:    orderBy,
		OrderDir:   orderDir,
	}

	jobs, err := h.jobManager.ListJobs(ctx, opts)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to list jobs")
		http.Error(w, "Failed to list jobs", http.StatusInternalServerError)
		return
	}

	// Extract parent job IDs for statistics calculation
	parentJobIDs := make([]string, 0)
	for _, job := range jobs {
		// Only calculate stats for parent jobs (jobs with no parent_id)
		if job.ParentID == "" {
			parentJobIDs = append(parentJobIDs, job.ID)
		}
	}

	if grouped && parentID != "" && parentID != "root" {
		// Add the parent ID to the list of IDs to get stats for
		found := false
		for _, id := range parentJobIDs {
			if id == parentID {
				found = true
				break
			}
		}
		if !found {
			parentJobIDs = append(parentJobIDs, parentID)
		}
	}

	// Fetch child statistics in batch
	var childStatsMap map[string]*interfaces.JobChildStats
	if len(parentJobIDs) > 0 {
		var err error
		childStatsMap, err = h.jobManager.GetJobChildStats(ctx, parentJobIDs)
		if err != nil {
			h.logger.Warn().Err(err).Msg("Failed to get child statistics, continuing without stats")
			childStatsMap = make(map[string]*interfaces.JobChildStats)
		}
	} else {
		childStatsMap = make(map[string]*interfaces.JobChildStats)
	}

	// Get total count using JobManager (ensures consistent filtering)
	totalCount, err := h.jobManager.CountJobs(ctx, opts)
	if err != nil {
		h.logger.Warn().Err(err).Msg("Failed to count jobs, using result length")
		totalCount = len(jobs)
	}

	if !grouped {
		// Mask sensitive data and enrich with statistics
		enrichedJobs := make([]map[string]interface{}, 0, len(jobs))
		for _, job := range jobs {
			masked := job.MaskSensitiveData()

			// Convert to map and add statistics
			jobMap := convertJobToMap(masked)
			jobMap["parent_id"] = masked.ParentID

			// Add child statistics
			if stats, exists := childStatsMap[masked.ID]; exists {
				jobMap["child_count"] = stats.ChildCount
				jobMap["completed_children"] = stats.CompletedChildren
				jobMap["failed_children"] = stats.FailedChildren
			} else {
				jobMap["child_count"] = 0
				jobMap["completed_children"] = 0
				jobMap["failed_children"] = 0
			}

			enrichedJobs = append(enrichedJobs, jobMap)
		}

		response := map[string]interface{}{
			"jobs":        enrichedJobs,
			"total_count": totalCount,
			"limit":       limit,
			"offset":      offset,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Group jobs by parent
	groupsMap := make(map[string]*JobGroup)
	var orphanJobs []*models.CrawlJob

	if grouped && parentID != "" && parentID != "root" {
		// Fetch the parent job to ensure it's in the response
		parentJob, err := h.jobManager.GetJob(ctx, parentID)
		if err == nil {
			if p, ok := parentJob.(*models.CrawlJob); ok {
				groupsMap[p.ID] = &JobGroup{
					Parent:   p,
					Children: make([]*models.CrawlJob, 0),
				}
			}
		}
	}

	for _, job := range jobs {
		if job.ParentID == "" {
			// This is a parent job
			if _, exists := groupsMap[job.ID]; !exists {
				groupsMap[job.ID] = &JobGroup{
					Parent:   job,
					Children: make([]*models.CrawlJob, 0),
				}
			}
		} else {
			// This is a child job
			if group, exists := groupsMap[job.ParentID]; exists {
				group.Children = append(group.Children, job)
			} else {
				// Parent not in current page, treat as orphan
				orphanJobs = append(orphanJobs, job.MaskSensitiveData())
			}
		}
	}

	// Convert to array and enrich with statistics
	groups := make([]map[string]interface{}, 0, len(groupsMap))
	for parentID, group := range groupsMap {
		maskedParent := group.Parent.MaskSensitiveData()
		parentMap := convertJobToMap(maskedParent)
		parentMap["parent_id"] = maskedParent.ParentID

		// Add statistics
		if stats, exists := childStatsMap[parentID]; exists {
			parentMap["child_count"] = stats.ChildCount
			parentMap["completed_children"] = stats.CompletedChildren
			parentMap["failed_children"] = stats.FailedChildren
		} else {
			parentMap["child_count"] = 0
			parentMap["completed_children"] = 0
			parentMap["failed_children"] = 0
		}

		// Mask children
		maskedChildren := make([]*models.CrawlJob, 0, len(group.Children))
		for _, child := range group.Children {
			maskedChildren = append(maskedChildren, child.MaskSensitiveData())
		}

		groups = append(groups, map[string]interface{}{
			"parent":   parentMap,
			"children": maskedChildren,
		})
	}

	response := map[string]interface{}{
		"groups":      groups,
		"orphans":     orphanJobs, // Jobs whose parent is not in current page
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

	jobInterface, err := h.jobManager.GetJob(ctx, jobID)
	if err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job")
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	// Type assert the job from active jobs
	job, ok := jobInterface.(*models.CrawlJob)
	if !ok {
		http.Error(w, "Invalid job type", http.StatusInternalServerError)
		return
	}

	// Mask sensitive data before returning
	masked := job.MaskSensitiveData()

	// For parent jobs (empty parent_id), enrich with child statistics
	if masked.ParentID == "" {
		childStatsMap, err := h.jobManager.GetJobChildStats(ctx, []string{masked.ID})
		if err != nil {
			h.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to get child statistics, continuing without stats")
		}

		// Convert to map for enrichment
		jobMap := convertJobToMap(masked)
		jobMap["parent_id"] = masked.ParentID

		// Add child statistics
		if stats, exists := childStatsMap[masked.ID]; exists {
			jobMap["child_count"] = stats.ChildCount
			jobMap["completed_children"] = stats.CompletedChildren
			jobMap["failed_children"] = stats.FailedChildren
		} else {
			jobMap["child_count"] = 0
			jobMap["completed_children"] = 0
			jobMap["failed_children"] = 0
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jobMap)
		return
	}

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
// GET /api/jobs/{id}/logs?order=desc (desc=newest-first, asc=oldest-first)
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

	// Parse order query parameter (default: desc = newest-first)
	order := r.URL.Query().Get("order")
	if order == "" {
		order = "desc" // Default to newest-first
	}

	logs, err := h.logService.GetLogs(ctx, jobID, 1000) // Limit to 1000 most recent logs
	if err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job logs")
		http.Error(w, "Failed to get job logs", http.StatusInternalServerError)
		return
	}

	// If logs are empty, check if job exists (return 404 if job doesn't exist)
	if len(logs) == 0 {
		_, err := h.jobStorage.GetJob(ctx, jobID)
		if err != nil {
			h.logger.Debug().Err(err).Str("job_id", jobID).Msg("Job not found")
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		// Job exists but has no logs yet - return empty array with 200 OK
	}

	// Apply ordering: logs come from DB in DESC order (newest-first)
	// If asc requested, reverse the slice
	if order == "asc" {
		// Reverse slice for oldest-first ordering
		for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
			logs[i], logs[j] = logs[j], logs[i]
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_id": jobID,
		"logs":   logs,
		"count":  len(logs),
		"order":  order, // Include order in response for client awareness
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
		job, ok := jobInterface.(*models.CrawlJob)
		if !ok {
			http.Error(w, "Invalid job type", http.StatusInternalServerError)
			return
		}
		if job.Status == models.JobStatusRunning {
			h.logger.Warn().Str("job_id", jobID).Msg("Cannot rerun active job")
			http.Error(w, "Cannot rerun an active job. Wait for it to complete or cancel it first.", http.StatusBadRequest)
			return
		}
	}

	// Check if job is marked as running in storage
	jobFromStorage, err := h.jobStorage.GetJob(ctx, jobID)
	if err == nil && jobFromStorage != nil {
		if crawlJob, ok := jobFromStorage.(*models.CrawlJob); ok {
			if crawlJob.Status == models.JobStatusRunning {
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

// CopyJobHandler duplicates a job with a new ID
// POST /api/jobs/{id}/copy
func (h *JobHandler) CopyJobHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract job ID from path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}
	jobID := pathParts[2]

	// Copy job via JobManager
	newJobID, err := h.jobManager.CopyJob(ctx, jobID)
	if err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to copy job")
		http.Error(w, "Failed to copy job", http.StatusInternalServerError)
		return
	}

	h.logger.Info().Str("original_job_id", jobID).Str("new_job_id", newJobID).Msg("Job copied")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"original_job_id": jobID,
		"new_job_id":      newJobID,
		"message":         "Job copied successfully",
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
		job, ok := jobInterface.(*models.CrawlJob)
		if !ok {
			http.Error(w, "Invalid job type", http.StatusInternalServerError)
			return
		}
		if job.Status == models.JobStatusRunning {
			http.Error(w, "Cannot delete a running job. Cancel it first.", http.StatusBadRequest)
			return
		}
	}

	err = h.jobManager.DeleteJob(ctx, jobID)
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

	pendingCount, _ := h.jobStorage.CountJobsByStatus(ctx, string(models.JobStatusPending))
	runningCount, _ := h.jobStorage.CountJobsByStatus(ctx, string(models.JobStatusRunning))
	completedCount, _ := h.jobStorage.CountJobsByStatus(ctx, string(models.JobStatusCompleted))
	failedCount, _ := h.jobStorage.CountJobsByStatus(ctx, string(models.JobStatusFailed))
	cancelledCount, _ := h.jobStorage.CountJobsByStatus(ctx, string(models.JobStatusCancelled))

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
	pendingJobsInterface, err := h.jobStorage.GetJobsByStatus(ctx, string(models.JobStatusPending))
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch pending jobs")
		http.Error(w, "Failed to fetch job queue", http.StatusInternalServerError)
		return
	}

	// Fetch running jobs
	runningJobsInterface, err := h.jobStorage.GetJobsByStatus(ctx, string(models.JobStatusRunning))
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch running jobs")
		http.Error(w, "Failed to fetch job queue", http.StatusInternalServerError)
		return
	}

	// Mask sensitive data
	pendingJobs := make([]*models.CrawlJob, 0, len(pendingJobsInterface))
	for _, job := range pendingJobsInterface {
		pendingJobs = append(pendingJobs, job.MaskSensitiveData())
	}

	runningJobs := make([]*models.CrawlJob, 0, len(runningJobsInterface))
	for _, job := range runningJobsInterface {
		runningJobs = append(runningJobs, job.MaskSensitiveData())
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

	job, ok := jobInterface.(*models.CrawlJob)
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

// convertJobToMap converts a CrawlJob struct to a map for JSON response enrichment
func convertJobToMap(job *models.CrawlJob) map[string]interface{} {
	// Marshal to JSON then unmarshal to map to preserve all fields and JSON tags
	data, err := json.Marshal(job)
	if err != nil {
		return map[string]interface{}{"id": job.ID, "error": "failed to serialize job"}
	}

	var jobMap map[string]interface{}
	if err := json.Unmarshal(data, &jobMap); err != nil {
		return map[string]interface{}{"id": job.ID, "error": "failed to deserialize job"}
	}

	return jobMap
}
