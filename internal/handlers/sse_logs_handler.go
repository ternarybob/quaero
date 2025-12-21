package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// SSELogsHandler handles Server-Sent Events for real-time log streaming
type SSELogsHandler struct {
	logService   interfaces.LogService
	eventService interfaces.EventService
	logger       arbor.ILogger

	// Service log subscribers
	serviceSubsMu sync.RWMutex
	serviceSubs   map[*serviceLogSubscriber]struct{}

	// Job log subscribers (keyed by job ID)
	jobSubsMu sync.RWMutex
	jobSubs   map[string]map[*jobLogSubscriber]struct{}
}

// serviceLogSubscriber represents an SSE client subscribed to service logs
type serviceLogSubscriber struct {
	logs   chan interfaces.LogEntry
	done   chan struct{}
	level  string
	limit  int
	ctx    context.Context
	cancel context.CancelFunc
}

// jobLogSubscriber represents an SSE client subscribed to job logs
type jobLogSubscriber struct {
	logs   chan jobLogEntry
	status chan jobStatusUpdate
	done   chan struct{}
	jobID  string
	stepID string
	level  string
	limit  int
	ctx    context.Context
	cancel context.CancelFunc
}

// jobLogEntry is a log entry for job logs
type jobLogEntry struct {
	ID            string    `json:"id,omitempty"`
	Timestamp     string    `json:"timestamp"`
	FullTimestamp time.Time `json:"full_timestamp,omitempty"`
	Level         string    `json:"level"`
	Message       string    `json:"message"`
	JobID         string    `json:"job_id,omitempty"`
	StepName      string    `json:"step_name,omitempty"`
	StepID        string    `json:"step_id,omitempty"`
	LineNumber    int       `json:"line_number,omitempty"`
}

// jobStatusUpdate is a status update for job/step
type jobStatusUpdate struct {
	Job   *jobStatus   `json:"job,omitempty"`
	Steps []stepStatus `json:"steps,omitempty"`
}

type jobStatus struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Progress int    `json:"progress,omitempty"`
}

type stepStatus struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// SSE event payloads
type sseLogBatch struct {
	Logs []interface{} `json:"logs"`
	Meta sseMeta       `json:"meta"`
}

type sseMeta struct {
	TotalCount     int    `json:"total_count"`
	DisplayedCount int    `json:"displayed_count"`
	HasMore        bool   `json:"has_more"`
	OldestID       string `json:"oldest_id,omitempty"`
	NewestID       string `json:"newest_id,omitempty"`
}

type ssePing struct {
	Timestamp time.Time `json:"timestamp"`
}

// NewSSELogsHandler creates a new SSE logs handler
func NewSSELogsHandler(logService interfaces.LogService, eventService interfaces.EventService, logger arbor.ILogger) *SSELogsHandler {
	h := &SSELogsHandler{
		logService:   logService,
		eventService: eventService,
		logger:       logger,
		serviceSubs:  make(map[*serviceLogSubscriber]struct{}),
		jobSubs:      make(map[string]map[*jobLogSubscriber]struct{}),
	}

	// Subscribe to service log events
	eventService.Subscribe("log_event", h.handleServiceLogEvent)

	// Subscribe to job log events
	eventService.Subscribe(interfaces.EventJobLog, h.handleJobLogEvent)

	// Subscribe to job status changes
	eventService.Subscribe(interfaces.EventJobStatusChange, h.handleJobStatusEvent)

	return h
}

// handleServiceLogEvent handles incoming service log events
// Also routes job logs (identified by job_id in payload) to job subscribers
func (h *SSELogsHandler) handleServiceLogEvent(ctx context.Context, event interfaces.Event) error {
	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		return nil
	}

	// Check if this is a job log (has job_id) - route to job subscribers
	if jobID, ok := payload["job_id"].(string); ok && jobID != "" {
		return h.routeJobLogFromLogEvent(payload, jobID)
	}

	// Parse log entry from payload (service log - no job_id)
	entry := interfaces.LogEntry{
		Timestamp: fmt.Sprintf("%v", payload["timestamp"]),
		Level:     fmt.Sprintf("%v", payload["level"]),
		Message:   fmt.Sprintf("%v", payload["message"]),
	}

	// Broadcast to all service log subscribers
	h.serviceSubsMu.RLock()
	defer h.serviceSubsMu.RUnlock()

	for sub := range h.serviceSubs {
		// Apply level filter
		if !h.shouldIncludeLevel(entry.Level, sub.level) {
			continue
		}

		select {
		case sub.logs <- entry:
		default:
			// Buffer full, skip this entry
		}
	}

	return nil
}

// routeJobLogFromLogEvent routes a job log (from log_event with job_id) to job subscribers
// This handles logs from arbor logger (workers, etc.) that publish via log_event
func (h *SSELogsHandler) routeJobLogFromLogEvent(payload map[string]interface{}, jobID string) error {
	// Extract additional fields that may be present
	managerID, _ := payload["manager_id"].(string)
	parentID, _ := payload["parent_id"].(string)
	stepID, _ := payload["step_id"].(string)
	stepName, _ := payload["step_name"].(string)

	// Parse line_number if present
	lineNumber := 0
	if ln, ok := payload["line_number"].(float64); ok {
		lineNumber = int(ln)
	}

	entry := jobLogEntry{
		Timestamp:  fmt.Sprintf("%v", payload["timestamp"]),
		Level:      fmt.Sprintf("%v", payload["level"]),
		Message:    fmt.Sprintf("%v", payload["message"]),
		JobID:      jobID,
		StepName:   stepName,
		StepID:     stepID,
		LineNumber: lineNumber,
	}

	// Broadcast to subscribers
	// Match subscribers for: exact job ID, parent ID, or manager ID
	h.jobSubsMu.RLock()
	defer h.jobSubsMu.RUnlock()

	// Collect all matching subscriber job IDs
	matchingJobIDs := []string{jobID}
	if managerID != "" && managerID != jobID {
		matchingJobIDs = append(matchingJobIDs, managerID)
	}
	if parentID != "" && parentID != jobID && parentID != managerID {
		matchingJobIDs = append(matchingJobIDs, parentID)
	}

	for _, matchJobID := range matchingJobIDs {
		subs := h.jobSubs[matchJobID]
		for sub := range subs {
			// Apply level filter
			if !h.shouldIncludeLevel(entry.Level, sub.level) {
				continue
			}

			// Apply step filter - only filter if subscriber requested specific step
			if sub.stepID != "" && entry.StepID != sub.stepID {
				continue
			}

			select {
			case sub.logs <- entry:
			default:
				// Buffer full, skip this entry
			}
		}
	}

	return nil
}

// handleJobLogEvent handles incoming job log events
func (h *SSELogsHandler) handleJobLogEvent(ctx context.Context, event interfaces.Event) error {
	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		return nil
	}

	jobID, _ := payload["job_id"].(string)
	if jobID == "" {
		return nil
	}

	// Extract parent/manager IDs for matching subscribers
	managerID, _ := payload["manager_id"].(string)
	parentID, _ := payload["parent_id"].(string)
	stepID, _ := payload["step_id"].(string)
	stepName, _ := payload["step_name"].(string)

	// Debug: log event reception
	h.logger.Debug().
		Str("job_id", jobID).
		Str("manager_id", managerID).
		Str("step_name", stepName).
		Msg("[SSE DEBUG] Received EventJobLog")

	// Parse log entry from payload - handle optional fields safely
	// Note: line_number can be int (from direct Go map) or float64 (from JSON decode)
	lineNumber := 0
	switch ln := payload["line_number"].(type) {
	case int:
		lineNumber = ln
	case float64:
		lineNumber = int(ln)
	}

	// Extract string fields with proper nil handling
	timestamp, _ := payload["timestamp"].(string)
	level, _ := payload["level"].(string)
	message, _ := payload["message"].(string)

	entry := jobLogEntry{
		Timestamp:  timestamp,
		Level:      level,
		Message:    message,
		JobID:      jobID,
		StepName:   stepName,
		StepID:     stepID,
		LineNumber: lineNumber,
	}

	// Broadcast to subscribers
	// Match subscribers for: exact job ID, parent ID, or manager ID
	h.jobSubsMu.RLock()
	defer h.jobSubsMu.RUnlock()

	// Collect all matching subscriber job IDs
	matchingJobIDs := []string{jobID}
	if managerID != "" && managerID != jobID {
		matchingJobIDs = append(matchingJobIDs, managerID)
	}
	if parentID != "" && parentID != jobID && parentID != managerID {
		matchingJobIDs = append(matchingJobIDs, parentID)
	}

	// Debug: log subscriber lookup
	totalSubs := 0
	routedCount := 0
	for _, matchJobID := range matchingJobIDs {
		subs := h.jobSubs[matchJobID]
		totalSubs += len(subs)
		for sub := range subs {
			// Apply level filter
			if !h.shouldIncludeLevel(entry.Level, sub.level) {
				continue
			}

			// Apply step filter - only filter if subscriber requested specific step
			if sub.stepID != "" && entry.StepID != sub.stepID {
				continue
			}

			select {
			case sub.logs <- entry:
				routedCount++
			default:
				// Buffer full, skip this entry
				h.logger.Warn().Str("job_id", matchJobID).Msg("[SSE DEBUG] Buffer full, skipping entry")
			}
		}
	}

	if totalSubs == 0 {
		h.logger.Debug().
			Str("job_id", jobID).
			Str("manager_id", managerID).
			Strs("matching_ids", matchingJobIDs).
			Msg("[SSE DEBUG] No subscribers found for job log")
	} else {
		h.logger.Debug().
			Str("job_id", jobID).
			Str("manager_id", managerID).
			Int("total_subs", totalSubs).
			Int("routed", routedCount).
			Msg("[SSE DEBUG] Routed job log to subscribers")
	}

	return nil
}

// handleJobStatusEvent handles job status change events
func (h *SSELogsHandler) handleJobStatusEvent(ctx context.Context, event interfaces.Event) error {
	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		return nil
	}

	jobID, _ := payload["job_id"].(string)
	if jobID == "" {
		return nil
	}

	status, _ := payload["status"].(string)

	update := jobStatusUpdate{
		Job: &jobStatus{
			ID:     jobID,
			Status: status,
		},
	}

	// Broadcast to subscribers for this job
	h.jobSubsMu.RLock()
	defer h.jobSubsMu.RUnlock()

	subs := h.jobSubs[jobID]
	for sub := range subs {
		select {
		case sub.status <- update:
		default:
			// Buffer full, skip
		}
	}

	return nil
}

// StreamLogs handles GET /api/logs/stream - unified SSE endpoint for all logs
// Query parameters:
//   - scope: "service" (default) or "job"
//   - job_id: Required when scope=job
//   - step: Step name filter (only for scope=job)
//   - level: Log level filter - debug, info, warn, error (default: info)
//   - limit: Initial log limit (default: 100)
func (h *SSELogsHandler) StreamLogs(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = "service"
	}

	switch scope {
	case "service":
		h.streamServiceLogs(w, r)
	case "job":
		h.streamJobLogs(w, r)
	default:
		http.Error(w, "Invalid scope. Valid values are: service, job", http.StatusBadRequest)
	}
}

// streamServiceLogs handles SSE streaming for service logs
func (h *SSELogsHandler) streamServiceLogs(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
			if limit > 500 {
				limit = 500
			}
		}
	}

	level := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("level")))
	if level == "" {
		level = "info"
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Flush headers immediately to trigger browser's EventSource.onopen
	flusher.Flush()

	// Create subscriber
	// Buffer size of 10000 handles high-throughput logging (e.g., codebase_classify job with 150+ files)
	// without dropping entries during burst periods
	ctx, cancel := context.WithCancel(r.Context())
	sub := &serviceLogSubscriber{
		logs:   make(chan interfaces.LogEntry, 10000),
		done:   make(chan struct{}),
		level:  level,
		limit:  limit,
		ctx:    ctx,
		cancel: cancel,
	}

	// Register subscriber
	h.serviceSubsMu.Lock()
	h.serviceSubs[sub] = struct{}{}
	h.serviceSubsMu.Unlock()

	defer func() {
		h.serviceSubsMu.Lock()
		delete(h.serviceSubs, sub)
		h.serviceSubsMu.Unlock()
		cancel()
	}()

	// Send initial logs
	h.sendInitialServiceLogs(w, flusher, level, limit)

	// Adaptive backoff rate limiting for high-throughput service log streams
	// Same strategy as job logs
	backoffLevels := []time.Duration{
		500 * time.Millisecond,
		1 * time.Second,
		2 * time.Second,
		3 * time.Second,
		5 * time.Second,
	}
	currentBackoffLevel := 0
	currentInterval := backoffLevels[0]
	const logsPerIntervalThreshold = 200

	pingInterval := 15 * time.Second

	batchTicker := time.NewTicker(currentInterval)
	pingTicker := time.NewTicker(pingInterval)
	defer batchTicker.Stop()
	defer pingTicker.Stop()

	var logBatch []interfaces.LogEntry
	var logsReceivedThisInterval int

	for {
		select {
		case <-r.Context().Done():
			return

		case log := <-sub.logs:
			logBatch = append(logBatch, log)
			logsReceivedThisInterval++

		case <-batchTicker.C:
			// Adaptive backoff: adjust interval based on log throughput
			if logsReceivedThisInterval > logsPerIntervalThreshold {
				if currentBackoffLevel < len(backoffLevels)-1 {
					currentBackoffLevel++
					currentInterval = backoffLevels[currentBackoffLevel]
					batchTicker.Reset(currentInterval)
				}
			} else if logsReceivedThisInterval < logsPerIntervalThreshold/2 {
				if currentBackoffLevel > 0 {
					currentBackoffLevel--
					currentInterval = backoffLevels[currentBackoffLevel]
					batchTicker.Reset(currentInterval)
				}
			}

			if len(logBatch) > 0 {
				h.sendServiceLogBatch(w, flusher, logBatch)
				logBatch = logBatch[:0]
				pingTicker.Reset(pingInterval)
			}

			logsReceivedThisInterval = 0

		case <-pingTicker.C:
			h.sendPing(w, flusher)
		}
	}
}

// streamJobLogs handles SSE streaming for job/step logs
// Called from StreamLogs with scope=job or from path-based route
func (h *SSELogsHandler) streamJobLogs(w http.ResponseWriter, r *http.Request) {
	// Get job ID from query param (unified endpoint) or path (legacy endpoint)
	jobID := r.URL.Query().Get("job_id")
	if jobID == "" {
		// Try extracting from path: /api/jobs/{id}/logs/stream
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/jobs/") {
			pathSuffix := path[len("/api/jobs/"):]
			if idx := strings.Index(pathSuffix, "/"); idx > 0 {
				jobID = pathSuffix[:idx]
			}
		}
	}
	if jobID == "" {
		http.Error(w, "job_id is required when scope=job", http.StatusBadRequest)
		return
	}

	// Parse query parameters
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
			if limit > 5000 {
				limit = 5000
			}
		}
	}

	level := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("level")))
	if level == "" {
		level = "info"
	}

	stepID := r.URL.Query().Get("step")

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Flush headers immediately to trigger browser's EventSource.onopen
	flusher.Flush()

	// Create subscriber
	// Buffer size of 10000 handles high-throughput parallel jobs (e.g., codebase_classify with 150+ files)
	// without dropping logs during burst periods. Increased from 2000 after observing 719 buffer overflows.
	ctx, cancel := context.WithCancel(r.Context())
	sub := &jobLogSubscriber{
		logs:   make(chan jobLogEntry, 10000),
		status: make(chan jobStatusUpdate, 10),
		done:   make(chan struct{}),
		jobID:  jobID,
		stepID: stepID,
		level:  level,
		limit:  limit,
		ctx:    ctx,
		cancel: cancel,
	}

	// Register subscriber
	h.jobSubsMu.Lock()
	if h.jobSubs[jobID] == nil {
		h.jobSubs[jobID] = make(map[*jobLogSubscriber]struct{})
	}
	h.jobSubs[jobID][sub] = struct{}{}
	subCount := len(h.jobSubs[jobID])
	h.jobSubsMu.Unlock()

	h.logger.Info().
		Str("job_id", jobID).
		Str("step_id", stepID).
		Str("level", level).
		Int("subscriber_count", subCount).
		Msg("[SSE DEBUG] Job log subscriber registered")

	defer func() {
		h.jobSubsMu.Lock()
		delete(h.jobSubs[jobID], sub)
		remaining := len(h.jobSubs[jobID])
		if remaining == 0 {
			delete(h.jobSubs, jobID)
		}
		h.jobSubsMu.Unlock()
		cancel()
		h.logger.Info().
			Str("job_id", jobID).
			Int("remaining_subscribers", remaining).
			Msg("[SSE DEBUG] Job log subscriber unregistered")
	}()

	// Send initial logs and status
	h.sendInitialJobLogs(w, flusher, jobID, stepID, level, limit)

	// Adaptive backoff rate limiting for high-throughput log streams
	// Prevents browser overload while ensuring logs are delivered
	//
	// Backoff strategy:
	// - Base interval: 500ms for faster initial delivery
	// - When logs exceed threshold (200/interval), increase interval
	// - Backoff levels: 500ms → 1s → 2s → 3s → 5s (max)
	// - Reset to base when log rate drops below threshold
	// - Always flush immediately on status change (job completion)
	backoffLevels := []time.Duration{
		500 * time.Millisecond,
		1 * time.Second,
		2 * time.Second,
		3 * time.Second,
		5 * time.Second,
	}
	currentBackoffLevel := 0
	currentInterval := backoffLevels[0]
	const logsPerIntervalThreshold = 200 // Higher threshold for parallel jobs (300+ workers)

	pingInterval := 15 * time.Second

	batchTicker := time.NewTicker(currentInterval)
	pingTicker := time.NewTicker(pingInterval)
	defer batchTicker.Stop()
	defer pingTicker.Stop()

	var logBatch []jobLogEntry
	var logsReceivedThisInterval int

	for {
		select {
		case <-r.Context().Done():
			return

		case log := <-sub.logs:
			// Always accept logs into the batch (no dropping)
			// We'll control the push rate via backoff instead
			logBatch = append(logBatch, log)
			logsReceivedThisInterval++

		case status := <-sub.status:
			// Status change: flush logs immediately, then send status
			// This ensures logs are delivered on job completion
			if len(logBatch) > 0 {
				h.sendJobLogBatch(w, flusher, logBatch, jobID, stepID, level)
				logBatch = logBatch[:0]
			}
			h.sendStatus(w, flusher, status)
			pingTicker.Reset(pingInterval)

			// Reset backoff on status change (job likely completed or progressed)
			currentBackoffLevel = 0
			currentInterval = backoffLevels[0]
			batchTicker.Reset(currentInterval)
			logsReceivedThisInterval = 0

		case <-batchTicker.C:
			// Adaptive backoff: adjust interval based on log throughput
			if logsReceivedThisInterval > logsPerIntervalThreshold {
				// High throughput: increase backoff
				if currentBackoffLevel < len(backoffLevels)-1 {
					currentBackoffLevel++
					currentInterval = backoffLevels[currentBackoffLevel]
					batchTicker.Reset(currentInterval)
					h.logger.Debug().
						Int("logs_received", logsReceivedThisInterval).
						Dur("new_interval", currentInterval).
						Msg("[SSE] Increasing backoff due to high throughput")
				}
			} else if logsReceivedThisInterval < logsPerIntervalThreshold/2 {
				// Low throughput: decrease backoff (recover faster)
				if currentBackoffLevel > 0 {
					currentBackoffLevel--
					currentInterval = backoffLevels[currentBackoffLevel]
					batchTicker.Reset(currentInterval)
				}
			}

			// Send accumulated logs
			if len(logBatch) > 0 {
				h.sendJobLogBatch(w, flusher, logBatch, jobID, stepID, level)
				logBatch = logBatch[:0]
				pingTicker.Reset(pingInterval)
			}

			logsReceivedThisInterval = 0

		case <-pingTicker.C:
			h.sendPing(w, flusher)
		}
	}
}

// sendInitialServiceLogs sends initial service logs on connection
func (h *SSELogsHandler) sendInitialServiceLogs(w http.ResponseWriter, flusher http.Flusher, level string, limit int) {
	// Get recent logs from memory writer
	memWriter := arbor.GetRegisteredMemoryWriter(arbor.WRITER_MEMORY)
	var logs []interface{}

	if memWriter != nil {
		entries, err := memWriter.GetEntriesWithLimit(limit)
		if err != nil {
			h.logger.Error().Err(err).Msg("Failed to get initial service logs")
			return
		}

		// Extract and sort keys for deterministic ordering
		keys := make([]string, 0, len(entries))
		for key := range entries {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		// Parse and filter logs
		for _, key := range keys {
			logLine := entries[key]
			// Skip internal handler logs
			if strings.Contains(logLine, "WebSocket client connected") ||
				strings.Contains(logLine, "WebSocket client disconnected") ||
				strings.Contains(logLine, "HTTP request") ||
				strings.Contains(logLine, "HTTP response") {
				continue
			}

			// Parse log line
			parts := strings.SplitN(logLine, "|", 3)
			if len(parts) != 3 {
				continue
			}

			levelStr := strings.TrimSpace(parts[0])
			dateTime := strings.TrimSpace(parts[1])
			message := strings.TrimSpace(parts[2])

			// Map level to 3-letter format for consistency with live streaming logs
			logLevel := "INF"
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

			if !h.shouldIncludeLevel(logLevel, level) {
				continue
			}

			// Parse timestamp and add .000 for alignment with live logs (which use 15:04:05.000)
			timeParts := strings.Fields(dateTime)
			var timestamp string
			if len(timeParts) >= 3 {
				timestamp = timeParts[len(timeParts)-1] + ".000"
			} else {
				timestamp = time.Now().Format("15:04:05.000")
			}

			logs = append(logs, map[string]interface{}{
				"timestamp": timestamp,
				"level":     logLevel,
				"message":   message,
			})
		}
	}

	if len(logs) == 0 {
		logs = []interface{}{}
	}

	batch := sseLogBatch{
		Logs: logs,
		Meta: sseMeta{
			TotalCount:     len(logs),
			DisplayedCount: len(logs),
			HasMore:        false,
		},
	}

	h.sendEvent(w, flusher, "logs", batch)
}

// sendInitialJobLogs sends initial job logs on connection
func (h *SSELogsHandler) sendInitialJobLogs(w http.ResponseWriter, flusher http.Flusher, jobID, stepID, level string, limit int) {
	ctx := context.Background()

	var logs []interface{}
	var totalCount int

	if stepID != "" {
		// Fetch step-specific logs (desc order to get most recent, then reverse for display)
		logEntries, _, _, err := h.logService.GetAggregatedLogs(ctx, jobID, false, level, limit, "", "desc")
		if err != nil {
			// Debug level: "job not found" is expected for stale job IDs after server restart
			h.logger.Debug().Err(err).Str("job_id", jobID).Msg("Failed to get initial job logs (job may not exist)")
			return
		}

		// Reverse to get ascending order (oldest first, newest last)
		for i := len(logEntries) - 1; i >= 0; i-- {
			log := logEntries[i]
			logs = append(logs, map[string]interface{}{
				"timestamp":   log.Timestamp,
				"level":       log.Level,
				"message":     log.Message,
				"job_id":      log.JobID(),
				"step_name":   log.StepName(),
				"line_number": log.LineNumber,
			})
		}

		totalCount, _ = h.logService.CountAggregatedLogs(ctx, jobID, false, level)
	} else {
		// Fetch job logs with children (desc order to get most recent, then reverse for display)
		logEntries, _, _, err := h.logService.GetAggregatedLogs(ctx, jobID, true, level, limit, "", "desc")
		if err != nil {
			// Debug level: "job not found" is expected for stale job IDs after server restart
			h.logger.Debug().Err(err).Str("job_id", jobID).Msg("Failed to get initial job logs (job may not exist)")
			return
		}

		// Reverse to get ascending order (oldest first, newest last)
		for i := len(logEntries) - 1; i >= 0; i-- {
			log := logEntries[i]
			logs = append(logs, map[string]interface{}{
				"timestamp":   log.Timestamp,
				"level":       log.Level,
				"message":     log.Message,
				"job_id":      log.JobID(),
				"step_name":   log.StepName(),
				"line_number": log.LineNumber,
			})
		}

		totalCount, _ = h.logService.CountAggregatedLogs(ctx, jobID, true, level)
	}

	if logs == nil {
		logs = []interface{}{}
	}

	batch := sseLogBatch{
		Logs: logs,
		Meta: sseMeta{
			TotalCount:     totalCount,
			DisplayedCount: len(logs),
			HasMore:        totalCount > len(logs),
		},
	}

	h.sendEvent(w, flusher, "logs", batch)
}

// sendServiceLogBatch sends a batch of service logs
func (h *SSELogsHandler) sendServiceLogBatch(w http.ResponseWriter, flusher http.Flusher, logs []interfaces.LogEntry) {
	logData := make([]interface{}, len(logs))
	for i, log := range logs {
		logData[i] = map[string]interface{}{
			"timestamp": log.Timestamp,
			"level":     log.Level,
			"message":   log.Message,
		}
	}

	batch := sseLogBatch{
		Logs: logData,
		Meta: sseMeta{
			DisplayedCount: len(logs),
		},
	}

	h.sendEvent(w, flusher, "logs", batch)
}

// sendJobLogBatch sends a batch of job logs
func (h *SSELogsHandler) sendJobLogBatch(w http.ResponseWriter, flusher http.Flusher, logs []jobLogEntry, jobID, stepID, level string) {
	ctx := context.Background()

	logData := make([]interface{}, len(logs))
	for i, log := range logs {
		logData[i] = map[string]interface{}{
			"timestamp":   log.Timestamp,
			"level":       log.Level,
			"message":     log.Message,
			"job_id":      log.JobID,
			"step_name":   log.StepName,
			"line_number": log.LineNumber,
		}
	}

	// Get total count
	totalCount := 0
	if stepID != "" {
		totalCount, _ = h.logService.CountAggregatedLogs(ctx, jobID, false, level)
	} else {
		totalCount, _ = h.logService.CountAggregatedLogs(ctx, jobID, true, level)
	}

	batch := sseLogBatch{
		Logs: logData,
		Meta: sseMeta{
			TotalCount:     totalCount,
			DisplayedCount: len(logs),
		},
	}

	h.sendEvent(w, flusher, "logs", batch)
}

// sendStatus sends a status update
func (h *SSELogsHandler) sendStatus(w http.ResponseWriter, flusher http.Flusher, status jobStatusUpdate) {
	h.sendEvent(w, flusher, "status", status)
}

// sendPing sends a heartbeat ping
func (h *SSELogsHandler) sendPing(w http.ResponseWriter, flusher http.Flusher) {
	h.sendEvent(w, flusher, "ping", ssePing{Timestamp: time.Now()})
}

// sendEvent writes an SSE event to the response
func (h *SSELogsHandler) sendEvent(w http.ResponseWriter, flusher http.Flusher, event string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to marshal SSE event data")
		return
	}

	fmt.Fprintf(w, "event: %s\n", event)
	fmt.Fprintf(w, "data: %s\n\n", jsonData)
	flusher.Flush()
}

// shouldIncludeLevel checks if a log entry should be included based on level filter
func (h *SSELogsHandler) shouldIncludeLevel(logLevel, filterLevel string) bool {
	if filterLevel == "all" || filterLevel == "debug" {
		return true
	}

	// Level hierarchy: error > warn > info > debug
	levelPriority := map[string]int{
		"error": 4,
		"err":   4,
		"warn":  3,
		"wrn":   3,
		"info":  2,
		"inf":   2,
		"debug": 1,
		"dbg":   1,
	}

	filterPrio := levelPriority[strings.ToLower(filterLevel)]
	logPrio := levelPriority[strings.ToLower(logLevel)]

	return logPrio >= filterPrio
}
