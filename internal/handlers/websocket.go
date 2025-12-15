// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 9:38:41 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/services/events"
	"golang.org/x/time/rate"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for local development
	},
}

// AuthLoader interface for loading stored authentication
type AuthLoader interface {
	LoadAuth() (*interfaces.AuthData, error)
}

type WebSocketHandler struct {
	logger                 arbor.ILogger
	clients                map[*websocket.Conn]bool
	clientMutex            map[*websocket.Conn]*sync.Mutex
	mu                     sync.RWMutex
	authLoader             AuthLoader
	eventService           interfaces.EventService
	crawlProgressThrottler *rate.Limiter                // Rate limiter for crawl_progress events
	jobSpawnThrottler      *rate.Limiter                // Rate limiter for job_spawn events
	allowedEvents          map[string]bool              // Whitelist of events to broadcast (empty = allow all)
	unifiedLogAggregator   *events.UnifiedLogAggregator // Unified aggregator for service and step logs
	serverInstanceID       string                       // Unique ID generated on startup - clients use to detect server restart
}

func NewWebSocketHandler(eventService interfaces.EventService, logger arbor.ILogger, config *common.WebSocketConfig) *WebSocketHandler {
	h := &WebSocketHandler{
		logger:           logger,
		clients:          make(map[*websocket.Conn]bool),
		clientMutex:      make(map[*websocket.Conn]*sync.Mutex),
		eventService:     eventService,
		serverInstanceID: uuid.New().String(),
	}

	logger.Info().Str("server_instance_id", h.serverInstanceID).Msg("WebSocket handler initialized with server instance ID")

	// Initialize allowedEvents map (whitelist pattern)
	// Empty list means allow all events (backward compatible)
	h.allowedEvents = make(map[string]bool)
	if config != nil && len(config.AllowedEvents) > 0 {
		for _, eventType := range config.AllowedEvents {
			h.allowedEvents[eventType] = true
		}
		logger.Debug().
			Int("allowed_events", len(h.allowedEvents)).
			Msg("Initialized event whitelist for WebSocketHandler")
	}

	// Initialize throttlers from config (only if explicitly configured)
	// Nil throttlers = no throttling (disabled)
	if config != nil && len(config.ThrottleIntervals) > 0 {
		// Initialize crawl_progress throttler only if configured
		if intervalStr, ok := config.ThrottleIntervals["crawl_progress"]; ok {
			if duration, err := time.ParseDuration(intervalStr); err == nil {
				h.crawlProgressThrottler = rate.NewLimiter(rate.Every(duration), 1)
				logger.Debug().
					Str("event_type", "crawl_progress").
					Str("interval", intervalStr).
					Msg("Throttler initialized for crawl_progress events")
			} else {
				logger.Warn().
					Err(err).
					Str("interval", intervalStr).
					Msg("Failed to parse crawl_progress throttle interval - throttler disabled")
			}
		}

		// Initialize job_spawn throttler only if configured
		if intervalStr, ok := config.ThrottleIntervals["job_spawn"]; ok {
			if duration, err := time.ParseDuration(intervalStr); err == nil {
				h.jobSpawnThrottler = rate.NewLimiter(rate.Every(duration), 1)
				logger.Debug().
					Str("event_type", "job_spawn").
					Str("interval", intervalStr).
					Msg("Throttler initialized for job_spawn events")
			} else {
				logger.Warn().
					Err(err).
					Str("interval", intervalStr).
					Msg("Failed to parse job_spawn throttle interval - throttler disabled")
			}
		}
	}

	// Initialize unified log aggregator for trigger-based UI updates
	// Handles both service logs and step logs with a single aggregator
	// Triggers every timeThreshold (default 10s) for pending events
	// Also triggers immediately when a step finishes
	if config != nil {
		timeThreshold := 10 * time.Second // Default 10 seconds to reduce WebSocket message frequency
		if config.TimeThreshold != "" {
			if parsed, err := time.ParseDuration(config.TimeThreshold); err == nil {
				timeThreshold = parsed
			}
		}

		// Create unified aggregator with callback to broadcast refresh trigger
		arborLogger := arbor.NewLogger()
		h.unifiedLogAggregator = events.NewUnifiedLogAggregator(
			timeThreshold,
			h.broadcastUnifiedRefreshTrigger,
			arborLogger,
		)

		logger.Info().
			Dur("time_threshold", timeThreshold).
			Msg("Unified log aggregator initialized (service + step logs)")

		// Start periodic flush in background
		h.unifiedLogAggregator.StartPeriodicFlush(context.Background())
	}

	// Subscribe to crawler events if eventService is provided
	if eventService != nil {
		h.SubscribeToCrawlerEvents()
	}

	return h
}

// SetAuthLoader sets the auth loader for loading stored authentication
func (h *WebSocketHandler) SetAuthLoader(loader AuthLoader) {
	h.authLoader = loader
}

// broadcastUnifiedRefreshTrigger sends a unified WebSocket message to trigger UI refresh
// This is called by the unified log aggregator when thresholds are reached
// The message includes scope (service/job) and step_ids for job-scoped triggers
func (h *WebSocketHandler) broadcastUnifiedRefreshTrigger(ctx context.Context, trigger events.LogRefreshTrigger) {
	payload := map[string]interface{}{
		"scope":     trigger.Scope,
		"timestamp": trigger.Timestamp.Format(time.RFC3339),
	}

	// Include step_ids and finished flag for job scope
	if trigger.Scope == "job" {
		payload["step_ids"] = trigger.StepIDs
		payload["finished"] = trigger.Finished
	}

	msg := WSMessage{
		Type:    "refresh_logs",
		Payload: payload,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to marshal refresh_logs message")
		return
	}

	h.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	mutexes := make([]*sync.Mutex, 0, len(h.clients))
	for conn := range h.clients {
		clients = append(clients, conn)
		mutexes = append(mutexes, h.clientMutex[conn])
	}
	h.mu.RUnlock()

	for i, conn := range clients {
		mutex := mutexes[i]
		mutex.Lock()
		err := conn.WriteMessage(websocket.TextMessage, data)
		mutex.Unlock()

		if err != nil {
			h.logger.Warn().Err(err).Msg("Failed to send refresh_logs to client")
		}
	}

	// NOTE: Don't log here - logging would trigger another log_event
	// which would trigger another refresh_logs, creating an infinite loop
}

// Message types
type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type StatusUpdate struct {
	Service          string `json:"service"`
	Status           string `json:"status"`
	Database         string `json:"database"`
	ExtensionAuth    string `json:"extensionAuth"`
	ProjectsCount    int    `json:"projectsCount"`
	IssuesCount      int    `json:"issuesCount"`
	PagesCount       int    `json:"pagesCount"`
	LastScrape       string `json:"lastScrape"`
	ServerInstanceID string `json:"serverInstanceId"` // Unique ID per server startup - clients clear state on change
}

type CrawlProgressUpdate struct {
	JobID               string    `json:"jobId"`
	SourceType          string    `json:"sourceType"`
	EntityType          string    `json:"entityType"`
	Status              string    `json:"status"`
	TotalURLs           int       `json:"totalUrls"`
	CompletedURLs       int       `json:"completedUrls"`
	FailedURLs          int       `json:"failedUrls"`
	PendingURLs         int       `json:"pendingUrls"`
	CurrentURL          string    `json:"currentUrl,omitempty"`
	Percentage          float64   `json:"percentage"`
	EstimatedCompletion time.Time `json:"estimatedCompletion,omitempty"`
	Errors              []string  `json:"errors,omitempty"`
	Details             string    `json:"details,omitempty"`
}

type AppStatusUpdate struct {
	State     string                 `json:"state"`
	Metadata  map[string]interface{} `json:"metadata"`
	Timestamp time.Time              `json:"timestamp"`
}

type QueueStatsUpdate struct {
	TotalMessages    int       `json:"total_messages"`
	PendingMessages  int       `json:"pending_messages"`
	InFlightMessages int       `json:"in_flight_messages"`
	QueueName        string    `json:"queue_name"`
	Concurrency      int       `json:"concurrency"`
	Timestamp        time.Time `json:"timestamp"`
}

type JobSpawnUpdate struct {
	ParentJobID string    `json:"parent_job_id"`
	ChildJobID  string    `json:"child_job_id"`
	JobType     string    `json:"job_type"`
	URL         string    `json:"url,omitempty"`
	Depth       int       `json:"depth"`
	Timestamp   time.Time `json:"timestamp"`
}

type JobStatusUpdate struct {
	JobID             string    `json:"job_id"`
	ParentID          string    `json:"parent_id,omitempty"`           // Parent job ID (for child job identification)
	Status            string    `json:"status"`                        // "pending", "running", "completed", "failed", "cancelled"
	SourceType        string    `json:"source_type"`                   // "jira", "confluence", "github"
	EntityType        string    `json:"entity_type"`                   // "project", "issue", "space", "page"
	ResultCount       int       `json:"result_count"`                  // Documents successfully processed
	FailedCount       int       `json:"failed_count"`                  // Documents that failed
	TotalURLs         int       `json:"total_urls"`                    // Total URLs discovered
	CompletedURLs     int       `json:"completed_urls"`                // URLs completed
	PendingURLs       int       `json:"pending_urls"`                  // URLs still in queue
	Error             string    `json:"error,omitempty"`               // Error message for failed jobs
	Duration          float64   `json:"duration,omitempty"`            // Duration in seconds for completed jobs
	ChildCount        int       `json:"child_count,omitempty"`         // Total child jobs (for error tolerance context)
	ChildFailureCount int       `json:"child_failure_count,omitempty"` // Number of failed child jobs (for error tolerance)
	ErrorTolerance    int       `json:"error_tolerance,omitempty"`     // Error tolerance threshold (0 = unlimited)
	DocumentCount     int       `json:"document_count,omitempty"`      // Document count from job metadata
	Timestamp         time.Time `json:"timestamp"`                     // Event timestamp
	// Status report fields from backend (GetStatusReport)
	ProgressText    string   `json:"progress_text,omitempty"`    // Human-readable progress from backend
	Errors          []string `json:"errors,omitempty"`           // List of error messages from status_report
	Warnings        []string `json:"warnings,omitempty"`         // List of warning messages from status_report
	RunningChildren int      `json:"running_children,omitempty"` // Number of running child jobs
}

// JobUpdatePayload represents a unified job/step status update for real-time UI sync
// This is a simplified message format that replaces the multiple overlapping message types
type JobUpdatePayload struct {
	Context     string `json:"context"`                // "job" or "job_step"
	JobID       string `json:"job_id"`                 // Manager/parent job ID
	StepName    string `json:"step_name,omitempty"`    // Only for context="job_step"
	Status      string `json:"status"`                 // Job or step status
	RefreshLogs bool   `json:"refresh_logs,omitempty"` // True if UI should fetch updated logs
}

// CrawlerJobProgressUpdate represents real-time progress updates for crawler jobs
// This includes comprehensive parent-child job statistics and link following metrics
type CrawlerJobProgressUpdate struct {
	// Basic job information
	JobID     string    `json:"job_id"`
	ParentID  string    `json:"parent_id,omitempty"`
	Status    string    `json:"status"`
	JobType   string    `json:"job_type"`
	Timestamp time.Time `json:"timestamp"`

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

	// Current activity
	CurrentURL      string `json:"current_url,omitempty"`
	CurrentActivity string `json:"current_activity,omitempty"`
}

// HandleWebSocket handles WebSocket connections
func (h *WebSocketHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to upgrade WebSocket connection")
		return
	}

	h.mu.Lock()
	h.clients[conn] = true
	h.clientMutex[conn] = &sync.Mutex{}
	h.mu.Unlock()

	h.logger.Debug().Msgf("WebSocket client connected (total: %d)", len(h.clients))

	// Send initial status
	h.sendStatus(conn)

	// Send stored authentication if available
	if h.authLoader != nil {
		if authData, err := h.authLoader.LoadAuth(); err == nil {
			h.sendAuthToClient(conn, authData)
		}
	}

	// Handle client disconnection
	defer func() {
		h.mu.Lock()
		delete(h.clients, conn)
		delete(h.clientMutex, conn)
		clientCount := len(h.clients)
		h.mu.Unlock()

		conn.Close()
		h.logger.Debug().Msgf("WebSocket client disconnected (remaining: %d)", clientCount)
	}()

	// Read messages from client (keep connection alive)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Warn().Err(err).Msg("WebSocket error")
			}
			break
		}
	}
}

// BroadcastStatus sends status updates to all connected clients
func (h *WebSocketHandler) BroadcastStatus(status StatusUpdate) {
	msg := WSMessage{
		Type:    "status",
		Payload: status,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to marshal status message")
		return
	}

	h.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	mutexes := make([]*sync.Mutex, 0, len(h.clients))
	for conn := range h.clients {
		clients = append(clients, conn)
		mutexes = append(mutexes, h.clientMutex[conn])
	}
	h.mu.RUnlock()

	for i, conn := range clients {
		mutex := mutexes[i]
		mutex.Lock()
		err := conn.WriteMessage(websocket.TextMessage, data)
		mutex.Unlock()

		if err != nil {
			h.logger.Warn().Err(err).Msg("Failed to send status to client")
		}
	}
}

// sendAuthToClient sends authentication data to a single client
func (h *WebSocketHandler) sendAuthToClient(conn *websocket.Conn, authData *interfaces.AuthData) {
	type AuthPayload struct {
		BaseURL   string                        `json:"baseUrl"`
		CloudID   string                        `json:"cloudId"`
		Cookies   []*interfaces.ExtensionCookie `json:"cookies"`
		Timestamp int64                         `json:"timestamp"`
	}

	cloudID := ""
	if cid, ok := authData.Tokens["cloudId"].(string); ok {
		cloudID = cid
	}

	payload := AuthPayload{
		BaseURL:   authData.BaseURL,
		CloudID:   cloudID,
		Cookies:   authData.Cookies,
		Timestamp: authData.Timestamp,
	}

	msg := WSMessage{
		Type:    "auth",
		Payload: payload,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to marshal auth message")
		return
	}

	mutex := h.clientMutex[conn]
	if mutex != nil {
		mutex.Lock()
		conn.WriteMessage(websocket.TextMessage, data)
		mutex.Unlock()
	}
}

// BroadcastAuth sends authentication data to all connected clients
func (h *WebSocketHandler) BroadcastAuth(authData *interfaces.AuthData) {
	h.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	for conn := range h.clients {
		clients = append(clients, conn)
	}
	h.mu.RUnlock()

	for _, conn := range clients {
		h.sendAuthToClient(conn, authData)
	}
}

// sendStatus sends current status to a specific client
func (h *WebSocketHandler) sendStatus(conn *websocket.Conn) {
	status := StatusUpdate{
		Service:          "ONLINE",
		Status:           "ONLINE",
		Database:         "CONNECTED",
		ExtensionAuth:    "WAITING",
		ProjectsCount:    0,
		IssuesCount:      0,
		PagesCount:       0,
		LastScrape:       "Never",
		ServerInstanceID: h.serverInstanceID,
	}

	msg := WSMessage{
		Type:    "status",
		Payload: status,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to marshal initial status")
		return
	}

	h.mu.RLock()
	mutex := h.clientMutex[conn]
	h.mu.RUnlock()

	if mutex != nil {
		mutex.Lock()
		err := conn.WriteMessage(websocket.TextMessage, data)
		mutex.Unlock()

		if err != nil {
			h.logger.Warn().Err(err).Msg("Failed to send initial status")
		}
	}
}

// StartStatusBroadcaster starts periodic status updates
func (h *WebSocketHandler) StartStatusBroadcaster() {
	ticker := time.NewTicker(5 * time.Second)

	go func() {
		for range ticker.C {
			h.mu.RLock()
			clientCount := len(h.clients)
			h.mu.RUnlock()

			if clientCount > 0 {
				status := StatusUpdate{
					Service:       "ONLINE",
					Status:        "ONLINE",
					Database:      "CONNECTED",
					ExtensionAuth: "WAITING",
					ProjectsCount: 0,
					IssuesCount:   0,
					PagesCount:    0,
					LastScrape:    "Never",
				}
				h.BroadcastStatus(status)
			}
		}
	}()
}

// GetRecentLogsHandler returns recent logs from the last 5 minutes as JSON
func (h *WebSocketHandler) GetRecentLogsHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodGet) {
		return
	}

	// Get recent logs from memory writer
	memWriter := arbor.GetRegisteredMemoryWriter(arbor.WRITER_MEMORY)
	var logs []interfaces.LogEntry

	if memWriter != nil {
		entries, err := memWriter.GetEntriesWithLimit(100)
		if err != nil {
			h.logger.Error().Err(err).Msg("Failed to get log entries")
			http.Error(w, "Failed to retrieve logs", http.StatusInternalServerError)
			return
		}

		// Extract and sort keys for deterministic ordering
		// Map keys are timestamps like "2025-01-01T12:00:00.000Z" - sorting gives chronological order
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

			// Parse log line
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
			level := "INF" // Default
			switch levelStr {
			case "ERR", "ERROR", "FATAL", "PANIC":
				level = "ERR"
			case "WRN", "WARN":
				level = "WRN"
			case "INF", "INFO":
				level = "INF"
			case "DBG", "DEBUG":
				level = "DBG"
			}

			entry := interfaces.LogEntry{
				Index:     len(logs), // Assign index based on insertion order from memory writer
				Timestamp: timestamp,
				Level:     level,
				Message:   messageWithFields,
			}

			logs = append(logs, entry)
		}
	}

	// Return empty array if no logs
	if logs == nil {
		logs = []interfaces.LogEntry{}
	}

	// Sort logs by index (insertion order from memory writer)
	// This preserves the exact order logs were received, even when timestamps collide
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Index < logs[j].Index
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs":  logs,
		"count": len(logs),
	})
}

// BroadcastCrawlProgress sends crawler progress updates to all connected clients
func (h *WebSocketHandler) BroadcastCrawlProgress(progress CrawlProgressUpdate) {
	msg := WSMessage{
		Type:    "crawl_progress",
		Payload: progress,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to marshal crawl progress message")
		return
	}

	h.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	mutexes := make([]*sync.Mutex, 0, len(h.clients))
	for conn := range h.clients {
		clients = append(clients, conn)
		mutexes = append(mutexes, h.clientMutex[conn])
	}
	h.mu.RUnlock()

	for i, conn := range clients {
		mutex := mutexes[i]
		mutex.Lock()
		err := conn.WriteMessage(websocket.TextMessage, data)
		mutex.Unlock()

		if err != nil {
			h.logger.Warn().Err(err).Msg("Failed to send crawl progress to client")
		}
	}
}

// BroadcastAppStatus sends application status updates to all connected clients
func (h *WebSocketHandler) BroadcastAppStatus(status AppStatusUpdate) {
	msg := WSMessage{
		Type:    "app_status",
		Payload: status,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to marshal app status message")
		return
	}

	h.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	mutexes := make([]*sync.Mutex, 0, len(h.clients))
	for conn := range h.clients {
		clients = append(clients, conn)
		mutexes = append(mutexes, h.clientMutex[conn])
	}
	h.mu.RUnlock()

	for i, conn := range clients {
		mutex := mutexes[i]
		mutex.Lock()
		err := conn.WriteMessage(websocket.TextMessage, data)
		mutex.Unlock()

		if err != nil {
			h.logger.Warn().Err(err).Msg("Failed to send app status to client")
		}
	}
}

// BroadcastQueueStats sends queue statistics to all connected clients
func (h *WebSocketHandler) BroadcastQueueStats(stats QueueStatsUpdate) {
	msg := WSMessage{
		Type:    "queue_stats",
		Payload: stats,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to marshal queue stats message")
		return
	}

	h.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	mutexes := make([]*sync.Mutex, 0, len(h.clients))
	for conn := range h.clients {
		clients = append(clients, conn)
		mutexes = append(mutexes, h.clientMutex[conn])
	}
	h.mu.RUnlock()

	for i, conn := range clients {
		mutex := mutexes[i]
		mutex.Lock()
		err := conn.WriteMessage(websocket.TextMessage, data)
		mutex.Unlock()

		if err != nil {
			h.logger.Warn().Err(err).Msg("Failed to send queue stats to client")
		}
	}
}

// BroadcastJobSpawn sends job spawning events to all connected clients
func (h *WebSocketHandler) BroadcastJobSpawn(spawn JobSpawnUpdate) {
	msg := WSMessage{
		Type:    "job_spawn",
		Payload: spawn,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to marshal job spawn message")
		return
	}

	h.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	mutexes := make([]*sync.Mutex, 0, len(h.clients))
	for conn := range h.clients {
		clients = append(clients, conn)
		mutexes = append(mutexes, h.clientMutex[conn])
	}
	h.mu.RUnlock()

	for i, conn := range clients {
		mutex := mutexes[i]
		mutex.Lock()
		err := conn.WriteMessage(websocket.TextMessage, data)
		mutex.Unlock()

		if err != nil {
			h.logger.Warn().Err(err).Msg("Failed to send job spawn to client")
		}
	}
}

// BroadcastJobStatusChange sends job status change events to all connected clients
func (h *WebSocketHandler) BroadcastJobStatusChange(update JobStatusUpdate) {
	msg := WSMessage{
		Type:    "job_status_change",
		Payload: update,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to marshal job status change message")
		return
	}

	h.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	mutexes := make([]*sync.Mutex, 0, len(h.clients))
	for conn := range h.clients {
		clients = append(clients, conn)
		mutexes = append(mutexes, h.clientMutex[conn])
	}
	h.mu.RUnlock()

	for i, conn := range clients {
		mutex := mutexes[i]
		mutex.Lock()
		err := conn.WriteMessage(websocket.TextMessage, data)
		mutex.Unlock()

		if err != nil {
			h.logger.Warn().Err(err).Msg("Failed to send job status change to client")
		}
	}
}

// BroadcastCrawlerJobProgress sends crawler job progress updates to all connected clients
// This includes comprehensive parent-child job statistics and link following metrics
func (h *WebSocketHandler) BroadcastCrawlerJobProgress(update CrawlerJobProgressUpdate) {
	msg := WSMessage{
		Type:    "crawler_job_progress",
		Payload: update,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to marshal crawler job progress message")
		return
	}

	h.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	mutexes := make([]*sync.Mutex, 0, len(h.clients))
	for conn := range h.clients {
		clients = append(clients, conn)
		mutexes = append(mutexes, h.clientMutex[conn])
	}
	h.mu.RUnlock()

	for i, conn := range clients {
		mutex := mutexes[i]
		mutex.Lock()
		err := conn.WriteMessage(websocket.TextMessage, data)
		mutex.Unlock()

		if err != nil {
			h.logger.Warn().Err(err).Msg("Failed to send crawler job progress to client")
		}
	}
}

// BroadcastJobUpdate sends a unified job/step status update to all connected clients
// This is the primary method for real-time UI status synchronization
// context: "job" for overall job status, "job_step" for step-level status
func (h *WebSocketHandler) BroadcastJobUpdate(jobID, context, stepName, status string, refreshLogs bool) {
	payload := JobUpdatePayload{
		Context:     context,
		JobID:       jobID,
		Status:      status,
		RefreshLogs: refreshLogs,
	}
	if context == "job_step" && stepName != "" {
		payload.StepName = stepName
	}

	msg := WSMessage{
		Type:    "job_update",
		Payload: payload,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to marshal job update message")
		return
	}

	h.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	mutexes := make([]*sync.Mutex, 0, len(h.clients))
	for conn := range h.clients {
		clients = append(clients, conn)
		mutexes = append(mutexes, h.clientMutex[conn])
	}
	h.mu.RUnlock()

	for i, conn := range clients {
		mutex := mutexes[i]
		mutex.Lock()
		err := conn.WriteMessage(websocket.TextMessage, data)
		mutex.Unlock()

		if err != nil {
			h.logger.Warn().Err(err).Msg("Failed to send job update to client")
		}
	}
}

// SubscribeToCrawlerEvents subscribes to crawler progress events
func (h *WebSocketHandler) SubscribeToCrawlerEvents() {
	if h.eventService == nil {
		return
	}

	// Subscribe to log events from LogService (replaces direct BroadcastLog calls)
	// Uses unified aggregator for trigger-based UI updates - NO direct log broadcasting
	// Architecture: WebSocket sends only triggers, UI fetches logs from REST API
	h.eventService.Subscribe("log_event", func(ctx context.Context, event interfaces.Event) error {
		// Route all service logs through the unified aggregator for trigger-based updates
		// This avoids heavy WebSocket load from streaming thousands of log entries
		if h.unifiedLogAggregator != nil {
			h.unifiedLogAggregator.RecordServiceLog(ctx)
		}
		// No fallback - aggregator is always initialized in production
		// If not initialized, logs are still persisted and available via REST API
		return nil
	})

	h.eventService.Subscribe(interfaces.EventCrawlProgress, func(ctx context.Context, event interfaces.Event) error {
		// Extract payload map
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			h.logger.Warn().Msg("Invalid crawl progress event payload type")
			return nil
		}

		// Check whitelist (empty allowedEvents = allow all)
		if len(h.allowedEvents) > 0 && !h.allowedEvents["crawl_progress"] {
			return nil
		}

		// Throttle crawl progress events to prevent WebSocket flooding
		if h.crawlProgressThrottler != nil && !h.crawlProgressThrottler.Allow() {
			// Event throttled, skip broadcasting
			return nil
		}

		// Convert to CrawlProgressUpdate struct
		progress := CrawlProgressUpdate{
			JobID:         getString(payload, "job_id"),
			SourceType:    getString(payload, "source_type"),
			EntityType:    getString(payload, "entity_type"),
			Status:        getString(payload, "status"),
			TotalURLs:     getInt(payload, "total_urls"),
			CompletedURLs: getInt(payload, "completed_urls"),
			FailedURLs:    getInt(payload, "failed_urls"),
			PendingURLs:   getInt(payload, "pending_urls"),
			CurrentURL:    getString(payload, "current_url"),
			Percentage:    getFloat64(payload, "percentage"),
		}

		// Parse estimated completion if present
		if estStr := getString(payload, "estimated_completion"); estStr != "" {
			if est, err := time.Parse(time.RFC3339, estStr); err == nil {
				progress.EstimatedCompletion = est
			}
		}

		// Extract errors array if present
		if errs, ok := payload["errors"].([]interface{}); ok {
			progress.Errors = make([]string, 0, len(errs))
			for _, e := range errs {
				if errStr, ok := e.(string); ok {
					progress.Errors = append(progress.Errors, errStr)
				}
			}
		}

		// Extract details if present
		progress.Details = getString(payload, "details")

		// Broadcast to all clients
		h.BroadcastCrawlProgress(progress)
		return nil
	})

	h.eventService.Subscribe(interfaces.EventStatusChanged, func(ctx context.Context, event interfaces.Event) error {
		// Extract payload map
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			h.logger.Warn().Msg("Invalid status changed event payload type")
			return nil
		}

		// Convert to AppStatusUpdate struct
		update := AppStatusUpdate{
			State:     getString(payload, "state"),
			Metadata:  make(map[string]interface{}),
			Timestamp: time.Now(),
		}

		// Extract metadata if present
		if metadata, ok := payload["metadata"].(map[string]interface{}); ok {
			update.Metadata = metadata
		}

		// Parse timestamp if present
		if tsStr := getString(payload, "timestamp"); tsStr != "" {
			if ts, err := time.Parse(time.RFC3339, tsStr); err == nil {
				update.Timestamp = ts
			}
		}

		// Broadcast to all clients
		h.BroadcastAppStatus(update)
		return nil
	})

	h.eventService.Subscribe(interfaces.EventJobSpawn, func(ctx context.Context, event interfaces.Event) error {
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			h.logger.Warn().Msg("Invalid job spawn event payload type")
			return nil
		}

		// Check whitelist (empty allowedEvents = allow all)
		if len(h.allowedEvents) > 0 && !h.allowedEvents["job_spawn"] {
			return nil
		}

		// Throttle job spawn events to prevent WebSocket flooding
		if h.jobSpawnThrottler != nil && !h.jobSpawnThrottler.Allow() {
			// Event throttled, skip broadcasting
			return nil
		}

		spawn := JobSpawnUpdate{
			ParentJobID: getString(payload, "parent_job_id"),
			ChildJobID:  getString(payload, "child_job_id"),
			JobType:     getString(payload, "job_type"),
			URL:         getString(payload, "url"),
			Depth:       getInt(payload, "depth"),
			Timestamp:   time.Now(),
		}

		h.BroadcastJobSpawn(spawn)
		return nil
	})

	// Subscribe to crawler job progress events for real-time monitoring
	h.eventService.Subscribe("crawler_job_progress", func(ctx context.Context, event interfaces.Event) error {
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			h.logger.Warn().Msg("Invalid crawler job progress event payload type")
			return nil
		}

		// Check whitelist (empty allowedEvents = allow all)
		if len(h.allowedEvents) > 0 && !h.allowedEvents["crawler_job_progress"] {
			return nil
		}

		// Convert payload to CrawlerJobProgressUpdate
		update := CrawlerJobProgressUpdate{
			JobID:             getString(payload, "job_id"),
			ParentID:          getString(payload, "parent_id"),
			Status:            getString(payload, "status"),
			JobType:           getString(payload, "job_type"),
			Timestamp:         time.Now(),
			TotalChildren:     getInt(payload, "total_children"),
			CompletedChildren: getInt(payload, "completed_children"),
			FailedChildren:    getInt(payload, "failed_children"),
			RunningChildren:   getInt(payload, "running_children"),
			PendingChildren:   getInt(payload, "pending_children"),
			CancelledChildren: getInt(payload, "cancelled_children"),
			OverallProgress:   getFloat64(payload, "overall_progress"),
			ProgressText:      getString(payload, "progress_text"),
			LinksFound:        getInt(payload, "links_found"),
			LinksFiltered:     getInt(payload, "links_filtered"),
			LinksFollowed:     getInt(payload, "links_followed"),
			LinksSkipped:      getInt(payload, "links_skipped"),
			CurrentURL:        getString(payload, "current_url"),
			CurrentActivity:   getString(payload, "current_activity"),
		}

		// Parse timing information
		if startedAtStr := getString(payload, "started_at"); startedAtStr != "" {
			if startedAt, err := time.Parse(time.RFC3339, startedAtStr); err == nil {
				update.StartedAt = &startedAt
			}
		}

		if estimatedEndStr := getString(payload, "estimated_end"); estimatedEndStr != "" {
			if estimatedEnd, err := time.Parse(time.RFC3339, estimatedEndStr); err == nil {
				update.EstimatedEnd = &estimatedEnd
			}
		}

		if duration := getFloat64(payload, "duration_seconds"); duration > 0 {
			update.Duration = &duration
		}

		// Extract errors and warnings
		update.Errors = getStringSlice(payload, "errors")
		update.Warnings = getStringSlice(payload, "warnings")

		// Broadcast to all clients
		h.BroadcastCrawlerJobProgress(update)
		return nil
	})

	// Subscribe to parent job progress events for real-time monitoring
	h.eventService.Subscribe("parent_job_progress", func(ctx context.Context, event interfaces.Event) error {
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			h.logger.Warn().Msg("Invalid parent_job_progress event payload type")
			return nil
		}

		// Check whitelist (empty allowedEvents = allow all)
		if len(h.allowedEvents) > 0 && !h.allowedEvents["parent_job_progress"] {
			return nil
		}

		// Extract job_id and progress_text for WebSocket message
		jobID := getString(payload, "job_id")
		progressText := getString(payload, "progress_text")
		status := getString(payload, "status")

		// Create simplified WebSocket message with job_id key
		// UI will use job_id to update specific job row
		wsPayload := map[string]interface{}{
			"job_id":        jobID,
			"progress_text": progressText, // "66 pending, 1 running, 41 completed, 0 failed"
			"status":        status,
			"timestamp":     getString(payload, "timestamp"),

			// Include child statistics for advanced UI features
			"total_children":     getInt(payload, "total_children"),
			"pending_children":   getInt(payload, "pending_children"),
			"running_children":   getInt(payload, "running_children"),
			"completed_children": getInt(payload, "completed_children"),
			"failed_children":    getInt(payload, "failed_children"),
			"cancelled_children": getInt(payload, "cancelled_children"),
			"document_count":     getInt(payload, "document_count"),
		}

		// Broadcast to all clients
		msg := WSMessage{
			Type:    "parent_job_progress",
			Payload: wsPayload,
		}

		data, err := json.Marshal(msg)
		if err != nil {
			h.logger.Error().Err(err).Msg("Failed to marshal parent job progress message")
			return nil
		}

		h.mu.RLock()
		clients := make([]*websocket.Conn, 0, len(h.clients))
		mutexes := make([]*sync.Mutex, 0, len(h.clients))
		for conn := range h.clients {
			clients = append(clients, conn)
			mutexes = append(mutexes, h.clientMutex[conn])
		}
		h.mu.RUnlock()

		for i, conn := range clients {
			mutex := mutexes[i]
			mutex.Lock()
			err := conn.WriteMessage(websocket.TextMessage, data)
			mutex.Unlock()

			if err != nil {
				h.logger.Warn().Err(err).Msg("Failed to send parent job progress to client")
			}
		}

		return nil
	})

	// Subscribe to job step progress events for real-time monitoring of multi-step jobs
	h.eventService.Subscribe(interfaces.EventJobProgress, func(ctx context.Context, event interfaces.Event) error {
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			h.logger.Warn().Msg("Invalid job_progress event payload type")
			return nil
		}

		// Check whitelist (empty allowedEvents = allow all)
		if len(h.allowedEvents) > 0 && !h.allowedEvents["job_progress"] {
			return nil
		}

		// Create WebSocket message for step progress
		wsPayload := map[string]interface{}{
			"job_id":           getString(payload, "job_id"),
			"job_name":         getString(payload, "job_name"),
			"step_index":       getInt(payload, "step_index"),
			"step_name":        getString(payload, "step_name"),
			"step_type":        getString(payload, "step_type"),
			"current_step":     getInt(payload, "current_step"),
			"total_steps":      getInt(payload, "total_steps"),
			"step_status":      getString(payload, "step_status"),
			"step_child_count": getInt(payload, "step_child_count"),
			"timestamp":        getString(payload, "timestamp"),
		}

		// Broadcast to all clients
		msg := WSMessage{
			Type:    "job_step_progress",
			Payload: wsPayload,
		}

		data, err := json.Marshal(msg)
		if err != nil {
			h.logger.Error().Err(err).Msg("Failed to marshal job step progress message")
			return nil
		}

		h.mu.RLock()
		clients := make([]*websocket.Conn, 0, len(h.clients))
		mutexes := make([]*sync.Mutex, 0, len(h.clients))
		for conn := range h.clients {
			clients = append(clients, conn)
			mutexes = append(mutexes, h.clientMutex[conn])
		}
		h.mu.RUnlock()

		for i, conn := range clients {
			mutex := mutexes[i]
			mutex.Lock()
			err := conn.WriteMessage(websocket.TextMessage, data)
			mutex.Unlock()

			if err != nil {
				h.logger.Warn().Err(err).Msg("Failed to send job step progress to client")
			}
		}

		return nil
	})

	// Subscribe to step progress events from StepMonitor (for step-level progress updates)
	// Uses unified aggregator for trigger-based UI updates instead of direct broadcast
	h.eventService.Subscribe(interfaces.EventStepProgress, func(ctx context.Context, event interfaces.Event) error {
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			h.logger.Warn().Msg("Invalid step_progress event payload type")
			return nil
		}

		// Check whitelist (empty allowedEvents = allow all)
		if len(h.allowedEvents) > 0 && !h.allowedEvents["step_progress"] {
			return nil
		}

		// Extract step_id for aggregation
		stepID := getString(payload, "step_id")
		if stepID == "" {
			h.logger.Warn().Msg("Step progress event missing step_id")
			return nil
		}

		// Use unified aggregator for trigger-based updates if available
		if h.unifiedLogAggregator != nil {
			// Check if step finished - trigger immediately for final state
			status := getString(payload, "status")
			if status == "completed" || status == "failed" || status == "cancelled" {
				// Step finished - trigger immediate refresh so UI shows final events
				h.unifiedLogAggregator.TriggerStepImmediately(ctx, stepID)
			} else {
				// Step still running - record event for periodic trigger
				h.unifiedLogAggregator.RecordStepEvent(ctx, stepID)
			}
			return nil
		}

		// Fallback: Direct broadcast (legacy behavior if aggregator not initialized)
		msg := WSMessage{
			Type:    "step_progress",
			Payload: payload,
		}

		data, err := json.Marshal(msg)
		if err != nil {
			h.logger.Error().Err(err).Msg("Failed to marshal step progress message")
			return nil
		}

		h.mu.RLock()
		clients := make([]*websocket.Conn, 0, len(h.clients))
		mutexes := make([]*sync.Mutex, 0, len(h.clients))
		for conn := range h.clients {
			clients = append(clients, conn)
			mutexes = append(mutexes, h.clientMutex[conn])
		}
		h.mu.RUnlock()

		for i, conn := range clients {
			mutex := mutexes[i]
			mutex.Lock()
			err := conn.WriteMessage(websocket.TextMessage, data)
			mutex.Unlock()

			if err != nil {
				h.logger.Warn().Err(err).Msg("Failed to send step progress to client")
			}
		}

		return nil
	})

	// Subscribe to unified job log events (EventJobLog) for all job types
	// Architecture: NO direct log broadcasting via WebSocket - use trigger-based approach
	// Logs are persisted to storage and UI fetches them via REST API when triggered
	// This prevents heavy WebSocket load when jobs produce 5000+ log entries
	h.eventService.Subscribe(interfaces.EventJobLog, func(ctx context.Context, event interfaces.Event) error {
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			return nil
		}

		// Route all job logs through the unified aggregator for trigger-based updates
		// The aggregator will send a refresh_logs trigger, and UI will fetch from /api/logs
		if h.unifiedLogAggregator != nil {
			// Use step_id to aggregate ALL logs for a step (manager -> step -> worker).
			// This prevents refresh_logs from carrying thousands of worker job IDs and flooding /api/logs.
			stepID := getString(payload, "step_id")
			if stepID == "" && getString(payload, "step_name") != "" {
				// Backward-compatible fallback: older payloads didn't include step_id.
				stepID = getString(payload, "job_id")
			}

			if stepID != "" {
				h.unifiedLogAggregator.RecordStepEvent(ctx, stepID)
			} else {
				// Non-step job logs - record as service log
				h.unifiedLogAggregator.RecordServiceLog(ctx)
			}
		}
		// Logs are already persisted to storage by LogService
		// UI will fetch them via REST API when it receives the refresh trigger
		return nil
	})

	// Subscribe to job stats events for real-time queue statistics dashboard
	h.eventService.Subscribe(interfaces.EventJobStats, func(ctx context.Context, event interfaces.Event) error {
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			h.logger.Warn().Msg("Invalid job_stats event payload type")
			return nil
		}

		// Check whitelist (empty allowedEvents = allow all)
		if len(h.allowedEvents) > 0 && !h.allowedEvents["job_stats"] {
			return nil
		}

		// Broadcast job stats to all clients
		msg := WSMessage{
			Type:    "job_stats",
			Payload: payload,
		}

		data, err := json.Marshal(msg)
		if err != nil {
			h.logger.Error().Err(err).Msg("Failed to marshal job stats message")
			return nil
		}

		h.mu.RLock()
		clients := make([]*websocket.Conn, 0, len(h.clients))
		mutexes := make([]*sync.Mutex, 0, len(h.clients))
		for conn := range h.clients {
			clients = append(clients, conn)
			mutexes = append(mutexes, h.clientMutex[conn])
		}
		h.mu.RUnlock()

		for i, conn := range clients {
			mutex := mutexes[i]
			mutex.Lock()
			err := conn.WriteMessage(websocket.TextMessage, data)
			mutex.Unlock()

			if err != nil {
				h.logger.Warn().Err(err).Msg("Failed to send job stats to client")
			}
		}

		return nil
	})

	// Subscribe to unified job update events for real-time UI status sync
	// This is a direct broadcast that bypasses the log aggregator for immediate status updates
	h.eventService.Subscribe(interfaces.EventJobUpdate, func(ctx context.Context, event interfaces.Event) error {
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			h.logger.Warn().Msg("Invalid job_update event payload type")
			return nil
		}

		// Check whitelist (empty allowedEvents = allow all)
		if len(h.allowedEvents) > 0 && !h.allowedEvents["job_update"] {
			return nil
		}

		// Extract fields and call BroadcastJobUpdate
		jobID := getString(payload, "job_id")
		context := getString(payload, "context")
		stepName := getString(payload, "step_name")
		status := getString(payload, "status")
		refreshLogs := false
		if rl, ok := payload["refresh_logs"].(bool); ok {
			refreshLogs = rl
		}

		h.BroadcastJobUpdate(jobID, context, stepName, status, refreshLogs)
		return nil
	})
}

// Helper functions for safe type conversion from map[string]interface{}
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return 0
}

func getFloat64(m map[string]interface{}, key string) float64 {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case string:
			// Attempt to parse numeric strings
			if f, err := time.ParseDuration(v); err == nil {
				// Handle duration strings like "5s", "1m", etc.
				return f.Seconds()
			}
			// Try parsing as plain float
			var f float64
			if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
				return f
			}
		}
	}
	return 0.0
}

func getStringSlice(m map[string]interface{}, key string) []string {
	if val, ok := m[key]; ok {
		// Try to convert from []interface{} (JSON arrays)
		if arr, ok := val.([]interface{}); ok {
			result := make([]string, 0, len(arr))
			for _, item := range arr {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		}
		// Try direct []string type assertion
		if arr, ok := val.([]string); ok {
			return arr
		}
	}
	return []string{}
}
