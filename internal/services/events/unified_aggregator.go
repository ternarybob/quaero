package events

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ternarybob/arbor"
)

// LogRefreshTrigger represents a unified log refresh trigger
type LogRefreshTrigger struct {
	Scope     string   // "service" or "job"
	StepIDs   []string // Only for scope=job
	Finished  bool     // Only for scope=job (true when step completed)
	Timestamp time.Time
}

// UnifiedLogAggregator batches both service logs and step events, triggering UI refresh on a time interval.
// Instead of pushing each event, it triggers the UI to fetch the latest logs from the API.
// Triggers occur every timeThreshold (default 2 seconds) when there are pending events.
// For step events, also triggers immediately when a step finishes (completed/failed/cancelled).
type UnifiedLogAggregator struct {
	mu            sync.Mutex
	timeThreshold time.Duration

	// Service logs tracking (global boolean)
	hasServiceLogs     bool
	lastServiceTrigger time.Time

	// Step logs tracking (per-step)
	stepHasEvents   map[string]bool      // step_id -> has pending events
	stepLastTrigger map[string]time.Time // step_id -> last trigger time

	// Single callback for all triggers
	onTrigger func(ctx context.Context, trigger LogRefreshTrigger)

	logger arbor.ILogger
}

// NewUnifiedLogAggregator creates an aggregator with time-based triggering for both service and step logs
func NewUnifiedLogAggregator(
	timeThreshold time.Duration,
	onTrigger func(ctx context.Context, trigger LogRefreshTrigger),
	logger arbor.ILogger,
) *UnifiedLogAggregator {
	if timeThreshold <= 0 {
		timeThreshold = 10 * time.Second // Default 10 seconds to reduce WebSocket message frequency
	}

	return &UnifiedLogAggregator{
		timeThreshold:      timeThreshold,
		hasServiceLogs:     false,
		lastServiceTrigger: time.Now(),
		stepHasEvents:      make(map[string]bool),
		stepLastTrigger:    make(map[string]time.Time),
		onTrigger:          onTrigger,
		logger:             logger,
	}
}

// RecordServiceLog records that a new service log event has occurred
func (a *UnifiedLogAggregator) RecordServiceLog(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.hasServiceLogs = true

	if a.lastServiceTrigger.IsZero() {
		a.lastServiceTrigger = time.Now()
	}
}

// RecordStepEvent records that a step has new events (will be included in next periodic trigger)
func (a *UnifiedLogAggregator) RecordStepEvent(ctx context.Context, stepID string) {
	if stepID == "" {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.stepHasEvents[stepID] = true

	if _, exists := a.stepLastTrigger[stepID]; !exists {
		a.stepLastTrigger[stepID] = time.Now()
	}
}

// TriggerStepImmediately sends a refresh trigger for a step immediately (e.g., when step finishes)
// Uses debouncing to prevent excessive triggers - if a trigger was sent recently (within timeThreshold/2),
// the step is marked for the next periodic flush instead of sending immediately.
func (a *UnifiedLogAggregator) TriggerStepImmediately(ctx context.Context, stepID string) {
	if stepID == "" {
		return
	}

	a.mu.Lock()
	now := time.Now()
	lastTrigger := a.stepLastTrigger[stepID]
	minInterval := a.timeThreshold / 2 // Debounce: don't trigger more often than half the threshold

	// Check if we triggered this step recently - if so, mark for next periodic flush instead
	if !lastTrigger.IsZero() && now.Sub(lastTrigger) < minInterval {
		// Recent trigger exists - mark as having events and let periodic flush handle it
		a.stepHasEvents[stepID] = true
		a.mu.Unlock()
		return
	}

	a.stepHasEvents[stepID] = false
	a.stepLastTrigger[stepID] = now
	a.mu.Unlock()

	// NOTE: Don't log here - logging triggers log_event which can cause loops
	// Fire trigger with finished=true since step is complete
	a.onTrigger(ctx, LogRefreshTrigger{
		Scope:     "job",
		StepIDs:   []string{stepID},
		Finished:  true,
		Timestamp: now,
	})
}

// FlushAll triggers refresh for all pending events (used on shutdown/cleanup)
func (a *UnifiedLogAggregator) FlushAll(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()

	// Flush service logs
	if a.hasServiceLogs {
		a.hasServiceLogs = false
		a.lastServiceTrigger = now
		go a.safeOnTrigger(ctx, LogRefreshTrigger{
			Scope:     "service",
			Timestamp: now,
		})
	}

	// Flush step logs
	stepIDs := make([]string, 0, len(a.stepHasEvents))
	for stepID, hasEvents := range a.stepHasEvents {
		if hasEvents {
			stepIDs = append(stepIDs, stepID)
			a.stepHasEvents[stepID] = false
			a.stepLastTrigger[stepID] = now
		}
	}

	if len(stepIDs) > 0 {
		go a.safeOnTrigger(ctx, LogRefreshTrigger{
			Scope:     "job",
			StepIDs:   stepIDs,
			Finished:  false,
			Timestamp: now,
		})
	}
}

// safeOnTrigger wraps onTrigger with panic recovery to prevent crashes
func (a *UnifiedLogAggregator) safeOnTrigger(ctx context.Context, trigger LogRefreshTrigger) {
	defer func() {
		if r := recover(); r != nil {
			a.logger.Error().
				Str("panic", fmt.Sprintf("%v", r)).
				Str("scope", trigger.Scope).
				Msg("PANIC in UnifiedLogAggregator.onTrigger - recovered")
		}
	}()
	a.onTrigger(ctx, trigger)
}

// StartPeriodicFlush starts a background goroutine that triggers every timeThreshold
func (a *UnifiedLogAggregator) StartPeriodicFlush(ctx context.Context) {
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

// flushPending triggers refresh for all pending events
// NOTE: This function must NOT log anything - logging would trigger another log_event
// which would set hasServiceLogs=true, creating an infinite loop
func (a *UnifiedLogAggregator) flushPending(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()

	// Check service logs
	if a.hasServiceLogs {
		a.hasServiceLogs = false
		a.lastServiceTrigger = now
		go a.safeOnTrigger(ctx, LogRefreshTrigger{
			Scope:     "service",
			Timestamp: now,
		})
	}

	// Check step logs
	stepIDs := make([]string, 0)
	for stepID, hasEvents := range a.stepHasEvents {
		if !hasEvents {
			continue
		}
		stepIDs = append(stepIDs, stepID)
		a.stepHasEvents[stepID] = false
		a.stepLastTrigger[stepID] = now
	}

	if len(stepIDs) > 0 {
		go a.safeOnTrigger(ctx, LogRefreshTrigger{
			Scope:     "job",
			StepIDs:   stepIDs,
			Finished:  false,
			Timestamp: now,
		})
	}
}

// CleanupStep removes tracking data for a specific step
func (a *UnifiedLogAggregator) CleanupStep(stepID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.stepHasEvents, stepID)
	delete(a.stepLastTrigger, stepID)
}
