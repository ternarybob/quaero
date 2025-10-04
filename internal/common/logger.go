package common

import (
	"github.com/ternarybob/arbor"
)

// InitLogger initializes the arbor logger with configuration
func InitLogger(config *Config) *arbor.Logger {
	logger := arbor.New()

	// Set log level
	switch config.Logging.Level {
	case "debug":
		logger.SetLevel(arbor.DebugLevel)
	case "info":
		logger.SetLevel(arbor.InfoLevel)
	case "warn":
		logger.SetLevel(arbor.WarnLevel)
	case "error":
		logger.SetLevel(arbor.ErrorLevel)
	default:
		logger.SetLevel(arbor.InfoLevel)
	}

	// Set output format
	if config.Logging.Format == "json" {
		logger.SetFormat(arbor.JSONFormat)
	} else {
		logger.SetFormat(arbor.TextFormat)
	}

	return logger
}
