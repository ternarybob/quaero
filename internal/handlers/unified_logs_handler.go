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

			// Parse timestamp from "Oct  2 16:27:13" format
			timeParts := strings.Fields(dateTime)
			var timestamp string
			if len(timeParts) >= 3 {
				timestamp = timeParts[len(timeParts)-1]
			} else {
				timestamp = time.Now().Format("15:04:05")
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

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
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

	cursor := r.URL.Query().Get("cursor")

	// Fetch aggregated logs
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
			"job_id":         log.AssociatedJobID,
			"step_name":      log.StepName,
			"source_type":    log.SourceType,
			"originator":     log.Originator,
		}

		if meta, exists := metadata[log.AssociatedJobID]; exists {
			enrichedLog["job_name"] = meta.JobName
			enrichedLog["job_url"] = meta.JobURL
			enrichedLog["job_depth"] = meta.JobDepth
			enrichedLog["job_type"] = meta.JobType
			enrichedLog["parent_id"] = meta.ParentID
		} else {
			enrichedLog["job_name"] = fmt.Sprintf("Job %s", log.AssociatedJobID)
			enrichedLog["job_type"] = "unknown"
			enrichedLog["parent_id"] = ""
		}

		enrichedLogs = append(enrichedLogs, enrichedLog)
	}

	// Apply ordering
	if order == "desc" {
		for i, j := 0, len(enrichedLogs)-1; i < j; i, j = i+1, j-1 {
			enrichedLogs[i], enrichedLogs[j] = enrichedLogs[j], enrichedLogs[i]
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"scope":            "job",
		"job_id":           jobID,
		"logs":             enrichedLogs,
		"count":            len(enrichedLogs),
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
