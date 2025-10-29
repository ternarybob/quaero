package handlers

import (
	"strings"

	plog "github.com/phuslu/log"
	"github.com/ternarybob/arbor/levels"
	"github.com/ternarybob/arbor/models"
	"github.com/ternarybob/arbor/writers"
)

// WebSocketWriter is an arbor writer that broadcasts logs to WebSocket clients
type WebSocketWriter struct {
	handler         *WebSocketHandler
	writer          writers.IGoroutineWriter
	config          models.WriterConfiguration
	minLevel        levels.LogLevel
	excludePatterns []string
}

// NewWebSocketWriter creates a new WebSocket arbor writer using GoroutineWriter pattern
func NewWebSocketWriter(handler *WebSocketHandler, config models.WriterConfiguration) (*WebSocketWriter, error) {
	w := &WebSocketWriter{
		handler:  handler,
		config:   config,
		minLevel: levels.InfoLevel,
		excludePatterns: []string{
			"WebSocket client connected",
			"WebSocket client disconnected",
			"HTTP request",
			"HTTP response",
			"Publishing Event",
			"DEBUG: Memory writer entry",
		},
	}

	// Define processor function for filtering and broadcasting
	processor := func(entry models.LogEvent) error {
		// Convert entry.Level from plog.Level to levels.LogLevel
		arborLevel := plogToArborLevel(entry.Level)

		// Filter by level
		if arborLevel < w.minLevel {
			return nil
		}

		// Filter by excluded patterns
		for _, pattern := range w.excludePatterns {
			if strings.Contains(entry.Message, pattern) {
				return nil
			}
		}

		// Transform to LogEntry format and broadcast
		logEntry := LogEntry{
			Timestamp: entry.Timestamp.Format("15:04:05"),
			Level:     mapLevel(arborLevel),
			Message:   entry.Message,
		}

		w.handler.BroadcastLog(logEntry)
		return nil
	}

	// Create GoroutineWriter with 1000-entry buffer
	gw, err := writers.NewGoroutineWriter(config, 1000, processor)
	if err != nil {
		return nil, err
	}
	gw.Start()

	w.writer = gw
	return w, nil
}

// plogToArborLevel converts phuslu/log.Level to arbor levels.LogLevel
func plogToArborLevel(level plog.Level) levels.LogLevel {
	switch level {
	case plog.ErrorLevel:
		return levels.ErrorLevel
	case plog.WarnLevel:
		return levels.WarnLevel
	case plog.InfoLevel:
		return levels.InfoLevel
	case plog.DebugLevel:
		return levels.DebugLevel
	default:
		return levels.InfoLevel
	}
}

// mapLevel maps arbor log levels to UI strings
func mapLevel(level levels.LogLevel) string {
	switch level {
	case levels.ErrorLevel:
		return "error"
	case levels.WarnLevel:
		return "warn"
	case levels.InfoLevel:
		return "info"
	case levels.DebugLevel:
		return "debug"
	default:
		return "info"
	}
}

// Write implements the IWriter interface - delegates to GoroutineWriter
func (w *WebSocketWriter) Write(data []byte) (int, error) {
	return w.writer.Write(data)
}

// WithLevel updates the minimum log level and returns self
func (w *WebSocketWriter) WithLevel(level plog.Level) writers.IWriter {
	// Map phuslu/log.Level to Arbor's levels.LogLevel
	switch level {
	case plog.ErrorLevel:
		w.minLevel = levels.ErrorLevel
	case plog.WarnLevel:
		w.minLevel = levels.WarnLevel
	case plog.InfoLevel:
		w.minLevel = levels.InfoLevel
	case plog.DebugLevel:
		w.minLevel = levels.DebugLevel
	default:
		w.minLevel = levels.InfoLevel
	}
	return w
}

// GetFilePath returns empty string (not file-based)
func (w *WebSocketWriter) GetFilePath() string {
	return ""
}

// Close performs graceful shutdown with buffer draining
func (w *WebSocketWriter) Close() error {
	return w.writer.Close()
}
