// -----------------------------------------------------------------------
// Last Modified: Tuesday, 14th October 2025 12:37:59 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package common

import (
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

// InitLogger stores the provided logger as the global singleton instance
func InitLogger(logger arbor.ILogger) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	globalLogger = logger
}

// Stop flushes any remaining context logs before application shutdown
// Safe to call multiple times (Arbor's Stop is idempotent)
func Stop() {
	arborcommon.Stop()
}
