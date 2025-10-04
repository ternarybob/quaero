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
		globalLogger = arbor.NewLogger().WithConsoleWriter(models.WriterConfiguration{
			Type:             models.LogWriterTypeConsole,
			TimeFormat:       "15:04:05",
			TextOutput:       true,
			DisableTimestamp: false,
		})
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

	// Set log level
	logger = logger.WithLevelFromString(config.Logging.Level)

	// Store as global logger
	globalLogger = logger

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
