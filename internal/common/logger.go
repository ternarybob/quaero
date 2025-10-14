package common

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/ternarybob/arbor"
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
		globalLogger = arbor.NewLogger().WithConsoleWriter(models.WriterConfiguration{
			Type:             models.LogWriterTypeConsole,
			TimeFormat:       "15:04:05",
			TextOutput:       true,
			DisableTimestamp: false,
		})
		// Log warning about initialization order issue
		globalLogger.Warn().Msg("Using fallback logger - InitLogger() should be called during startup")
	}
	return globalLogger
}

// InitLogger initializes the arbor logger with configuration
func InitLogger(config *Config) arbor.ILogger {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()

	logger := arbor.NewLogger()

	// Get executable path for log directory
	execPath, err := os.Executable()
	if err != nil {
		fmt.Printf("Warning: Failed to get executable path: %v\n", err)
		return logger.WithConsoleWriter(models.WriterConfiguration{
			Type:             models.LogWriterTypeConsole,
			TimeFormat:       "15:04:05",
			TextOutput:       true,
			DisableTimestamp: false,
		})
	}
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
			fmt.Printf("Warning: Failed to create logs directory: %v\n", err)
		} else {
			logFile := filepath.Join(logsDir, "quaero.log")
			logger = logger.WithFileWriter(models.WriterConfiguration{
				Type:             models.LogWriterTypeFile,
				FileName:         logFile,
				TimeFormat:       "15:04:05",
				MaxSize:          100 * 1024 * 1024, // 100 MB
				MaxBackups:       3,
				TextOutput:       true,
				DisableTimestamp: false,
			})
		}
	}

	// Configure console logging if enabled
	if hasStdoutOutput {
		logger = logger.WithConsoleWriter(models.WriterConfiguration{
			Type:             models.LogWriterTypeConsole,
			TimeFormat:       "15:04:05",
			TextOutput:       true,
			DisableTimestamp: false,
		})
	}

	// Ensure at least one visible log writer is configured
	if !hasFileOutput && !hasStdoutOutput {
		// Force-enable console writer to guarantee visible logs
		logger = logger.WithConsoleWriter(models.WriterConfiguration{
			Type:             models.LogWriterTypeConsole,
			TimeFormat:       "15:04:05",
			TextOutput:       true,
			DisableTimestamp: false,
		})
		// Note: We can't log the warning yet because the logger isn't fully configured
		// The warning will be logged after level is set
		fmt.Printf("Warning: No visible log outputs configured, falling back to console\n")
	}

	// Always add memory writer for WebSocket log streaming
	// This enables real-time log display in the web UI
	logger = logger.WithMemoryWriter(models.WriterConfiguration{
		Type:             models.LogWriterTypeMemory,
		TimeFormat:       "15:04:05",
		TextOutput:       true,
		DisableTimestamp: false,
	})

	// Set log level
	logger = logger.WithLevelFromString(config.Logging.Level)

	// Store as global logger
	globalLogger = logger

	// Log warning if we fell back to console due to misconfiguration
	if !hasFileOutput && !hasStdoutOutput {
		logger.Warn().Msg("No visible log outputs configured, falling back to console")
	}

	return logger
}

// GetLogFilePath returns the configured log file path from the logger
func GetLogFilePath(logger arbor.ILogger) string {
	if logger != nil {
		if logFilePath := logger.GetLogFilePath(); logFilePath != "" {
			return logFilePath
		}
	}
	return ""
}
