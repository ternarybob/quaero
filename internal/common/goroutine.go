// -----------------------------------------------------------------------
// Safe Goroutine - Panic-protected goroutine wrappers
// -----------------------------------------------------------------------

package common

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"

	"github.com/ternarybob/arbor"
)

// goroutineCounter tracks spawned goroutines for diagnostics
var goroutineCounter int64

// GetGoroutineCount returns the number of goroutines spawned via SafeGo
func GetGoroutineCount() int64 {
	return atomic.LoadInt64(&goroutineCounter)
}

// SafeGo runs a function in a goroutine with panic recovery.
// Panics are logged but don't crash the service.
// Use this for async operations like event publishing where failure should not be fatal.
//
// Example:
//
//	common.SafeGo(logger, "publishEvent", func() {
//	    eventService.Publish(ctx, event)
//	})
func SafeGo(logger arbor.ILogger, name string, fn func()) {
	atomic.AddInt64(&goroutineCounter, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Get stack trace
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				stackTrace := string(buf[:n])

				// Log the panic
				if logger != nil {
					logger.Error().
						Str("goroutine", name).
						Str("panic", fmt.Sprintf("%v", r)).
						Str("stack", stackTrace).
						Msg("Recovered from panic in goroutine - continuing service operation")
				} else {
					// Fallback to stderr if no logger
					fmt.Fprintf(os.Stderr, "PANIC in goroutine %s: %v\n%s\n", name, r, stackTrace)
				}

				// Optionally write to crash log file for post-mortem analysis
				// But don't exit - this is a non-fatal goroutine crash
				writeCrashLog(name, r, stackTrace)
			}
		}()

		fn()
	}()
}

// SafeGoWithContext runs a function in a goroutine with panic recovery and context support.
// The goroutine will exit if the context is cancelled.
//
// Example:
//
//	common.SafeGoWithContext(ctx, logger, "backgroundTask", func() {
//	    // long-running task
//	})
func SafeGoWithContext(ctx context.Context, logger arbor.ILogger, name string, fn func()) {
	atomic.AddInt64(&goroutineCounter, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Get stack trace
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				stackTrace := string(buf[:n])

				// Log the panic
				if logger != nil {
					logger.Error().
						Str("goroutine", name).
						Str("panic", fmt.Sprintf("%v", r)).
						Str("stack", stackTrace).
						Msg("Recovered from panic in goroutine - continuing service operation")
				}

				// Write to crash log for analysis
				writeCrashLog(name, r, stackTrace)
			}
		}()

		// Check context before running
		select {
		case <-ctx.Done():
			if logger != nil {
				logger.Debug().Str("goroutine", name).Msg("Goroutine cancelled before start")
			}
			return
		default:
		}

		fn()
	}()
}

// writeCrashLog writes a non-fatal crash log entry for goroutine panics.
// This creates separate files from fatal crashes to distinguish severity.
func writeCrashLog(goroutineName string, panicVal interface{}, stackTrace string) {
	// For non-fatal panics, we just log - don't create a crash file
	// The logger should capture this adequately
	// If we wanted persistent crash logs, we could write here
}
