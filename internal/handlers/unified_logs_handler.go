package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/logs"
	"github.com/ternarybob/quaero/internal/models"
)

// UnifiedLogsHandler handles the unified /api/logs endpoint
type UnifiedLogsHandler struct {
	logService interfaces.LogService
	logger     arbor.ILogger
}

// NewUnifiedLogsHandler creates a new unified logs handler
func NewUnifiedLogsHandler(logService interfaces.LogService, logger arbor.ILogger) *UnifiedLogsHandler {
	return &UnifiedLogsHandler{
		logService: logService,
		logger:     logger,
	}
}

// GetLogsHandler handles GET /api/logs with unified access to service and job logs
// Query parameters:
//   - scope: "service" (default) or "job"
//   - job_id: Required when scope=job
//   - include_children: Include child job logs (default: true, only for scope=job)
//   - level: Log level filter - debug, info, warn, error, all (default: info)
//   - limit: Max entries (default: 100)
//   - order: Sort order - asc (oldest first), desc (newest first) (default: desc)
//   - cursor: Pagination cursor (only for scope=job)
func (h *UnifiedLogsHandler) GetLogsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = "service"
	}

	switch scope {
	case "service":
		h.getServiceLogs(w, r)
	case "job":
		h.getJobLogs(w, r)
	default:
		http.Error(w, "Invalid scope. Valid values are: service, job", http.StatusBadRequest)
	}
}

// getServiceLogs returns recent service logs from the arbor memory writer
func (h *UnifiedLogsHandler) getServiceLogs(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
			if limit > 500 {
				limit = 500 // Cap service logs at 500
			}
		}
	}

	level := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("level")))
	if level == "" {
		level = "info"
	}

	order := r.URL.Query().Get("order")
	if order == "" {
		order = "desc"
	}

	// Get recent logs from memory writer
	memWriter := arbor.GetRegisteredMemoryWriter(arbor.WRITER_MEMORY)
	var logs []interfaces.LogEntry

	if memWriter != nil {
		entries, err := memWriter.GetEntriesWithLimit(limit)
		if err != nil {
			h.logger.Error().Err(err).Msg("Failed to get log entries")
			http.Error(w, "Failed to retrieve logs", http.StatusInternalServerError)
			return
		}

		// Extract and sort keys for deterministic ordering
		keys := make([]string, 0, len(entries))
		for key := range entries {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		// Parse and filter logs in sorted order (oldest first)
		for _, key := range keys {
			logLine := entries[key]
			// Skip internal handler logs
			if strings.Contains(logLine, "WebSocket client connected") ||
				strings.Contains(logLine, "WebSocket client disconnected") ||
				strings.Contains(logLine, "DEBUG: Memory writer entry") ||
				strings.Contains(logLine, "HTTP request") ||
				strings.Contains(logLine, "HTTP response") ||
				strings.Contains(logLine, "Publishing Event") {
				continue
			}

			// Parse log line (format: LEVEL | DATE TIME | MESSAGE)
			parts := strings.SplitN(logLine, "|", 3)
			if len(parts) != 3 {
				continue
			}

			levelStr := strings.TrimSpace(parts[0])
			dateTime := strings.TrimSpace(parts[1])
			messageWithFields := strings.TrimSpace(parts[2])

			// Parse timestamp from "Oct  2 16:27:13" format and add .000 for alignment with live logs
			timeParts := strings.Fields(dateTime)
			var timestamp string
			if len(timeParts) >= 3 {
				timestamp = timeParts[len(timeParts)-1] + ".000"
			} else {
				timestamp = time.Now().Format("15:04:05.000")
			}

			// Map level to 3-letter format for consistency
			logLevel := "INF" // Default
			switch levelStr {
			case "ERR", "ERROR", "FATAL", "PANIC":
				logLevel = "ERR"
			case "WRN", "WARN":
				logLevel = "WRN"
			case "INF", "INFO":
				logLevel = "INF"
			case "DBG", "DEBUG":
				logLevel = "DBG"
			}

			// Apply level filter
			if !h.shouldIncludeLevel(logLevel, level) {
				continue
			}

			entry := interfaces.LogEntry{
				Index:     len(logs),
				Timestamp: timestamp,
				Level:     logLevel,
				Message:   messageWithFields,
			}

			logs = append(logs, entry)
		}
	}

	// Return empty array if no logs
	if logs == nil {
		logs = []interfaces.LogEntry{}
	}

	// Sort by index (already in chronological order from key sorting)
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Index < logs[j].Index
	})

	// Apply ordering if desc requested
	if order == "desc" {
		for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
			logs[i], logs[j] = logs[j], logs[i]
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"scope": "service",
		"logs":  logs,
		"count": len(logs),
		"limit": limit,
		"order": order,
		"level": level,
	})
}

// getJobLogs returns logs for a specific job and optionally its children
func (h *UnifiedLogsHandler) getJobLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// job_id is required for scope=job
	jobID := r.URL.Query().Get("job_id")
	if jobID == "" {
		http.Error(w, "job_id is required when scope=job", http.StatusBadRequest)
		return
	}

	// Parse query parameters
	level := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("level")))
	if level == "" {
		level = "info"
	}

	// Normalize level aliases
	levelAliases := map[string]string{
		"warning": "warn",
		"err":     "error",
	}
	if normalized, exists := levelAliases[level]; exists {
		level = normalized
	}

	// Validate level
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

	// Parse limit/size (size is alias for limit per user request)
	limit := 100
	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		limitStr = r.URL.Query().Get("size") // Support 'size' as alias
	}
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
			if limit > 5000 {
				limit = 5000
			}
		}
	}

	includeChildren := true
	if includeChildrenStr := r.URL.Query().Get("include_children"); includeChildrenStr != "" {
		includeChildren = includeChildrenStr == "true"
	}

	order := r.URL.Query().Get("order")
	if order == "" {
		order = "desc"
	}

	// Step filter - when provided, returns step-grouped results for step log display
	stepFilter := r.URL.Query().Get("step")
	if stepFilter != "" {
		h.getStepGroupedLogs(w, r, jobID, stepFilter, level, limit, order, includeChildren)
		return
	}

	// Fast path: direct job log retrieval when not including children
	// This avoids the expensive GetAggregatedLogs call with descendant traversal
	if !includeChildren {
		type levelSlice struct {
			logs []models.LogEntry
			i    int
		}

		getIncludedLevels := func(filter string) []string {
			// Match shouldIncludeLevel semantics used for service logs:
			// - info: INF + WRN + ERR (exclude debug)
			// - warn: WRN + ERR
			// - error: ERR
			// - debug/all: include all levels
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

		// Fetch newest logs for this filter (DESC order, newest first).
		var logEntries []models.LogEntry
		var err error
		if level == "all" || level == "debug" {
			logEntries, err = h.logService.GetLogs(ctx, jobID, limit)
		} else if level == "error" {
			logEntries, err = h.logService.GetLogsByLevel(ctx, jobID, "error", limit)
		} else {
			included := getIncludedLevels(level)
			parts := make([]levelSlice, 0, len(included))
			for _, lv := range included {
				part, partErr := h.logService.GetLogsByLevel(ctx, jobID, lv, limit)
				if partErr != nil {
					continue
				}
				parts = append(parts, levelSlice{logs: part, i: 0})
			}

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
			logEntries = merged
		}

		if err != nil {
			h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get direct job logs")
			http.Error(w, "Failed to get job logs", http.StatusInternalServerError)
			return
		}

		// Get total count for pagination/scrolling indicator (must match filters)
		totalCount := 0
		var countErr error
		if level == "all" || level == "debug" {
			totalCount, countErr = h.logService.CountLogs(ctx, jobID)
		} else {
			for _, lv := range getIncludedLevels(level) {
				n, err := h.logService.CountLogsByLevel(ctx, jobID, lv)
				if err != nil {
					countErr = err
					break
				}
				totalCount += n
			}
		}
		if countErr != nil {
			h.logger.Warn().Err(countErr).Str("job_id", jobID).Str("level", level).Msg("Failed to count logs, using returned count")
			totalCount = len(logEntries)
		}

		// Build response logs
		responseLogs := make([]map[string]interface{}, 0, len(logEntries))
		for _, log := range logEntries {
			responseLog := map[string]interface{}{
				"timestamp":      log.Timestamp,
				"full_timestamp": log.FullTimestamp,
				"level":          log.Level,
				"message":        log.Message,
				"job_id":         log.JobID(),
				"step_name":      log.StepName(),
				"source_type":    log.SourceType(),
				"originator":     log.Originator(),
				"phase":          log.Phase(),
				"line_number":    log.LineNumber,
			}
			responseLogs = append(responseLogs, responseLog)
		}

		// Storage always returns DESC order (newest first)
		// Reverse if ASC order requested
		if order == "asc" {
			for i, j := 0, len(responseLogs)-1; i < j; i, j = i+1, j-1 {
				responseLogs[i], responseLogs[j] = responseLogs[j], responseLogs[i]
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"scope":            "job",
			"job_id":           jobID,
			"logs":             responseLogs,
			"count":            len(responseLogs),
			"total_count":      totalCount,
			"limit":            limit,
			"order":            order,
			"level":            level,
			"include_children": false,
		})
		return
	}

	cursor := r.URL.Query().Get("cursor")

	// Fetch aggregated logs (includes children)
	logEntries, metadata, nextCursor, err := h.logService.GetAggregatedLogs(ctx, jobID, includeChildren, level, limit, cursor, order)
	if err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job logs")
		if errors.Is(err, logs.ErrJobNotFound) {
			http.Error(w, "Job not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to get job logs", http.StatusInternalServerError)
		}
		return
	}

	// Enrich logs with job context from metadata
	enrichedLogs := make([]map[string]interface{}, 0, len(logEntries))
	for _, log := range logEntries {
		enrichedLog := map[string]interface{}{
			"timestamp":      log.Timestamp,
			"full_timestamp": log.FullTimestamp,
			"level":          log.Level,
			"message":        log.Message,
			"job_id":         log.JobID(),
			"step_name":      log.StepName(),
			"source_type":    log.SourceType(),
			"originator":     log.Originator(),
			"line_number":    log.LineNumber,
		}

		if meta, exists := metadata[log.JobID()]; exists {
			enrichedLog["job_name"] = meta.JobName
			enrichedLog["job_url"] = meta.JobURL
			enrichedLog["job_depth"] = meta.JobDepth
			enrichedLog["job_type"] = meta.JobType
			enrichedLog["parent_id"] = meta.ParentID
		} else {
			enrichedLog["job_name"] = fmt.Sprintf("Job %s", log.JobID())
			enrichedLog["job_type"] = "unknown"
			enrichedLog["parent_id"] = ""
		}

		enrichedLogs = append(enrichedLogs, enrichedLog)
	}

	// NOTE: GetAggregatedLogs already returns logs in the requested order
	// No additional reversal needed - the k-way merge respects the 'order' parameter

	// Get total count for pagination/display
	totalCount, err := h.logService.CountAggregatedLogs(ctx, jobID, includeChildren, level)
	if err != nil {
		h.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to count aggregated logs, using returned count")
		totalCount = len(enrichedLogs)
	}
	h.logger.Debug().Str("job_id", jobID).Bool("include_children", includeChildren).Int("total_count", totalCount).Int("enriched_count", len(enrichedLogs)).Msg("Aggregated logs total count")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"scope":            "job",
		"job_id":           jobID,
		"logs":             enrichedLogs,
		"count":            len(enrichedLogs),
		"total_count":      totalCount,
		"limit":            limit,
		"order":            order,
		"level":            level,
		"include_children": includeChildren,
		"metadata":         metadata,
		"next_cursor":      nextCursor,
	})
}

// shouldIncludeLevel checks if a log entry should be included based on level filter
func (h *UnifiedLogsHandler) shouldIncludeLevel(logLevel, filterLevel string) bool {
	if filterLevel == "all" {
		return true
	}

	// Level hierarchy: ERR > WRN > INF > DBG
	levelPriority := map[string]int{
		"ERR": 4,
		"WRN": 3,
		"INF": 2,
		"DBG": 1,
	}

	filterPriority := map[string]int{
		"error": 4,
		"warn":  3,
		"info":  2,
		"debug": 1,
	}

	logPrio, logOk := levelPriority[logLevel]
	filterPrio, filterOk := filterPriority[filterLevel]

	if !logOk || !filterOk {
		return true // Include unknown levels
	}

	// Include if log level >= filter level
	return logPrio >= filterPrio
}

// StepLog represents a log entry in step-grouped response
type StepLog struct {
	LineNumber int    `json:"line_number"`
	Level      string `json:"level"`
	Message    string `json:"message"`
}

// StepLogsResponse represents step-grouped logs response
type StepLogsResponse struct {
	StepName        string    `json:"step_name"`
	StepID          string    `json:"step_id,omitempty"`
	Status          string    `json:"status"`
	Logs            []StepLog `json:"logs"`
	TotalCount      int       `json:"total_count"`      // Total logs matching level filter
	UnfilteredCount int       `json:"unfiltered_count"` // Total logs regardless of filter
}

// getStepGroupedLogs returns logs grouped by step for step-level log retrieval
// Response format: { "job_id": "...", "steps": [{ "step_name": "...", "logs": [...], "total_count": N, "unfiltered_count": N }] }
// includeChildren: when true, includes logs from child/worker jobs (slower but complete)
func (h *UnifiedLogsHandler) getStepGroupedLogs(w http.ResponseWriter, r *http.Request, jobID, stepFilter, level string, limit int, order string, includeChildren bool) {
	ctx := r.Context()

	// Get logs for this step job, optionally including children/descendants
	// Note: includeChildren=true can be slow for steps with many worker jobs (k-way merge)
	// The caller controls this via query parameter to balance completeness vs performance
	logEntries, _, _, err := h.logService.GetAggregatedLogs(ctx, jobID, includeChildren, level, limit, "", order)
	if err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get step logs")
		if errors.Is(err, logs.ErrJobNotFound) {
			http.Error(w, "Job not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to get job logs", http.StatusInternalServerError)
		}
		return
	}

	// Count total logs matching filter (includes children if requested)
	totalCount, countErr := h.logService.CountAggregatedLogs(ctx, jobID, includeChildren, level)
	if countErr != nil {
		h.logger.Warn().Err(countErr).Str("job_id", jobID).Msg("Failed to count logs")
		totalCount = len(logEntries)
	}

	// Count unfiltered logs (all levels, same includeChildren setting)
	unfilteredCount, unfilteredErr := h.logService.CountAggregatedLogs(ctx, jobID, includeChildren, "all")
	if unfilteredErr != nil {
		h.logger.Warn().Err(unfilteredErr).Str("job_id", jobID).Msg("Failed to count unfiltered logs")
		unfilteredCount = totalCount
	}

	// Convert to step logs format
	// Logs from GetAggregatedLogs are already in the requested order
	// For ASC display, we want oldest first (line 1, 2, 3...)
	// If order=desc was requested, we got newest first - reverse for ASC display
	stepLogs := make([]StepLog, 0, len(logEntries))
	if order == "desc" {
		// Reverse to get ASC order for display (line numbers should be ascending)
		for i := len(logEntries) - 1; i >= 0; i-- {
			stepLogs = append(stepLogs, StepLog{
				LineNumber: logEntries[i].LineNumber,
				Level:      logEntries[i].Level,
				Message:    logEntries[i].Message,
			})
		}
	} else {
		// Already in ASC order
		for _, log := range logEntries {
			stepLogs = append(stepLogs, StepLog{
				LineNumber: log.LineNumber,
				Level:      log.Level,
				Message:    log.Message,
			})
		}
	}

	// Build response with step-grouped logs
	response := struct {
		JobID string             `json:"job_id"`
		Steps []StepLogsResponse `json:"steps"`
	}{
		JobID: jobID,
		Steps: []StepLogsResponse{
			{
				StepName:        stepFilter,
				StepID:          jobID,
				Status:          "completed", // TODO: Get actual status from job storage if needed
				Logs:            stepLogs,
				TotalCount:      totalCount,
				UnfilteredCount: unfilteredCount,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
