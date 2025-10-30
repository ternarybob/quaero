package handlers

import (
	"strings"

	plog "github.com/phuslu/log"
	"github.com/ternarybob/arbor/levels"
	"github.com/ternarybob/arbor/models"
	"github.com/ternarybob/arbor/writers"

	"github.com/ternarybob/quaero/internal/common"
)

const (
	// Default buffer size for WebSocket log queue
	defaultWebSocketBufferSize = 1000
)

// WebSocketWriter is an arbor writer that broadcasts logs to WebSocket clients
type WebSocketWriter struct {
	handler         *WebSocketHandler
	writer          writers.IChannelWriter
	config          models.WriterConfiguration
	minLevel        levels.LogLevel
	excludePatterns []string
}

// NewWebSocketWriter creates a new WebSocket arbor writer using ChannelWriter pattern
func NewWebSocketWriter(handler *WebSocketHandler, config models.WriterConfiguration, wsConfig *common.WebSocketConfig) (*WebSocketWriter, error) {
	// Nil-safety check: use safe defaults if wsConfig is nil
	var minLevel levels.LogLevel
	var excludePatterns []string

	if wsConfig == nil {
		// Use safe defaults when no config provided
		minLevel = levels.InfoLevel
		excludePatterns = []string{
			"WebSocket client connected",
			"WebSocket client disconnected",
			"HTTP request",
			"HTTP response",
			"Publishing event", // Fixed case to match actual log message
			"DEBUG: Memory writer entry",
		}
	} else {
		// Parse min level from config, default to InfoLevel
		minLevel = parseLogLevel(wsConfig.MinLevel)

		// Use config exclude patterns with fallback to defaults
		excludePatterns = wsConfig.ExcludePatterns
		if len(excludePatterns) == 0 {
			excludePatterns = []string{
				"WebSocket client connected",
				"WebSocket client disconnected",
				"HTTP request",
				"HTTP response",
				"Publishing event", // Fixed case to match actual log message
				"DEBUG: Memory writer entry",
			}
		}
	}

	w := &WebSocketWriter{
		handler:         handler,
		config:          config,
		minLevel:        minLevel,
		excludePatterns: excludePatterns,
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

	// Create ChannelWriter with buffer
	cw, err := writers.NewChannelWriter(config, defaultWebSocketBufferSize, processor)
	if err != nil {
		return nil, err
	}
	cw.Start()

	w.writer = cw
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

// parseLogLevel converts string log level to arbor levels.LogLevel
func parseLogLevel(level string) levels.LogLevel {
	switch strings.ToLower(level) {
	case "error":
		return levels.ErrorLevel
	case "warn", "warning":
		return levels.WarnLevel
	case "info":
		return levels.InfoLevel
	case "debug":
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
	w.minLevel = plogToArborLevel(level)
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
