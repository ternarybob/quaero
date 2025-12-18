// -----------------------------------------------------------------------
// Last Modified: Tuesday, 14th October 2025 12:37:59 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package common

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/ternarybob/arbor"
	arborcommon "github.com/ternarybob/arbor/common"
	"github.com/ternarybob/arbor/models"
)

var (
	globalLogger arbor.ILogger
	loggerMutex  sync.RWMutex
)

// GetLogger returns the global logger instance
// If InitLogger() hasn't been called yet, returns a fallback console logger
func GetLogger() arbor.ILogger {
	loggerMutex.RLock()
	if globalLogger != nil {
		loggerMutex.RUnlock()
		return globalLogger
	}
	loggerMutex.RUnlock()

	loggerMutex.Lock()
	defer loggerMutex.Unlock()

	// Double-check after acquiring write lock
	if globalLogger == nil {
		// WARNING: Using fallback logger - InitLogger() should be called during startup
		globalLogger = arbor.NewLogger().WithConsoleWriter(createWriterConfig(nil, models.LogWriterTypeConsole, ""))
		// Log warning about initialization order issue
		globalLogger.Warn().Msg("Using fallback logger - InitLogger() should be called during startup")
	}
	return globalLogger
}

// InitLogger stores the provided logger as the global singleton instance
func InitLogger(logger arbor.ILogger) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	globalLogger = logger
}

// SetupLogger configures and initializes the global logger based on configuration
func SetupLogger(config *Config) arbor.ILogger {
	logger := arbor.NewLogger()

	// Get executable path for log directory
	execPath, err := os.Executable()
	if err != nil {
		// Add console writer first, then log the warning
		logger = logger.WithConsoleWriter(createWriterConfig(config, models.LogWriterTypeConsole, ""))
		logger.Warn().Err(err).Msg("Failed to get executable path - using fallback console logging")
	} else {
		execDir := filepath.Dir(execPath)
		logsDir := filepath.Join(execDir, "logs")

		// Check if file output is enabled
		hasFileOutput := false
		hasStdoutOutput := false
		for _, output := range config.Logging.Output {
			if output == "file" {
				hasFileOutput = true
			}
			if output == "stdout" || output == "console" {
				hasStdoutOutput = true
			}
		}

		// Configure file logging if enabled
		if hasFileOutput {
			if err := os.MkdirAll(logsDir, 0755); err != nil {
				// Use console writer temporarily for this warning
				tempLogger := logger.WithConsoleWriter(createWriterConfig(config, models.LogWriterTypeConsole, ""))
				tempLogger.Warn().Err(err).Str("logs_dir", logsDir).Msg("Failed to create logs directory")
			} else {
				logFile := filepath.Join(logsDir, "quaero.log")
				logger = logger.WithFileWriter(createWriterConfig(config, models.LogWriterTypeFile, logFile))
			}
		}

		// Configure console logging if enabled
		if hasStdoutOutput {
			logger = logger.WithConsoleWriter(createWriterConfig(config, models.LogWriterTypeConsole, ""))
		}

		// Ensure at least one visible log writer is configured
		if !hasFileOutput && !hasStdoutOutput {
			logger = logger.WithConsoleWriter(createWriterConfig(config, models.LogWriterTypeConsole, ""))
			logger.Warn().
				Strs("configured_outputs", config.Logging.Output).
				Msg("No visible log outputs configured - falling back to console")
		}
	}

	// Always add memory writer for WebSocket log streaming (GetRecentLogsHandler)
	logger = logger.WithMemoryWriter(createWriterConfig(config, models.LogWriterTypeMemory, ""))

	// Set log level
	logger = logger.WithLevelFromString(config.Logging.Level)

	// Store logger in singleton for global access
	InitLogger(logger)

	return logger
}

// createWriterConfig creates a standard writer configuration with user preferences
func createWriterConfig(config *Config, writerType models.LogWriterType, filename string) models.WriterConfiguration {
	// Default time format if not specified (HH:MM:SS.mmm for alignment with SSE logs)
	timeFormat := "15:04:05.000"
	if config != nil && config.Logging.TimeFormat != "" {
		timeFormat = config.Logging.TimeFormat
	}

	return models.WriterConfiguration{
		Type:             writerType,
		FileName:         filename,
		TimeFormat:       timeFormat,
		DisableTimestamp: false,
		MaxSize:          100 * 1024 * 1024, // 100 MB (only used for file writer)
		MaxBackups:       3,                 // (only used for file writer)
	}
}

// Stop flushes any remaining context logs before application shutdown
// Safe to call multiple times (Arbor's Stop is idempotent)
func Stop() {
	arborcommon.Stop()
}
