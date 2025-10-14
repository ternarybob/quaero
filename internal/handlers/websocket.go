// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 9:38:41 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
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
	logger       arbor.ILogger
	clients      map[*websocket.Conn]bool
	clientMutex  map[*websocket.Conn]*sync.Mutex
	mu           sync.RWMutex
	lastLogKeys  map[string]bool
	logKeysMu    sync.RWMutex
	authLoader   AuthLoader
	eventService interfaces.EventService
}

func NewWebSocketHandler(eventService interfaces.EventService, logger arbor.ILogger) *WebSocketHandler {
	h := &WebSocketHandler{
		logger:       logger,
		clients:      make(map[*websocket.Conn]bool),
		clientMutex:  make(map[*websocket.Conn]*sync.Mutex),
		lastLogKeys:  make(map[string]bool),
		eventService: eventService,
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

// BroadcastUILog sends a formatted log message directly to UI clients
// This bypasses the arbor logger and sends complete, formatted messages
func (h *WebSocketHandler) BroadcastUILog(level, message string) {
	timestamp := time.Now().Format("15:04:05")
	entry := LogEntry{
		Timestamp: timestamp,
		Level:     level,
		Message:   message,
	}
	h.BroadcastLog(entry)
}

// Message types
type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type StatusUpdate struct {
	Service       string `json:"service"`
	Status        string `json:"status"`
	Database      string `json:"database"`
	ExtensionAuth string `json:"extensionAuth"`
	ProjectsCount int    `json:"projectsCount"`
	IssuesCount   int    `json:"issuesCount"`
	PagesCount    int    `json:"pagesCount"`
	LastScrape    string `json:"lastScrape"`
}

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
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

	h.logger.Info().Msgf("WebSocket client connected (total: %d)", len(h.clients))

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
		h.logger.Info().Msgf("WebSocket client disconnected (remaining: %d)", clientCount)
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

// BroadcastLog sends log entries to all connected clients
func (h *WebSocketHandler) BroadcastLog(entry LogEntry) {
	msg := WSMessage{
		Type:    "log",
		Payload: entry,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to marshal log message")
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
			h.logger.Warn().Err(err).Msg("Failed to send log to client")
		}
	}
}

// sendStatus sends current status to a specific client
func (h *WebSocketHandler) sendStatus(conn *websocket.Conn) {
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

// SendLog is a helper to broadcast log entries
func (h *WebSocketHandler) SendLog(level, message string) {
	entry := LogEntry{
		Timestamp: time.Now().Format("15:04:05"),
		Level:     level,
		Message:   message,
	}
	h.BroadcastLog(entry)
}

// StartLogStreamer starts streaming logs from arbor's memory writer to WebSocket clients
func (h *WebSocketHandler) StartLogStreamer() {
	ticker := time.NewTicker(2 * time.Second)
	go func() {
		for range ticker.C {
			h.mu.RLock()
			clientCount := len(h.clients)
			h.mu.RUnlock()

			if clientCount > 0 {
				h.sendLogs()
			}
		}
	}()
}

// sendLogs retrieves logs from arbor memory writer and broadcasts them
func (h *WebSocketHandler) sendLogs() {
	logger := h.logger
	if logger == nil {
		return
	}

	// Try to get the memory writer for more efficient log retrieval
	memWriter := arbor.GetRegisteredMemoryWriter(arbor.WRITER_MEMORY)
	if memWriter != nil {
		entries, err := memWriter.GetEntriesWithLimit(50)
		if err != nil {
			h.logger.Warn().Err(err).Msg("Failed to get log entries from memory writer")
			return
		}

		if len(entries) == 0 {
			return
		}

		// Only send new log entries (ones we haven't seen before)
		h.logKeysMu.Lock()
		newKeys := make(map[string]bool)
		for key, logLine := range entries {
			newKeys[key] = true
			if !h.lastLogKeys[key] {
				h.parseAndBroadcastLog(logLine)
			}
		}
		h.lastLogKeys = newKeys
		h.logKeysMu.Unlock()

		return
	}

	// Fallback to logger method if memory writer not available
	entries, err := logger.GetMemoryLogsWithLimit(50)
	if err != nil {
		h.logger.Warn().Err(err).Msg("Failed to get log entries")
		return
	}

	if len(entries) == 0 {
		return
	}

	// Convert map to array and parse log entries
	for _, logLine := range entries {
		h.parseAndBroadcastLog(logLine)
	}
}

// parseAndBroadcastLog parses a log line and broadcasts it as a LogEntry
// Arbor memory writer format: "INF|Oct  2 16:27:13|Message key=value key2=value2"
// Output format: "[16:27:13] [INFO] Message key=value key2=value2"
func (h *WebSocketHandler) parseAndBroadcastLog(logLine string) {
	if logLine == "" {
		return
	}

	// Filter out internal handler logs (WebSocket, UI handler, etc.)
	if strings.Contains(logLine, "WebSocket client connected") ||
		strings.Contains(logLine, "WebSocket client disconnected") ||
		strings.Contains(logLine, "DEBUG: Memory writer entry") ||
		strings.Contains(logLine, "HTTP request") ||
		strings.Contains(logLine, "HTTP response") {
		return
	}

	// Parse arbor memory writer format: "LEVEL|Date Time|Message with fields"
	// Example: "INF|Oct  2 16:27:13|Stored pages count=25"
	parts := strings.SplitN(logLine, "|", 3)
	if len(parts) != 3 {
		return
	}

	levelStr := strings.TrimSpace(parts[0])
	dateTime := strings.TrimSpace(parts[1])
	messageWithFields := strings.TrimSpace(parts[2])

	// Map level
	level := "info"
	switch levelStr {
	case "ERR", "ERROR", "FATAL", "PANIC":
		level = "error"
	case "WRN", "WARN":
		level = "warn"
	case "INF", "INFO", "DBG", "DEBUG":
		level = "info"
	}

	// Extract just the time from "Oct  2 16:27:13"
	// Time is the last part after splitting by spaces
	timeParts := strings.Fields(dateTime)
	var timestamp string
	if len(timeParts) >= 3 {
		timestamp = timeParts[len(timeParts)-1] // Get last part (HH:MM:SS)
	} else {
		timestamp = time.Now().Format("15:04:05")
	}

	entry := LogEntry{
		Timestamp: timestamp,
		Level:     level,
		Message:   messageWithFields, // Include the full message with structured fields
	}
	h.BroadcastLog(entry)
}

// GetRecentLogsHandler returns recent logs from the last 5 minutes as JSON
func (h *WebSocketHandler) GetRecentLogsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get recent logs from memory writer
	memWriter := arbor.GetRegisteredMemoryWriter(arbor.WRITER_MEMORY)
	var logs []LogEntry

	if memWriter != nil {
		entries, err := memWriter.GetEntriesWithLimit(100)
		if err != nil {
			h.logger.Error().Err(err).Msg("Failed to get log entries")
			http.Error(w, "Failed to retrieve logs", http.StatusInternalServerError)
			return
		}

		// Parse and filter logs
		for _, logLine := range entries {
			// Skip internal handler logs
			if strings.Contains(logLine, "WebSocket client connected") ||
				strings.Contains(logLine, "WebSocket client disconnected") ||
				strings.Contains(logLine, "DEBUG: Memory writer entry") ||
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
			messageWithFields := strings.TrimSpace(parts[2])

			// Parse timestamp from "Oct  2 16:27:13" format
			timeParts := strings.Fields(dateTime)
			var timestamp string
			if len(timeParts) >= 3 {
				timestamp = timeParts[len(timeParts)-1]
			} else {
				timestamp = time.Now().Format("15:04:05")
			}

			// Map level
			level := "info"
			switch levelStr {
			case "ERR", "ERROR", "FATAL", "PANIC":
				level = "error"
			case "WRN", "WARN":
				level = "warn"
			case "INF", "INFO", "DBG", "DEBUG":
				level = "info"
			}

			entry := LogEntry{
				Timestamp: timestamp,
				Level:     level,
				Message:   messageWithFields,
			}

			logs = append(logs, entry)
		}
	}

	// Return empty array if no logs
	if logs == nil {
		logs = []LogEntry{}
	}

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

// SubscribeToCrawlerEvents subscribes to crawler progress events
func (h *WebSocketHandler) SubscribeToCrawlerEvents() {
	if h.eventService == nil {
		return
	}

	h.eventService.Subscribe(interfaces.EventCrawlProgress, func(ctx context.Context, event interfaces.Event) error {
		// Extract payload map
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			h.logger.Warn().Msg("Invalid crawl progress event payload type")
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
		if f, ok := val.(float64); ok {
			return f
		}
	}
	return 0.0
}
