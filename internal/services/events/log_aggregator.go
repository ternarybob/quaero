package events

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ternarybob/arbor"
)

// LogEventAggregator batches log events and triggers UI refresh on a time interval.
// Instead of pushing each log event, it triggers the UI to fetch the latest logs from the API.
// Triggers occur every timeThreshold (default 1 second) when there are pending logs.
type LogEventAggregator struct {
	mu            sync.Mutex
	timeThreshold time.Duration

	// Track if there are pending logs (boolean, not individual logs)
	hasPendingLogs bool
	lastTrigger    time.Time

	// Callback to send WebSocket trigger (refresh_logs message)
	onTrigger func(ctx context.Context)

	logger arbor.ILogger
}

// NewLogEventAggregator creates an aggregator with time-based triggering
func NewLogEventAggregator(
	timeThreshold time.Duration,
	onTrigger func(ctx context.Context),
	logger arbor.ILogger,
) *LogEventAggregator {
	if timeThreshold <= 0 {
		timeThreshold = time.Second // Default 1 second
	}

	return &LogEventAggregator{
		timeThreshold:  timeThreshold,
		hasPendingLogs: false,
		lastTrigger:    time.Now(),
		onTrigger:      onTrigger,
		logger:         logger,
	}
}

// RecordEvent records that a new log event has occurred
func (a *LogEventAggregator) RecordEvent(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.hasPendingLogs = true

	// Initialize last trigger time if not set
	if a.lastTrigger.IsZero() {
		a.lastTrigger = time.Now()
	}
}

// FlushAll triggers refresh if there are pending logs (used on shutdown/cleanup)
func (a *LogEventAggregator) FlushAll(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.hasPendingLogs {
		a.logger.Debug().Msg("Log event aggregator flushing pending logs")
		a.hasPendingLogs = false
		a.lastTrigger = time.Now()
		go a.safeOnTrigger(ctx)
	}
}

// safeOnTrigger wraps onTrigger with panic recovery to prevent crashes
func (a *LogEventAggregator) safeOnTrigger(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			a.logger.Error().
				Str("panic", fmt.Sprintf("%v", r)).
				Msg("PANIC in LogEventAggregator.onTrigger - recovered")
		}
	}()
	a.onTrigger(ctx)
}

// StartPeriodicFlush starts a background goroutine that triggers every timeThreshold
func (a *LogEventAggregator) StartPeriodicFlush(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(a.timeThreshold)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				// Flush remaining events on shutdown
				a.FlushAll(context.Background())
				return
			case <-ticker.C:
				a.flushPending(ctx)
			}
		}
	}()
}

// flushPending triggers refresh if there are pending logs
func (a *LogEventAggregator) flushPending(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.hasPendingLogs {
		return
	}

	now := time.Now()
	a.hasPendingLogs = false
	a.lastTrigger = now

	a.logger.Debug().Msg("Log event aggregator: periodic trigger")
	go a.safeOnTrigger(ctx)
}
