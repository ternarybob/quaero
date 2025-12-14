// -----------------------------------------------------------------------
// Last Modified: Wednesday, 5th November 2025 8:25:01 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	queueManager     interfaces.QueueManager
	eventService     interfaces.EventService
	config           *common.Config
	logger           arbor.ILogger
}

// NewJobHandler creates a new job handler
func NewJobHandler(crawlerService *crawler.Service, jobStorage interfaces.QueueStorage, authStorage interfaces.AuthStorage, schedulerService interfaces.SchedulerService, logService interfaces.LogService, jobManager interfaces.JobManager, queueManager interfaces.QueueManager, eventService interfaces.EventService, config *common.Config, logger arbor.ILogger) *JobHandler {
	return &JobHandler{
		crawlerService:   crawlerService,
		jobStorage:       jobStorage,
		authStorage:      authStorage,
		schedulerService: schedulerService,
		logService:       logService,
		jobManager:       jobManager,
		queueManager:     queueManager,
		eventService:     eventService,
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
// GET /api/jobs/{id}/logs?order=desc&limit=100 (desc=newest-first, asc=oldest-first)
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

	// Parse limit query parameter (default: 1000, max: 5000)
	limit := 1000
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
			if limit > 5000 {
				limit = 5000 // Cap at 5000 to prevent memory issues
			}
		}
	}

	// Parse level query parameter for server-side filtering
	// Default: return INFO+ logs (excludes debug/trace) to reduce UI noise
	// Use level=all to get all logs including debug, or level=debug to get only debug logs
	level := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("level")))

	// Normalize level aliases to match storage layer conventions
	levelAliases := map[string]string{
		"warning": "warn",  // "warning" → "warn"
		"err":     "error", // "err" → "error"
	}
	if normalized, exists := levelAliases[level]; exists {
		level = normalized
	}

	// Fetch logs with level filtering
	var logs []models.LogEntry
	var err error

	// Validate level is one of: error, warn, info, debug, all (and accept aliases)
	validLevels := map[string]bool{
		"error": true,
		"warn":  true,
		"info":  true,
		"debug": true,
		"all":   true,
		"":      true, // Empty = default to info
	}

	if !validLevels[level] {
		h.logger.Warn().Str("job_id", jobID).Str("level", level).Msg("Invalid log level requested")
		http.Error(w, "Invalid log level. Valid levels are: error, warn/warning, info, debug, all", http.StatusBadRequest)
		return
	}

	if level == "all" {
		// Fetch ALL logs including debug (explicit request)
		logs, err = h.logService.GetLogs(ctx, jobID, limit)
		if err != nil {
			h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job logs")
			http.Error(w, "Failed to get job logs", http.StatusInternalServerError)
			return
		}
	} else {
		// Default to INFO+ if no level specified (excludes debug/trace for cleaner UI)
		// Or use specific level if provided
		filterLevel := level
		if filterLevel == "" {
			filterLevel = "info" // Default: INFO and above (excludes debug)
		}

		h.logger.Debug().Str("job_id", jobID).Str("level", filterLevel).Int("limit", limit).Msg("Fetching logs with level filter")
		logs, err = h.logService.GetLogsByLevel(ctx, jobID, filterLevel, limit)
		if err != nil {
			// Fall back to all logs if level filtering fails
			h.logger.Warn().Err(err).Str("job_id", jobID).Str("level", filterLevel).Msg("Failed to get logs by level, falling back to all logs")
			logs, err = h.logService.GetLogs(ctx, jobID, limit)
			if err != nil {
				h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job logs")
				http.Error(w, "Failed to get job logs", http.StatusInternalServerError)
				return
			}
			level = "all" // Update level to reflect actual response
		} else if level == "" {
			level = "info" // Set default value for response
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
		"limit":  limit, // Include limit in response
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

	// Enrich logs with job context from metadata using JobID (Comment 1)
	enrichedLogs := make([]map[string]interface{}, 0, len(logEntries))
	for _, log := range logEntries {
		// Use JobID to find the correct metadata for this log
		enrichedLog := map[string]interface{}{
			"timestamp":      log.Timestamp,
			"full_timestamp": log.FullTimestamp,
			"level":          log.Level,
			"message":        log.Message,
			"job_id":         log.JobID(),
			"step_name":      log.StepName(),   // Include step_name for UI filtering
			"source_type":    log.SourceType(), // Include source_type for worker context
			"originator":     log.Originator(), // Include originator for display context
		}

		// Find metadata for the job that produced this log
		if meta, exists := metadata[log.JobID()]; exists {
			enrichedLog["job_name"] = meta.JobName
			enrichedLog["job_url"] = meta.JobURL
			enrichedLog["job_depth"] = meta.JobDepth
			enrichedLog["job_type"] = meta.JobType
			enrichedLog["parent_id"] = meta.ParentID
		} else {
			// Use default values if no metadata found
			enrichedLog["job_name"] = fmt.Sprintf("Job %s", log.JobID())
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

	h.logger.Debug().Str("original_job_id", jobID).Str("new_job_id", newJobID).Msg("Job copied and queued")

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
	ctx := r.Context()

	// Extract job ID from path: /api/jobs/{id}/cancel
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Job ID is required"})
		return
	}
	jobID := pathParts[2]

	if jobID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Job ID is required"})
		return
	}

	// Try crawler service first (for legacy crawler jobs in activeJobs)
	err := h.crawlerService.CancelJob(jobID)
	if err == nil {
		h.logger.Debug().Str("job_id", jobID).Msg("Job cancelled via crawler service")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"job_id":  jobID,
			"message": "Job cancelled successfully",
		})
		return
	}

	// Crawler service failed - try queue-based cancellation
	h.logger.Debug().Err(err).Str("job_id", jobID).Msg("Crawler service cancel failed, trying queue-based cancellation")

	// Get job from storage to verify it exists and is running
	jobInterface, storageErr := h.jobStorage.GetJob(ctx, jobID)
	if storageErr != nil {
		h.logger.Error().Err(storageErr).Str("job_id", jobID).Msg("Job not found in storage")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Job not found"})
		return
	}

	// Type assert to QueueJobState
	jobState, ok := jobInterface.(*models.QueueJobState)
	if !ok {
		h.logger.Error().Str("job_id", jobID).Msg("Invalid job type - expected QueueJobState")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid job type"})
		return
	}

	// Check if job is in a cancellable state
	if jobState.Status != models.JobStatusRunning && jobState.Status != models.JobStatusPending {
		h.logger.Warn().Str("job_id", jobID).Str("status", string(jobState.Status)).Msg("Job is not in a cancellable state")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Job is not running (status: %s)", jobState.Status)})
		return
	}

	// Cancel the job via storage
	if err := h.jobStorage.UpdateJobStatus(ctx, jobID, string(models.JobStatusCancelled), "Cancelled by user"); err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to cancel job in storage")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update job status"})
		return
	}

	// Remove the job from the queue (if it's still pending)
	if h.queueManager != nil {
		deleted, err := h.queueManager.DeleteByJobID(ctx, jobID)
		if err != nil {
			h.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to remove job from queue")
		} else if deleted > 0 {
			h.logger.Debug().Str("job_id", jobID).Int("deleted", deleted).Msg("Removed job from queue")
		}
	}

	// Cancel all child jobs if this is a parent job
	if jobState.Type == "parent" || jobState.Type == "manager" || jobState.Type == "step" {
		// Update child job statuses (recursive - handles Manager → Step → Job hierarchy)
		// Also publishes EventJobCancelled for each running job so JobProcessor cancels them
		childrenCancelled, childErr := h.jobManager.StopAllChildJobs(ctx, jobID)
		if childErr != nil {
			h.logger.Warn().Err(childErr).Str("job_id", jobID).Msg("Failed to cancel child jobs")
		} else if childrenCancelled > 0 {
			h.logger.Debug().Str("job_id", jobID).Int("children_cancelled", childrenCancelled).Msg("Cancelled child jobs")
		}

		// Remove child jobs from the queue (recursive - handles nested children)
		if h.queueManager != nil {
			allChildIDs := h.collectAllChildJobIDs(ctx, jobID)
			if len(allChildIDs) > 0 {
				deleted, err := h.queueManager.DeleteByJobIDs(ctx, allChildIDs)
				if err != nil {
					h.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to remove child jobs from queue")
				} else if deleted > 0 {
					h.logger.Debug().Str("job_id", jobID).Int("deleted", deleted).Int("total_children", len(allChildIDs)).Msg("Removed child jobs from queue")
				}
			}
		}
	}

	// Publish EventJobCancelled for WebSocket broadcast
	if h.eventService != nil {
		cancelledEvent := interfaces.Event{
			Type: interfaces.EventJobCancelled,
			Payload: map[string]interface{}{
				"job_id":    jobID,
				"status":    "cancelled",
				"name":      jobState.Name,
				"timestamp": time.Now(),
			},
		}
		if err := h.eventService.Publish(ctx, cancelledEvent); err != nil {
			h.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to publish job cancelled event")
		}
	}

	h.logger.Debug().Str("job_id", jobID).Msg("Job cancelled via storage")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_id":  jobID,
		"message": "Job cancelled successfully",
	})
}

// collectAllChildJobIDs recursively collects all child job IDs for a parent job.
// Handles the Manager → Step → Job hierarchy by recursively traversing step jobs.
func (h *JobHandler) collectAllChildJobIDs(ctx context.Context, parentID string) []string {
	var allIDs []string

	childJobs, err := h.jobStorage.GetChildJobs(ctx, parentID)
	if err != nil || len(childJobs) == 0 {
		return allIDs
	}

	for _, child := range childJobs {
		allIDs = append(allIDs, child.ID)

		// Recursively collect children of step/manager/parent jobs
		if child.Type == "step" || child.Type == "manager" || child.Type == "parent" {
			nestedIDs := h.collectAllChildJobIDs(ctx, child.ID)
			allIDs = append(allIDs, nestedIDs...)
		}
	}

	return allIDs
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

	h.logger.Debug().Str("original_job_id", jobID).Str("new_job_id", newJobID).Msg("Job copied")

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
		h.logger.Debug().Str("job_id", jobID).Msg("Cancelling orchestrating job and children before deletion")

		// Cancel all child jobs first
		childrenCancelled, err := h.jobManager.StopAllChildJobs(ctx, jobID)
		if err != nil {
			h.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to cancel child jobs")
		} else {
			h.logger.Debug().Str("job_id", jobID).Int("children_cancelled", childrenCancelled).Msg("Cancelled child jobs")
		}

		// Cancel the parent job by updating its status
		jobState.Status = models.JobStatusCancelled
		if err := h.jobManager.UpdateJob(ctx, jobState); err != nil {
			h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to cancel orchestrating job")
			h.writeDeleteError(w, 500, "Failed to cancel job", err.Error(), jobID, string(jobState.Status), 0)
			return
		}

		h.logger.Debug().Str("job_id", jobID).Msg("Orchestrating job cancelled successfully")
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
	h.logger.Debug().
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
	h.logger.Debug().
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

	h.logger.Debug().
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

// JobTreeResponse represents a GitHub Actions-style tree view of a job
type JobTreeResponse struct {
	JobID      string        `json:"job_id"`
	JobName    string        `json:"job_name"`
	Status     string        `json:"status"`
	DurationMs int64         `json:"duration_ms"`
	StartedAt  *time.Time    `json:"started_at,omitempty"`
	FinishedAt *time.Time    `json:"finished_at,omitempty"`
	Steps      []JobTreeStep `json:"steps"`
}

// JobTreeStep represents a step in the job tree
type JobTreeStep struct {
	StepID       string           `json:"step_id,omitempty"` // Step job ID (for fetching more logs)
	Name         string           `json:"name"`
	Status       string           `json:"status"`
	DurationMs   int64            `json:"duration_ms"`
	StartedAt    *time.Time       `json:"started_at,omitempty"`
	FinishedAt   *time.Time       `json:"finished_at,omitempty"`
	Expanded     bool             `json:"expanded"`
	ChildSummary *ChildJobSummary `json:"child_summary,omitempty"`
	Logs         []JobTreeLog     `json:"logs"`
	TotalLogs    int              `json:"total_logs,omitempty"` // Total log count for "Show earlier logs"
}

// ChildJobSummary aggregates child job status counts
type ChildJobSummary struct {
	Total       int          `json:"total"`
	Completed   int          `json:"completed"`
	Failed      int          `json:"failed"`
	Cancelled   int          `json:"cancelled"`
	Running     int          `json:"running"`
	Pending     int          `json:"pending"`
	ErrorGroups []ErrorGroup `json:"error_groups,omitempty"`
}

// ErrorGroup groups similar errors by message
type ErrorGroup struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

// JobTreeLog represents a log line in the tree view
type JobTreeLog struct {
	LineNumber int    `json:"line_number"` // Per-job line number (1-based)
	Level      string `json:"level"`
	Text       string `json:"text"`
}

// JobStructureResponse represents a lightweight job structure for UI status updates
// This is a simplified version of JobTreeResponse without logs or detailed child info
type JobStructureResponse struct {
	JobID     string       `json:"job_id"`
	Status    string       `json:"status"`
	Steps     []StepStatus `json:"steps"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// StepStatus represents minimal step status info for the structure endpoint
type StepStatus struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	LogCount   int    `json:"log_count"`
	ChildCount int    `json:"child_count,omitempty"`
}

// GetJobTreeHandler returns a GitHub Actions-style tree view of a job
// GET /api/jobs/{id}/tree
//
// The tree view is built from step_definitions in the parent job's metadata.
// Each step definition corresponds to a step job (child of the manager job).
// Grandchildren of step jobs are counted in ChildSummary.
func (h *JobHandler) GetJobTreeHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract job ID from path: /api/jobs/{id}/tree
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

	// Get the parent job (manager job)
	jobInterface, err := h.jobManager.GetJob(ctx, jobID)
	if err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job for tree view")
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	parentJob, ok := jobInterface.(*models.QueueJobState)
	if !ok {
		http.Error(w, "Invalid job type", http.StatusInternalServerError)
		return
	}

	// Calculate duration
	var durationMs int64
	if parentJob.StartedAt != nil {
		endTime := time.Now()
		if parentJob.FinishedAt != nil {
			endTime = *parentJob.FinishedAt
		}
		durationMs = endTime.Sub(*parentJob.StartedAt).Milliseconds()
	}

	// Get step_definitions from parent job metadata - this is the source of truth
	var stepDefinitions []map[string]interface{}
	if parentJob.Metadata != nil {
		if defs, ok := parentJob.Metadata["step_definitions"]; ok {
			// Handle both []map[string]interface{} and []interface{} types
			switch v := defs.(type) {
			case []map[string]interface{}:
				stepDefinitions = v
			case []interface{}:
				for _, item := range v {
					if m, ok := item.(map[string]interface{}); ok {
						stepDefinitions = append(stepDefinitions, m)
					}
				}
			}
		}
	}

	// Get child jobs (step jobs) for this parent
	childOpts := &interfaces.JobListOptions{
		ParentID: jobID,
		Limit:    1000,
		Offset:   0,
	}
	stepJobs, err := h.jobStorage.ListJobs(ctx, childOpts)
	if err != nil {
		h.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to get step jobs for tree")
		stepJobs = []*models.QueueJobState{}
	}

	// Create a map of step_name -> step job for quick lookup
	stepJobMap := make(map[string]*models.QueueJobState)
	for _, stepJob := range stepJobs {
		if stepJob.Metadata != nil {
			if stepName, ok := stepJob.Metadata["step_name"].(string); ok && stepName != "" {
				stepJobMap[stepName] = stepJob
			}
		}
	}

	// Get current_step_name from parent metadata for expansion logic
	var currentStepName string
	if parentJob.Metadata != nil {
		if csn, ok := parentJob.Metadata["current_step_name"].(string); ok {
			currentStepName = csn
		}
	}

	// Build steps from step_definitions
	steps := make([]JobTreeStep, 0, len(stepDefinitions))

	for _, stepDef := range stepDefinitions {
		stepName, _ := stepDef["name"].(string)
		if stepName == "" {
			stepName = "unknown"
		}

		// Find matching step job
		stepJob := stepJobMap[stepName]

		// Build step from step definition and step job (if found)
		step := JobTreeStep{
			Name:     stepName,
			Status:   "pending", // Default if no step job found
			Expanded: false,
			ChildSummary: nil,
			Logs: []JobTreeLog{},
		}

		if stepJob != nil {
			// Set step job ID for "Show earlier logs" functionality
			step.StepID = stepJob.ID

			// Use step job's own status directly
			step.Status = string(stepJob.Status)
			step.StartedAt = stepJob.StartedAt
			step.FinishedAt = stepJob.FinishedAt
			// Expanded is set AFTER logs are fetched (see below)

			// Calculate step duration
			if stepJob.StartedAt != nil {
				endTime := time.Now()
				if stepJob.FinishedAt != nil {
					endTime = *stepJob.FinishedAt
				}
				step.DurationMs = endTime.Sub(*stepJob.StartedAt).Milliseconds()
			}

			// Backend-driven expansion: expand if failed, running, or is current step.
			// This endpoint intentionally does not load logs/child summaries to remain fast;
			// logs are fetched via `/api/jobs/{id}/tree/logs` and `/api/logs`.
			isRunning := stepJob.Status == models.JobStatusRunning
			isFailed := stepJob.Status == models.JobStatusFailed
			isCurrentStep := stepName == currentStepName
			step.Expanded = isFailed || isRunning || isCurrentStep
		}

		steps = append(steps, step)
	}

	// If no step_definitions, fall back to discovering steps from step jobs
	if len(stepDefinitions) == 0 && len(stepJobs) > 0 {
		steps = h.buildStepsFromStepJobs(ctx, stepJobs)
	}

	// If still no steps, create a single step from parent job logs
	if len(steps) == 0 {
		rawLogs, err := h.logService.GetLogs(ctx, jobID, 100)
		if err != nil {
			h.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to get logs for tree")
		}

		// Logs from storage are in DESC order (newest first)
		// Reverse to get ASC order for display (oldest at top)
		parentLogs := make([]JobTreeLog, 0, len(rawLogs))
		for i := len(rawLogs) - 1; i >= 0; i-- {
			parentLogs = append(parentLogs, JobTreeLog{
				Level: rawLogs[i].Level,
				Text:  rawLogs[i].Message,
			})
		}

		steps = append(steps, JobTreeStep{
			Name:       parentJob.Name,
			Status:     string(parentJob.Status),
			DurationMs: durationMs,
			StartedAt:  parentJob.StartedAt,
			FinishedAt: parentJob.FinishedAt,
			Expanded:   true,
			Logs:       parentLogs,
		})
	}

	response := JobTreeResponse{
		JobID:      jobID,
		JobName:    parentJob.Name,
		Status:     string(parentJob.Status),
		DurationMs: durationMs,
		StartedAt:  parentJob.StartedAt,
		FinishedAt: parentJob.FinishedAt,
		Steps:      steps,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetJobTreeLogsHandler returns logs for a job's tree view, grouped by step
// GET /api/jobs/{id}/tree/logs?step=step_name&limit=100
//
// If step is provided, returns logs only for that step.
// Otherwise returns logs for all steps, grouped by step_name.
func (h *JobHandler) GetJobTreeLogsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract job ID from path: /api/jobs/{id}/tree/logs
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
	stepFilter := r.URL.Query().Get("step")
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
			if limit > 20000 {
				limit = 20000
			}
		}
	}
	level := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("level")))
	if level == "" {
		level = "all"
	}
	// Normalize aliases
	switch level {
	case "warning":
		level = "warn"
	case "err":
		level = "error"
	}
	validLevels := map[string]bool{
		"error": true,
		"warn":  true,
		"info":  true,
		"debug": true,
		"all":   true,
	}
	if !validLevels[level] {
		http.Error(w, "Invalid log level. Valid levels are: error, warn, info, debug, all", http.StatusBadRequest)
		return
	}

	// Get the parent job (manager job) from storage for authoritative status.
	// This avoids races where the in-memory runtime state lags behind persisted status.
	jobInterface, err := h.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	parentJob, ok := jobInterface.(*models.QueueJobState)
	if !ok {
		http.Error(w, "Invalid job type", http.StatusInternalServerError)
		return
	}

	// Get step_definitions from parent job metadata
	var stepDefinitions []map[string]interface{}
	if parentJob.Metadata != nil {
		if defs, ok := parentJob.Metadata["step_definitions"]; ok {
			switch v := defs.(type) {
			case []map[string]interface{}:
				stepDefinitions = v
			case []interface{}:
				for _, item := range v {
					if m, ok := item.(map[string]interface{}); ok {
						stepDefinitions = append(stepDefinitions, m)
					}
				}
			}
		}
	}

	// Get child jobs (step jobs) for this parent
	childOpts := &interfaces.JobListOptions{
		ParentID: jobID,
		Limit:    1000,
		Offset:   0,
	}
	stepJobs, err := h.jobStorage.ListJobs(ctx, childOpts)
	if err != nil {
		stepJobs = []*models.QueueJobState{}
	}

	// Create a map of step_name -> step job for quick lookup
	stepJobMap := make(map[string]*models.QueueJobState)
	for _, stepJob := range stepJobs {
		if stepJob.Metadata != nil {
			if stepName, ok := stepJob.Metadata["step_name"].(string); ok && stepName != "" {
				stepJobMap[stepName] = stepJob
			}
		}
	}

	// Response structure
	type StepLogs struct {
		StepName        string       `json:"step_name"`
		StepID          string       `json:"step_id,omitempty"`
		Status          string       `json:"status"`
		Logs            []JobTreeLog `json:"logs"`
		TotalCount      int          `json:"total_count"`      // Total logs matching level filter (for pagination)
		UnfilteredCount int          `json:"unfiltered_count"` // Total logs regardless of level filter (for UI display)
	}

	response := struct {
		JobID string     `json:"job_id"`
		Steps []StepLogs `json:"steps"`
	}{
		JobID: jobID,
		Steps: []StepLogs{},
	}

	// Build logs response for each step
	for _, stepDef := range stepDefinitions {
		stepName, _ := stepDef["name"].(string)
		if stepName == "" {
			continue
		}

		// If step filter is set, skip non-matching steps
		if stepFilter != "" && stepName != stepFilter {
			continue
		}

		stepJob := stepJobMap[stepName]
		stepLogs := StepLogs{
			StepName:   stepName,
			Status:     "pending",
			Logs:       []JobTreeLog{},
			TotalCount: 0,
		}

		if stepJob != nil {
			stepLogs.StepID = stepJob.ID
			stepLogs.Status = string(stepJob.Status)

			// Resolve total_count and newest logs using the same level semantics as /api/logs:
			// - all/debug: include all levels
			// - info: include info + warn + error (exclude debug)
			// - warn: include warn + error
			// - error: include error only
			type levelSlice struct {
				level string
				logs  []models.LogEntry
				i     int
			}

			getIncludedLevels := func(filter string) []string {
				switch filter {
				case "error":
					return []string{"error"}
				case "warn":
					return []string{"warn", "error"}
				case "info":
					return []string{"info", "warn", "error"}
				case "debug", "all":
					return []string{"debug", "info", "warn", "error"}
				default:
					return []string{"debug", "info", "warn", "error"}
				}
			}

			// Count ALL logs (unfiltered) for UI display
			unfilteredCount, unfilteredErr := h.logService.CountLogs(ctx, stepJob.ID)
			if unfilteredErr != nil {
				h.logger.Warn().Err(unfilteredErr).Str("step_job_id", stepJob.ID).Msg("Failed to count unfiltered logs for step")
			} else {
				stepLogs.UnfilteredCount = unfilteredCount
			}

			// Count logs matching the filter (for pagination)
			totalCount := 0
			var countErr error
			if level == "all" || level == "debug" {
				totalCount = unfilteredCount // Same as unfiltered when no filter applied
			} else {
				for _, lv := range getIncludedLevels(level) {
					// CountLogsByLevel expects exact level; we sum included levels for hierarchical filters.
					n, err := h.logService.CountLogsByLevel(ctx, stepJob.ID, lv)
					if err != nil {
						countErr = err
						break
					}
					totalCount += n
				}
			}
			if countErr != nil {
				h.logger.Warn().Err(countErr).Str("step_job_id", stepJob.ID).Str("level", level).Msg("Failed to count logs for step")
			} else {
				stepLogs.TotalCount = totalCount
			}

			// Fetch newest logs matching the filter (bounded by limit).
			// NOTE: Storage returns DESC order (newest first). We'll reverse for display.
			var newest []models.LogEntry
			if level == "all" || level == "debug" {
				newest, _ = h.logService.GetLogsWithOffset(ctx, stepJob.ID, limit, 0)
			} else if level == "error" {
				newest, _ = h.logService.GetLogsByLevel(ctx, stepJob.ID, "error", limit)
			} else {
				included := getIncludedLevels(level)
				parts := make([]levelSlice, 0, len(included))
				for _, lv := range included {
					part, err := h.logService.GetLogsByLevel(ctx, stepJob.ID, lv, limit)
					if err != nil {
						continue
					}
					parts = append(parts, levelSlice{level: lv, logs: part, i: 0})
				}

				// K-way merge by line number (newest first).
				merged := make([]models.LogEntry, 0, limit)
				for len(merged) < limit {
					bestIdx := -1
					bestLine := -1
					for pi := range parts {
						if parts[pi].i >= len(parts[pi].logs) {
							continue
						}
						ln := parts[pi].logs[parts[pi].i].LineNumber
						if ln > bestLine {
							bestLine = ln
							bestIdx = pi
						}
					}
					if bestIdx < 0 {
						break
					}
					merged = append(merged, parts[bestIdx].logs[parts[bestIdx].i])
					parts[bestIdx].i++
				}
				newest = merged
			}

			for i := len(newest) - 1; i >= 0; i-- {
				stepLogs.Logs = append(stepLogs.Logs, JobTreeLog{
					LineNumber: newest[i].LineNumber,
					Level:      newest[i].Level,
					Text:       newest[i].Message,
				})
			}
		}

		response.Steps = append(response.Steps, stepLogs)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// buildStepsFromStepJobs builds steps array from step jobs when step_definitions is not available
func (h *JobHandler) buildStepsFromStepJobs(ctx context.Context, stepJobs []*models.QueueJobState) []JobTreeStep {
	steps := make([]JobTreeStep, 0, len(stepJobs))

	for _, stepJob := range stepJobs {
		stepName := "unknown"
		if stepJob.Metadata != nil {
			if sn, ok := stepJob.Metadata["step_name"].(string); ok && sn != "" {
				stepName = sn
			}
		}

		step := JobTreeStep{
			StepID:     stepJob.ID, // Step job ID for "Show earlier logs" functionality
			Name:       stepName,
			Status:     string(stepJob.Status),
			StartedAt:  stepJob.StartedAt,
			FinishedAt: stepJob.FinishedAt,
			// Expanded is set AFTER logs are fetched (see below)
			ChildSummary: &ChildJobSummary{
				ErrorGroups: []ErrorGroup{},
			},
			Logs: []JobTreeLog{},
		}

		// Calculate duration
		if stepJob.StartedAt != nil {
			endTime := time.Now()
			if stepJob.FinishedAt != nil {
				endTime = *stepJob.FinishedAt
			}
			step.DurationMs = endTime.Sub(*stepJob.StartedAt).Milliseconds()
		}

		// Get grandchildren for ChildSummary
		grandchildOpts := &interfaces.JobListOptions{
			ParentID: stepJob.ID,
			Limit:    10000,
			Offset:   0,
		}
		grandchildren, err := h.jobStorage.ListJobs(ctx, grandchildOpts)
		if err == nil {
			for _, grandchild := range grandchildren {
				step.ChildSummary.Total++
				switch grandchild.Status {
				case models.JobStatusCompleted:
					step.ChildSummary.Completed++
				case models.JobStatusFailed:
					step.ChildSummary.Failed++
					if grandchild.Error != "" {
						h.addErrorToGroup(step.ChildSummary, grandchild.Error)
					}
				case models.JobStatusCancelled:
					step.ChildSummary.Cancelled++
				case models.JobStatusRunning:
					step.ChildSummary.Running++
				case models.JobStatusPending:
					step.ChildSummary.Pending++
				}
			}
		}

		// Get total log count for "Show earlier logs" indicator
		totalCount, countErr := h.logService.CountLogs(ctx, stepJob.ID)
		if countErr != nil {
			h.logger.Warn().Err(countErr).Str("step_job_id", stepJob.ID).Msg("Failed to count logs for step")
		} else {
			step.TotalLogs = totalCount
		}

		// Get the NEWEST 100 logs for this step job (newest first, then reverse for display)
		stepLogs, err := h.logService.GetLogsWithOffset(ctx, stepJob.ID, 100, 0)
		if err == nil {
			// Logs from GetLogsWithOffset are in DESC order (newest first)
			// Reverse to get ASC order for display (oldest of the newest 100 at top)
			for i := len(stepLogs) - 1; i >= 0; i-- {
				step.Logs = append(step.Logs, JobTreeLog{
					Level: stepLogs[i].Level,
					Text:  stepLogs[i].Message,
				})
			}
		}

		// Backend-driven expansion: expand if failed, running, or has logs
		// This moves expansion logic from frontend to backend for simpler UI
		hasLogs := len(step.Logs) > 0
		isRunning := stepJob.Status == models.JobStatusRunning
		isFailed := stepJob.Status == models.JobStatusFailed
		step.Expanded = isFailed || isRunning || hasLogs

		// Set child_summary to nil if no grandchildren
		if step.ChildSummary.Total == 0 {
			step.ChildSummary = nil
		}

		steps = append(steps, step)
	}

	return steps
}

// addErrorToGroup adds an error message to the appropriate group or creates a new one
func (h *JobHandler) addErrorToGroup(summary *ChildJobSummary, errorMsg string) {
	// Truncate long error messages for grouping
	truncated := errorMsg
	if len(truncated) > 100 {
		truncated = truncated[:100] + "..."
	}

	// Find existing group
	for i, group := range summary.ErrorGroups {
		if group.Message == truncated {
			summary.ErrorGroups[i].Count++
			return
		}
	}

	// Create new group
	summary.ErrorGroups = append(summary.ErrorGroups, ErrorGroup{
		Message: truncated,
		Count:   1,
	})
}

// GetJobStructureHandler returns a lightweight job structure for UI status updates
// GET /api/jobs/{id}/structure
//
// This is a simplified version of the tree endpoint that returns only:
// - Job status
// - Step statuses with log counts
// - No actual log content (UI fetches logs separately for expanded steps)
func (h *JobHandler) GetJobStructureHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract job ID from path: /api/jobs/{id}/structure
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

	// Get the parent job (manager job)
	jobInterface, err := h.jobManager.GetJob(ctx, jobID)
	if err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job for structure")
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	parentJob, ok := jobInterface.(*models.QueueJobState)
	if !ok {
		http.Error(w, "Invalid job type", http.StatusInternalServerError)
		return
	}

	// Get step_definitions from parent job metadata
	var stepDefinitions []map[string]interface{}
	if parentJob.Metadata != nil {
		if defs, ok := parentJob.Metadata["step_definitions"]; ok {
			switch v := defs.(type) {
			case []map[string]interface{}:
				stepDefinitions = v
			case []interface{}:
				for _, item := range v {
					if m, ok := item.(map[string]interface{}); ok {
						stepDefinitions = append(stepDefinitions, m)
					}
				}
			}
		}
	}

	// Get child jobs (step jobs) for this parent
	childOpts := &interfaces.JobListOptions{
		ParentID: jobID,
		Limit:    1000,
		Offset:   0,
	}
	stepJobs, err := h.jobStorage.ListJobs(ctx, childOpts)
	if err != nil {
		h.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to get step jobs for structure")
		stepJobs = []*models.QueueJobState{}
	}

	// Create a map of step_name -> step job for quick lookup
	stepJobMap := make(map[string]*models.QueueJobState)
	for _, stepJob := range stepJobs {
		if stepJob.Metadata != nil {
			if stepName, ok := stepJob.Metadata["step_name"].(string); ok && stepName != "" {
				stepJobMap[stepName] = stepJob
			}
		}
	}

	// Build minimal step status list
	steps := make([]StepStatus, 0, len(stepDefinitions))

	for _, stepDef := range stepDefinitions {
		stepName, _ := stepDef["name"].(string)
		if stepName == "" {
			stepName = "unknown"
		}

		step := StepStatus{
			Name:   stepName,
			Status: "pending",
		}

		// Find matching step job
		if stepJob, ok := stepJobMap[stepName]; ok {
			step.Status = string(stepJob.Status)

			// Get log count (not content)
			if logs, err := h.logService.GetLogs(ctx, stepJob.ID, 1000); err == nil {
				step.LogCount = len(logs)
			}

			// Get child count
			grandchildOpts := &interfaces.JobListOptions{
				ParentID: stepJob.ID,
				Limit:    1, // We only need count, not actual jobs
				Offset:   0,
			}
			if grandchildren, err := h.jobStorage.ListJobs(ctx, grandchildOpts); err == nil {
				// ListJobs returns actual jobs, not count - get count from stats if available
				if stats, err := h.jobManager.GetJobChildStats(ctx, []string{stepJob.ID}); err == nil {
					if stepStats, ok := stats[stepJob.ID]; ok {
						step.ChildCount = stepStats.ChildCount
					}
				} else {
					// Fallback to counting returned jobs
					step.ChildCount = len(grandchildren)
				}
			}
		}

		steps = append(steps, step)
	}

	// If no step_definitions, build from step jobs directly
	if len(stepDefinitions) == 0 && len(stepJobs) > 0 {
		for _, stepJob := range stepJobs {
			stepName := "unknown"
			if stepJob.Metadata != nil {
				if sn, ok := stepJob.Metadata["step_name"].(string); ok && sn != "" {
					stepName = sn
				}
			}

			step := StepStatus{
				Name:   stepName,
				Status: string(stepJob.Status),
			}

			// Get log count
			if logs, err := h.logService.GetLogs(ctx, stepJob.ID, 1000); err == nil {
				step.LogCount = len(logs)
			}

			steps = append(steps, step)
		}
	}

	response := JobStructureResponse{
		JobID:     jobID,
		Status:    string(parentJob.Status),
		Steps:     steps,
		UpdatedAt: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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
