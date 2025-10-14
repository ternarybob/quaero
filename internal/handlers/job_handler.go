// -----------------------------------------------------------------------
// Last Modified: Monday, 14th October 2025 3:45:00 pm
// Modified By: Claude Code
// -----------------------------------------------------------------------

package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// JobHandler handles job-related API requests
type JobHandler struct {
	crawlerService *crawler.Service
	jobStorage     interfaces.JobStorage
	logger         arbor.ILogger
}

// NewJobHandler creates a new job handler
func NewJobHandler(crawlerService *crawler.Service, jobStorage interfaces.JobStorage, logger arbor.ILogger) *JobHandler {
	return &JobHandler{
		crawlerService: crawlerService,
		jobStorage:     jobStorage,
		logger:         logger,
	}
}

// ListJobsHandler returns a paginated list of jobs
// GET /api/jobs?limit=50&offset=0&status=completed&source=jira&entity=project
func (h *JobHandler) ListJobsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	status := r.URL.Query().Get("status")
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

	jobs, err := h.crawlerService.ListJobs(ctx, opts)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to list jobs")
		http.Error(w, "Failed to list jobs", http.StatusInternalServerError)
		return
	}

	// Get total count
	totalCount, err := h.jobStorage.CountJobs(ctx)
	if err != nil {
		h.logger.Warn().Err(err).Msg("Failed to count jobs")
		totalCount = len(jobs)
	}

	response := map[string]interface{}{
		"jobs":        jobs,
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

	job, err := h.crawlerService.GetJobStatus(jobID)
	if err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job")
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	// If job is not in active jobs, try to get it from database
	if job == nil {
		jobInterface, err := h.jobStorage.GetJob(ctx, jobID)
		if err != nil {
			h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job from storage")
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}

		var ok bool
		job, ok = jobInterface.(*crawler.CrawlJob)
		if !ok {
			http.Error(w, "Invalid job type", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
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

	results, err := h.crawlerService.GetJobResults(jobID)
	if err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job results")
		http.Error(w, "Failed to get job results", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_id":  jobID,
		"results": results,
		"count":   len(results),
	})
}

// RerunJobHandler re-executes a previous job
// POST /api/jobs/{id}/rerun
func (h *JobHandler) RerunJobHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract job ID from path: /api/jobs/{id}/rerun
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
		http.Error(w, "Failed to rerun job", http.StatusInternalServerError)
		return
	}

	h.logger.Info().Str("original_job_id", jobID).Str("new_job_id", newJobID).Msg("Job rerun started")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"original_job_id": jobID,
		"new_job_id":      newJobID,
		"message":         "Job rerun started successfully",
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
	job, err := h.crawlerService.GetJobStatus(jobID)
	if err == nil && job != nil && job.Status == crawler.JobStatusRunning {
		http.Error(w, "Cannot delete a running job. Cancel it first.", http.StatusBadRequest)
		return
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
