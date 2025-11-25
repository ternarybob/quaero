// -----------------------------------------------------------------------
// Last Modified: Wednesday, 5th November 2025 8:25:01 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/logs"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// JobGroup represents a parent job with its children
type JobGroup struct {
	Parent   *models.QueueJobState
	Children []*models.QueueJobState
}

// JobHandler handles job-related API requests
type JobHandler struct {
	crawlerService   *crawler.Service
	jobStorage       interfaces.QueueStorage
	authStorage      interfaces.AuthStorage
	schedulerService interfaces.SchedulerService
	logService       interfaces.LogService
	jobManager       interfaces.JobManager
	config           *common.Config
	logger           arbor.ILogger
}

// NewJobHandler creates a new job handler
func NewJobHandler(crawlerService *crawler.Service, jobStorage interfaces.QueueStorage, authStorage interfaces.AuthStorage, schedulerService interfaces.SchedulerService, logService interfaces.LogService, jobManager interfaces.JobManager, config *common.Config, logger arbor.ILogger) *JobHandler {
	return &JobHandler{
		crawlerService:   crawlerService,
		jobStorage:       jobStorage,
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

	// Normalize orderBy field name to match struct field names (BadgerHold is case-sensitive)
	// Map common query parameter names to actual struct field names
	fieldNameMap := map[string]string{
		"created_at":   "CreatedAt",
		"updated_at":   "UpdatedAt",
		"started_at":   "StartedAt",
		"completed_at": "CompletedAt",
		"finished_at":  "FinishedAt",
		"status":       "Status",
		"name":         "Name",
		"type":         "Type",
	}

	if orderBy == "" {
		orderBy = "CreatedAt" // Default to CreatedAt
	} else if normalized, exists := fieldNameMap[strings.ToLower(orderBy)]; exists {
		orderBy = normalized // Normalize to struct field name
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
		if job.ParentID == nil || *job.ParentID == "" {
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
		} else {
			// Debug: Log child statistics
			for parentID, stats := range childStatsMap {
				h.logger.Debug().
					Str("parent_id", parentID).
					Int("child_count", stats.ChildCount).
					Int("pending", stats.PendingChildren).
					Int("running", stats.RunningChildren).
					Int("completed", stats.CompletedChildren).
					Int("failed", stats.FailedChildren).
					Msg("Child statistics for parent job")
			}
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
		// Enrich with statistics and job types
		enrichedJobs := make([]map[string]interface{}, 0, len(jobs))
		for _, job := range jobs {
			// Convert to map and add statistics
			jobMap := convertJobToMap(job)
			jobMap["parent_id"] = job.ParentID

			// Add child statistics
			var stats *interfaces.JobChildStats
			if s, exists := childStatsMap[job.ID]; exists {
				stats = s
				jobMap["child_count"] = stats.ChildCount
				jobMap["completed_children"] = stats.CompletedChildren
				jobMap["failed_children"] = stats.FailedChildren
				jobMap["cancelled_children"] = stats.CancelledChildren
				jobMap["pending_children"] = stats.PendingChildren
				jobMap["running_children"] = stats.RunningChildren
			} else {
				jobMap["child_count"] = 0
				jobMap["completed_children"] = 0
				jobMap["failed_children"] = 0
				jobMap["cancelled_children"] = 0
				jobMap["pending_children"] = 0
				jobMap["running_children"] = 0
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
	var orphanJobs []*models.QueueJobState

	if grouped && parentID != "" && parentID != "root" {
		// Fetch the parent job to ensure it's in the response
		parentJob, err := h.jobManager.GetJob(ctx, parentID)
		if err == nil {
			if p, ok := parentJob.(*models.QueueJobState); ok {
				groupsMap[p.ID] = &JobGroup{
					Parent:   p,
					Children: make([]*models.QueueJobState, 0),
				}
			}
		}
	}

	for _, job := range jobs {
		if job.ParentID == nil || *job.ParentID == "" {
			// This is a parent job
			if _, exists := groupsMap[job.ID]; !exists {
				groupsMap[job.ID] = &JobGroup{
					Parent:   job,
					Children: make([]*models.QueueJobState, 0),
				}
			}
		} else {
			// This is a child job
			if group, exists := groupsMap[*job.ParentID]; exists {
				group.Children = append(group.Children, job)
			} else {
				// Parent not in current page, treat as orphan
				orphanJobs = append(orphanJobs, job)
			}
		}
	}

	// Convert to array and enrich with statistics
	groups := make([]map[string]interface{}, 0, len(groupsMap))
	for parentID, group := range groupsMap {
		parentMap := convertJobToMap(group.Parent)
		parentMap["parent_id"] = group.Parent.ParentID

		// Add statistics
		var stats *interfaces.JobChildStats
		if s, exists := childStatsMap[parentID]; exists {
			stats = s
			parentMap["child_count"] = stats.ChildCount
			parentMap["completed_children"] = stats.CompletedChildren
			parentMap["failed_children"] = stats.FailedChildren
			parentMap["cancelled_children"] = stats.CancelledChildren
			parentMap["pending_children"] = stats.PendingChildren
			parentMap["running_children"] = stats.RunningChildren
		} else {
			parentMap["child_count"] = 0
			parentMap["completed_children"] = 0
			parentMap["failed_children"] = 0
			parentMap["cancelled_children"] = 0
			parentMap["pending_children"] = 0
			parentMap["running_children"] = 0
		}

		groups = append(groups, map[string]interface{}{
			"parent":   parentMap,
			"children": group.Children,
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
	jobState, ok := jobInterface.(*models.QueueJobState)
	if !ok {
		http.Error(w, "Invalid job type", http.StatusInternalServerError)
		return
	}

	// For parent jobs (empty parent_id), enrich with child statistics
	if jobState.ParentID == nil || *jobState.ParentID == "" {
		childStatsMap, err := h.jobManager.GetJobChildStats(ctx, []string{jobState.ID})
		if err != nil {
			h.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to get child statistics, continuing without stats")
		}

		// Convert to map for enrichment
		jobMap := convertJobToMap(jobState)
		jobMap["parent_id"] = jobState.ParentID

		// Add child statistics
		var stats *interfaces.JobChildStats
		if s, exists := childStatsMap[jobState.ID]; exists {
			stats = s
			jobMap["child_count"] = stats.ChildCount
			jobMap["completed_children"] = stats.CompletedChildren
			jobMap["failed_children"] = stats.FailedChildren
			jobMap["cancelled_children"] = stats.CancelledChildren
			jobMap["pending_children"] = stats.PendingChildren
			jobMap["running_children"] = stats.RunningChildren
		} else {
			jobMap["child_count"] = 0
			jobMap["completed_children"] = 0
			jobMap["failed_children"] = 0
			jobMap["cancelled_children"] = 0
			jobMap["pending_children"] = 0
			jobMap["running_children"] = 0
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jobMap)
		return
	}

	// For child jobs
	jobMap := convertJobToMap(jobState)
	jobMap["parent_id"] = jobState.ParentID

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobMap)
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

	// Parse level query parameter for server-side filtering
	level := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("level")))

	// Normalize level aliases to match storage layer conventions
	levelAliases := map[string]string{
		"warning": "warn",  // "warning" → "warn"
		"err":     "error", // "err" → "error"
	}
	if normalized, exists := levelAliases[level]; exists {
		level = normalized
	}

	// Fetch logs with optional level filtering
	var logs []models.JobLogEntry
	var err error

	// If level is specified and valid, use level filtering
	if level != "" && level != "all" {
		// Validate level is one of: error, warn, info, debug (and accept aliases)
		validLevels := map[string]bool{
			"error": true,
			"warn":  true,
			"info":  true,
			"debug": true,
		}

		if !validLevels[level] {
			h.logger.Warn().Str("job_id", jobID).Str("level", level).Msg("Invalid log level requested")
			http.Error(w, "Invalid log level. Valid levels are: error, warn/warning, info, debug, all", http.StatusBadRequest)
			return
		}

		h.logger.Debug().Str("job_id", jobID).Str("level", level).Msg("Fetching logs with level filter")
		logs, err = h.logService.GetLogsByLevel(ctx, jobID, level, 1000)
		if err != nil {
			// Fall back to all logs if level filtering fails
			h.logger.Warn().Err(err).Str("job_id", jobID).Str("level", level).Msg("Failed to get logs by level, falling back to all logs")
			logs, err = h.logService.GetLogs(ctx, jobID, 1000)
			if err != nil {
				h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job logs")
				http.Error(w, "Failed to get job logs", http.StatusInternalServerError)
				return
			}
			level = "all" // Update level to reflect actual response
		}
	} else {
		// Fetch all logs (no filtering)
		logs, err = h.logService.GetLogs(ctx, jobID, 1000) // Limit to 1000 most recent logs
		if err != nil {
			h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job logs")
			http.Error(w, "Failed to get job logs", http.StatusInternalServerError)
			return
		}
		if level == "" {
			level = "all" // Set default value for response
		}
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
		"level":  level, // Include level filter applied (or "all" if no filter)
	})
}

// GetAggregatedJobLogsHandler returns aggregated logs for a job and its children
// GET /api/jobs/{id}/logs/aggregated?level=error&limit=500&include_children=true&order=asc
func (h *JobHandler) GetAggregatedJobLogsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract job ID from path: /api/jobs/{id}/logs/aggregated
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

	// Parse query parameters
	// level: Log level filter (error, warn, info, debug, all) - default: "all"
	level := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("level")))
	if level == "" {
		level = "all"
	}

	// limit: Max logs to return - default: 1000
	limit := 1000
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	// include_children: Include child job logs - default: true
	includeChildren := true
	if includeChildrenStr := r.URL.Query().Get("include_children"); includeChildrenStr != "" {
		includeChildren = includeChildrenStr == "true"
	}

	// order: Sort order (asc=oldest-first, desc=newest-first) - default: "asc"
	order := r.URL.Query().Get("order")
	if order == "" {
		order = "asc"
	}

	// cursor: Pagination cursor (opaque, base64-encoded) - default: ""
	cursor := r.URL.Query().Get("cursor")

	// Normalize level aliases to match storage layer conventions
	levelAliases := map[string]string{
		"warning": "warn",  // "warning" → "warn"
		"err":     "error", // "err" → "error"
	}
	if normalized, exists := levelAliases[level]; exists {
		level = normalized
	}

	// Validate level parameter
	if level != "all" {
		validLevels := map[string]bool{
			"error": true,
			"warn":  true,
			"info":  true,
			"debug": true,
		}
		if !validLevels[level] {
			h.logger.Warn().Str("job_id", jobID).Str("level", level).Msg("Invalid log level requested")
			http.Error(w, "Invalid log level. Valid levels are: error, warn/warning, info, debug, all", http.StatusBadRequest)
			return
		}
	}

	// Fetch aggregated logs with pagination support (Comment 2)
	logEntries, metadata, nextCursor, err := h.logService.GetAggregatedLogs(ctx, jobID, includeChildren, level, limit, cursor, order)
	if err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get aggregated logs")
		// Check for job not found using errors.Is (Comment 3)
		if errors.Is(err, logs.ErrJobNotFound) {
			http.Error(w, "Job not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to get aggregated logs", http.StatusInternalServerError)
		}
		return
	}

	// Enrich logs with job context from metadata using AssociatedJobID (Comment 1)
	enrichedLogs := make([]map[string]interface{}, 0, len(logEntries))
	for _, log := range logEntries {
		// Use AssociatedJobID to find the correct metadata for this log
		enrichedLog := map[string]interface{}{
			"timestamp":      log.Timestamp,
			"full_timestamp": log.FullTimestamp,
			"level":          log.Level,
			"message":        log.Message,
			"job_id":         log.AssociatedJobID,
		}

		// Find metadata for the job that produced this log
		if meta, exists := metadata[log.AssociatedJobID]; exists {
			enrichedLog["job_name"] = meta.JobName
			enrichedLog["job_url"] = meta.JobURL
			enrichedLog["job_depth"] = meta.JobDepth
			enrichedLog["job_type"] = meta.JobType
			enrichedLog["parent_id"] = meta.ParentID
		} else {
			// Use default values if no metadata found
			enrichedLog["job_name"] = fmt.Sprintf("Job %s", log.AssociatedJobID)
			enrichedLog["job_type"] = "unknown"
			enrichedLog["parent_id"] = ""
		}

		enrichedLogs = append(enrichedLogs, enrichedLog)
	}

	// Apply ordering: logs come from LogService in oldest-first order (asc)
	// If desc requested, reverse the slice
	if order == "desc" {
		// Reverse slice for newest-first ordering
		for i, j := 0, len(enrichedLogs)-1; i < j; i, j = i+1, j-1 {
			enrichedLogs[i], enrichedLogs[j] = enrichedLogs[j], enrichedLogs[i]
		}
	}

	// Build response
	response := map[string]interface{}{
		"job_id":           jobID,
		"logs":             enrichedLogs,
		"count":            len(enrichedLogs),
		"order":            order,
		"level":            level,
		"include_children": includeChildren,
		"metadata":         metadata,
		"next_cursor":      nextCursor, // For pagination
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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

// DeleteJobErrorResponse represents a structured error response for job deletion
type DeleteJobErrorResponse struct {
	Error      string `json:"error"`
	Details    string `json:"details,omitempty"`
	JobID      string `json:"job_id,omitempty"`
	Status     string `json:"status,omitempty"`
	ChildCount int    `json:"child_count,omitempty"`
}

// writeDeleteError writes a structured JSON error response
func (h *JobHandler) writeDeleteError(w http.ResponseWriter, statusCode int, errorMsg, details, jobID, jobStatus string, childCount int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := DeleteJobErrorResponse{
		Error:      errorMsg,
		Details:    details,
		JobID:      jobID,
		Status:     jobStatus,
		ChildCount: childCount,
	}

	json.NewEncoder(w).Encode(response)

	h.logger.Error().
		Str("job_id", jobID).
		Str("status", jobStatus).
		Int("child_count", childCount).
		Str("error", errorMsg).
		Str("details", details).
		Msg("Job deletion error")
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

	// Check if job exists and get its status
	jobInterface, err := h.jobManager.GetJob(ctx, jobID)
	if err != nil {
		// Job not found
		h.logger.Debug().Str("job_id", jobID).Msg("Job not found for deletion")
		h.writeDeleteError(w, 404, "Job not found", fmt.Sprintf("Job %s does not exist", jobID), jobID, "", 0)
		return
	}

	// Type assert to get job details
	jobState, ok := jobInterface.(*models.QueueJobState)
	if !ok {
		h.logger.Error().Str("job_id", jobID).Msg("Invalid job type retrieved")
		h.writeDeleteError(w, 500, "Internal server error", "Invalid job type", jobID, "", 0)
		return
	}

	// Check if job is running (but allow deletion of orchestrating jobs)
	// Orchestrating jobs are parent jobs monitoring children - deletion will cancel them
	if jobState.Status == models.JobStatusRunning && jobState.Type != "parent" {
		h.logger.Warn().Str("job_id", jobID).Msg("Attempt to delete running job blocked")
		h.writeDeleteError(w, 400, "Cannot delete running job", fmt.Sprintf("Job %s is currently running. Cancel it first.", jobID), jobID, string(jobState.Status), 0)
		return
	}

	// If job is orchestrating (parent job), cancel it and all children first before deletion
	if jobState.Status == models.JobStatusRunning && jobState.Type == "parent" {
		h.logger.Info().Str("job_id", jobID).Msg("Cancelling orchestrating job and children before deletion")

		// Cancel all child jobs first
		childrenCancelled, err := h.jobManager.StopAllChildJobs(ctx, jobID)
		if err != nil {
			h.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to cancel child jobs")
		} else {
			h.logger.Info().Str("job_id", jobID).Int("children_cancelled", childrenCancelled).Msg("Cancelled child jobs")
		}

		// Cancel the parent job by updating its status
		jobState.Status = models.JobStatusCancelled
		if err := h.jobManager.UpdateJob(ctx, jobState); err != nil {
			h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to cancel orchestrating job")
			h.writeDeleteError(w, 500, "Failed to cancel job", err.Error(), jobID, string(jobState.Status), 0)
			return
		}

		h.logger.Info().Str("job_id", jobID).Msg("Orchestrating job cancelled successfully")
	}

	// Get child count for response
	childStatsMap, err := h.jobManager.GetJobChildStats(ctx, []string{jobID})
	childCount := 0
	if err == nil {
		if stats, exists := childStatsMap[jobID]; exists {
			childCount = stats.ChildCount
		}
	}

	// Log deletion attempt
	h.logger.Info().
		Str("job_id", jobID).
		Int("child_count", childCount).
		Msg("Attempting to delete job")

	// Attempt to delete job
	cascadeDeleted, err := h.jobManager.DeleteJob(ctx, jobID)
	if err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to delete job")
		// Determine error type and return structured response
		errorMsg := err.Error()
		if strings.Contains(strings.ToLower(errorMsg), "running") {
			h.writeDeleteError(w, 400, "Cannot delete running job", errorMsg, jobID, string(jobState.Status), childCount)
		} else if strings.Contains(strings.ToLower(errorMsg), "not found") {
			h.writeDeleteError(w, 404, "Job not found", errorMsg, jobID, "", childCount)
		} else {
			h.writeDeleteError(w, 500, "Failed to delete job", errorMsg, jobID, string(jobState.Status), childCount)
		}
		return
	}

	// Log successful deletion
	h.logger.Info().
		Str("job_id", jobID).
		Int("cascade_deleted", cascadeDeleted).
		Int("child_count", childCount).
		Msg("Job deleted successfully")

	// Return success response with cascade info
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_id":          jobID,
		"message":         "Job deleted successfully",
		"cascade_deleted": cascadeDeleted,
		"child_count":     childCount,
	})
}

// GetJobStatsHandler returns statistics about jobs
// GET /api/jobs/stats
// NOTE: Counts ALL jobs (parent + children) to show total queue depth
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

	// Convert jobs to enriched maps to extract document_count from metadata
	// This ensures the queue endpoint returns consistent data with document_count field
	pendingJobs := make([]map[string]interface{}, 0, len(pendingJobsInterface))
	for _, queueJob := range pendingJobsInterface {
		// Convert QueueJob to QueueJobState for convertJobToMap compatibility
		jobState := models.NewQueueJobState(queueJob)
		jobMap := convertJobToMap(jobState)
		jobMap["parent_id"] = queueJob.ParentID
		pendingJobs = append(pendingJobs, jobMap)
	}

	runningJobs := make([]map[string]interface{}, 0, len(runningJobsInterface))
	for _, queueJob := range runningJobsInterface {
		// Convert QueueJob to QueueJobState for convertJobToMap compatibility
		jobState := models.NewQueueJobState(queueJob)
		jobMap := convertJobToMap(jobState)
		jobMap["parent_id"] = queueJob.ParentID
		runningJobs = append(runningJobs, jobMap)
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

// convertJobToMap converts a QueueJobState struct to a map for JSON response enrichment
// IMPORTANT: Converts the Config field to a flexible map[string]interface{} for executor-agnostic display
// Extracts document_count from metadata for easier UI access
func convertJobToMap(jobState *models.QueueJobState) map[string]interface{} {
	// Marshal to JSON then unmarshal to map to preserve all fields and JSON tags
	data, err := json.Marshal(jobState)
	if err != nil {
		return map[string]interface{}{"id": jobState.ID, "error": "failed to serialize job"}
	}

	var jobMap map[string]interface{}
	if err := json.Unmarshal(data, &jobMap); err != nil {
		return map[string]interface{}{"id": jobState.ID, "error": "failed to deserialize job"}
	}

	// Convert the config field from CrawlConfig struct to map[string]interface{}
	// This ensures the API returns executor-agnostic configuration as key-value pairs
	if configInterface, ok := jobMap["config"]; ok {
		// Config is already a map[string]interface{} after JSON round-trip
		// Ensure it's displayed as a flexible object in the UI
		jobMap["config"] = configInterface
	}

	// Extract document_count from metadata for easier access in UI
	// This ensures completed parent jobs retain their document count after page reload
	if metadataInterface, ok := jobMap["metadata"]; ok {
		if metadata, ok := metadataInterface.(map[string]interface{}); ok {
			if documentCount, ok := metadata["document_count"]; ok {
				// Handle both float64 (from JSON unmarshal) and int types
				if floatVal, ok := documentCount.(float64); ok {
					jobMap["document_count"] = int(floatVal)
				} else if intVal, ok := documentCount.(int); ok {
					jobMap["document_count"] = intVal
				}
			}
		}
	}

	return jobMap
}
