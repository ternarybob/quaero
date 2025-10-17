// -----------------------------------------------------------------------
// Last Modified: Monday, 14th October 2025 3:45:00 pm
// Modified By: Claude Code
// -----------------------------------------------------------------------

package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/crawler"
	"github.com/ternarybob/quaero/internal/services/sources"
)

// JobHandler handles job-related API requests
type JobHandler struct {
	crawlerService   *crawler.Service
	jobStorage       interfaces.JobStorage
	sourceService    *sources.Service
	authStorage      interfaces.AuthStorage
	schedulerService interfaces.SchedulerService
	logger           arbor.ILogger
}

// NewJobHandler creates a new job handler
func NewJobHandler(crawlerService *crawler.Service, jobStorage interfaces.JobStorage, sourceService *sources.Service, authStorage interfaces.AuthStorage, schedulerService interfaces.SchedulerService, logger arbor.ILogger) *JobHandler {
	return &JobHandler{
		crawlerService:   crawlerService,
		jobStorage:       jobStorage,
		sourceService:    sourceService,
		authStorage:      authStorage,
		schedulerService: schedulerService,
		logger:           logger,
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

	// Mask sensitive data in jobs
	maskedJobs := make([]*crawler.CrawlJob, len(jobs))
	for i, job := range jobs {
		maskedJobs[i] = job.MaskSensitiveData()
	}

	// Get total count
	totalCount, err := h.jobStorage.CountJobs(ctx)
	if err != nil {
		h.logger.Warn().Err(err).Msg("Failed to count jobs")
		totalCount = len(jobs)
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
	job, err := h.crawlerService.GetJobStatus(jobID)

	// If job is not in active jobs or there was an error, try to get it from database
	if err != nil || job == nil {
		if err != nil {
			h.logger.Debug().Err(err).Str("job_id", jobID).Msg("Job not in active jobs, checking storage")
		}

		jobInterface, storageErr := h.jobStorage.GetJob(ctx, jobID)
		if storageErr != nil {
			h.logger.Error().Err(storageErr).Str("job_id", jobID).Msg("Failed to get job from storage")
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

// ConfigOverrides allows selective overriding of crawl configuration
// Using pointers to detect presence of fields
type ConfigOverrides struct {
	MaxDepth        *int      `json:"max_depth,omitempty"`
	MaxPages        *int      `json:"max_pages,omitempty"`
	Concurrency     *int      `json:"concurrency,omitempty"`
	RateLimit       *int      `json:"rate_limit,omitempty"` // milliseconds
	FollowLinks     *bool     `json:"follow_links,omitempty"`
	IncludePatterns *[]string `json:"include_patterns,omitempty"`
	ExcludePatterns *[]string `json:"exclude_patterns,omitempty"`
}

// CreateJobHandler creates a new job from a source configuration
// POST /api/jobs/create
func (h *JobHandler) CreateJobHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request body
	var req struct {
		SourceID        string           `json:"source_id"`
		RefreshSource   bool             `json:"refresh_source"`
		SeedURLs        []string         `json:"seed_urls"`
		ConfigOverrides *ConfigOverrides `json:"config_overrides"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error().Err(err).Msg("Failed to parse request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.SourceID == "" {
		http.Error(w, "source_id is required", http.StatusBadRequest)
		return
	}

	// Fetch source configuration
	source, err := h.sourceService.GetSource(ctx, req.SourceID)
	if err != nil {
		h.logger.Error().Err(err).Str("source_id", req.SourceID).Msg("Failed to fetch source")
		http.Error(w, "Source not found", http.StatusNotFound)
		return
	}

	// Validate source configuration
	if err := source.Validate(); err != nil {
		h.logger.Error().Err(err).Str("source_id", req.SourceID).Msg("Source validation failed")
		http.Error(w, fmt.Sprintf("Invalid source configuration: %v", err), http.StatusBadRequest)
		return
	}

	// Fetch auth credentials if source has auth_id
	var authCreds *models.AuthCredentials
	if source.AuthID != "" {
		authCreds, err = h.authStorage.GetCredentialsByID(ctx, source.AuthID)
		if err != nil {
			h.logger.Error().Err(err).Str("auth_id", source.AuthID).Msg("Failed to fetch auth credentials")
			http.Error(w, "Authentication credentials not found", http.StatusNotFound)
			return
		}
	}

	// Derive seed URLs if not provided
	seedURLs := req.SeedURLs
	if len(seedURLs) == 0 {
		seedURLs = h.deriveSeedURLs(source)
		if len(seedURLs) == 0 {
			http.Error(w, "Failed to derive seed URLs from source configuration", http.StatusBadRequest)
			return
		}
	}

	// Merge config overrides with source's crawl config
	// Start with source config, then apply overrides only if explicitly provided
	crawlConfig := source.CrawlConfig
	if req.ConfigOverrides != nil {
		if req.ConfigOverrides.MaxDepth != nil {
			crawlConfig.MaxDepth = *req.ConfigOverrides.MaxDepth
		}
		if req.ConfigOverrides.Concurrency != nil {
			crawlConfig.Concurrency = *req.ConfigOverrides.Concurrency
		}
		if req.ConfigOverrides.MaxPages != nil {
			crawlConfig.MaxPages = *req.ConfigOverrides.MaxPages
		}
		if req.ConfigOverrides.RateLimit != nil {
			crawlConfig.RateLimit = *req.ConfigOverrides.RateLimit
		}
		if req.ConfigOverrides.FollowLinks != nil {
			crawlConfig.FollowLinks = *req.ConfigOverrides.FollowLinks
		}
		if req.ConfigOverrides.IncludePatterns != nil {
			crawlConfig.IncludePatterns = *req.ConfigOverrides.IncludePatterns
		}
		if req.ConfigOverrides.ExcludePatterns != nil {
			crawlConfig.ExcludePatterns = *req.ConfigOverrides.ExcludePatterns
		}
	}

	// Convert source.CrawlConfig to crawler.CrawlConfig (RateLimit: int ms -> time.Duration)
	crawlerConfig := crawler.CrawlConfig{
		MaxDepth:        crawlConfig.MaxDepth,
		MaxPages:        crawlConfig.MaxPages,
		FollowLinks:     crawlConfig.FollowLinks,
		Concurrency:     crawlConfig.Concurrency,
		RateLimit:       time.Duration(crawlConfig.RateLimit) * time.Millisecond,
		IncludePatterns: crawlConfig.IncludePatterns,
		ExcludePatterns: crawlConfig.ExcludePatterns,
		DetailLevel:     "full",
		RetryAttempts:   3,
		RetryBackoff:    2 * time.Second,
	}

	// Derive entity type based on source type
	entityType := h.deriveEntityType(source)

	// Start crawl with snapshots
	jobID, err := h.crawlerService.StartCrawl(
		source.Type,
		entityType,
		seedURLs,
		crawlerConfig,
		req.SourceID,
		req.RefreshSource,
		source,
		authCreds,
	)
	if err != nil {
		h.logger.Error().Err(err).Str("source_id", req.SourceID).Msg("Failed to start crawl")
		http.Error(w, fmt.Sprintf("Failed to create job: %v", err), http.StatusInternalServerError)
		return
	}

	h.logger.Info().
		Str("job_id", jobID).
		Str("source_id", req.SourceID).
		Str("refresh_source", fmt.Sprintf("%t", req.RefreshSource)).
		Msg("Job created successfully")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_id":         jobID,
		"source_id":      req.SourceID,
		"source_type":    source.Type,
		"refresh_source": req.RefreshSource,
		"message":        "Job created successfully",
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

// GetDefaultJobsHandler returns all default job statuses
// GET /api/jobs/default
func (h *JobHandler) GetDefaultJobsHandler(w http.ResponseWriter, r *http.Request) {
	statuses := h.schedulerService.GetAllJobStatuses()

	// Convert map to array for JSON response
	jobsList := make([]map[string]interface{}, 0, len(statuses))
	for name, status := range statuses {
		jobsList = append(jobsList, map[string]interface{}{
			"name":        name,
			"enabled":     status.Enabled,
			"schedule":    status.Schedule,
			"description": status.Description,
			"last_run":    status.LastRun,
			"next_run":    status.NextRun,
			"is_running":  status.IsRunning,
			"last_error":  status.LastError,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jobs":  jobsList,
		"total": len(jobsList),
	})
}

// EnableDefaultJobHandler enables a default job
// POST /api/jobs/default/{name}/enable
func (h *JobHandler) EnableDefaultJobHandler(w http.ResponseWriter, r *http.Request) {
	// Extract job name from path: /api/jobs/default/{name}/enable
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 4 {
		http.Error(w, "Job name is required", http.StatusBadRequest)
		return
	}
	jobName := pathParts[3]

	if err := h.schedulerService.EnableJob(jobName); err != nil {
		h.logger.Error().Err(err).Str("job_name", jobName).Msg("Failed to enable job")
		http.Error(w, fmt.Sprintf("Failed to enable job: %v", err), http.StatusInternalServerError)
		return
	}

	h.logger.Info().Str("job_name", jobName).Msg("Job enabled")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_name": jobName,
		"enabled":  true,
		"message":  "Job enabled successfully",
	})
}

// DisableDefaultJobHandler disables a default job
// POST /api/jobs/default/{name}/disable
func (h *JobHandler) DisableDefaultJobHandler(w http.ResponseWriter, r *http.Request) {
	// Extract job name from path: /api/jobs/default/{name}/disable
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 4 {
		http.Error(w, "Job name is required", http.StatusBadRequest)
		return
	}
	jobName := pathParts[3]

	if err := h.schedulerService.DisableJob(jobName); err != nil {
		h.logger.Error().Err(err).Str("job_name", jobName).Msg("Failed to disable job")
		http.Error(w, fmt.Sprintf("Failed to disable job: %v", err), http.StatusInternalServerError)
		return
	}

	h.logger.Info().Str("job_name", jobName).Msg("Job disabled")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_name": jobName,
		"enabled":  false,
		"message":  "Job disabled successfully",
	})
}

// UpdateDefaultJobScheduleHandler updates a default job's schedule
// PUT /api/jobs/default/{name}/schedule
func (h *JobHandler) UpdateDefaultJobScheduleHandler(w http.ResponseWriter, r *http.Request) {
	// Extract job name from path: /api/jobs/default/{name}/schedule
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 4 {
		http.Error(w, "Job name is required", http.StatusBadRequest)
		return
	}
	jobName := pathParts[3]

	// Parse request body
	var req struct {
		Schedule string `json:"schedule"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Schedule == "" {
		http.Error(w, "Schedule is required", http.StatusBadRequest)
		return
	}

	// Validate schedule format
	if err := common.ValidateJobSchedule(req.Schedule); err != nil {
		http.Error(w, fmt.Sprintf("Invalid schedule: %v", err), http.StatusBadRequest)
		return
	}

	// Update job schedule
	if err := h.schedulerService.UpdateJobSchedule(jobName, req.Schedule); err != nil {
		h.logger.Error().Err(err).Str("job_name", jobName).Msg("Failed to update job schedule")
		http.Error(w, fmt.Sprintf("Failed to update schedule: %v", err), http.StatusInternalServerError)
		return
	}

	h.logger.Info().
		Str("job_name", jobName).
		Str("new_schedule", req.Schedule).
		Msg("Job schedule updated successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_name": jobName,
		"schedule": req.Schedule,
		"message":  "Schedule updated successfully",
	})
}

// deriveEntityType derives entity type based on source type
func (h *JobHandler) deriveEntityType(source *models.SourceConfig) string {
	switch source.Type {
	case models.SourceTypeJira:
		return "projects"
	case models.SourceTypeConfluence:
		return "spaces"
	case models.SourceTypeGithub:
		return "repos"
	default:
		return "all"
	}
}

// deriveSeedURLs derives seed URLs from source configuration
// Normalizes base URL using net/url to handle various path formats properly
func (h *JobHandler) deriveSeedURLs(source *models.SourceConfig) []string {
	// Parse base URL using net/url for proper normalization
	parsedURL, err := url.Parse(source.BaseURL)
	if err != nil {
		h.logger.Error().Err(err).Str("base_url", source.BaseURL).Msg("Failed to parse base URL")
		return []string{}
	}

	// Normalize the path by removing trailing slashes
	path := strings.TrimRight(parsedURL.Path, "/")

	// Check if path already contains /rest/ to avoid duplication
	if strings.Contains(path, "/rest/") {
		h.logger.Warn().
			Str("base_url", source.BaseURL).
			Msg("Base URL already contains /rest/ path, using as-is")
		return []string{source.BaseURL}
	}

	// Build base URL with scheme and host
	baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	switch source.Type {
	case models.SourceTypeJira:
		// For Jira, strip any path segments after host and add /rest/api/3/project
		// Handles: https://example.atlassian.net, https://example.atlassian.net/jira, https://example.atlassian.net/jira/
		return []string{fmt.Sprintf("%s/rest/api/3/project", baseURL)}

	case models.SourceTypeConfluence:
		// For Confluence, preserve /wiki root path but drop any additional segments
		// Handles: https://example.atlassian.net/wiki, https://example.atlassian.net/wiki/, https://example.atlassian.net/wiki/home
		if strings.HasPrefix(path, "/wiki") {
			return []string{fmt.Sprintf("%s/wiki/rest/api/space", baseURL)}
		}
		// Fallback if /wiki not in path - add it
		return []string{fmt.Sprintf("%s/wiki/rest/api/space", baseURL)}

	case models.SourceTypeGithub:
		// GitHub repos endpoint (if filters specify org/user)
		if org, ok := source.Filters["org"].(string); ok {
			return []string{fmt.Sprintf("%s/orgs/%s/repos", baseURL, org)}
		}
		if user, ok := source.Filters["user"].(string); ok {
			return []string{fmt.Sprintf("%s/users/%s/repos", baseURL, user)}
		}
		return []string{}

	default:
		h.logger.Warn().Str("source_type", source.Type).Msg("Unknown source type, cannot derive seed URLs")
		return []string{}
	}
}
