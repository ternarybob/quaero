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

// UnifiedLogAggregator batches both service logs and step events using periodic flushing.
// Instead of pushing each event, it triggers the UI to fetch the latest logs from the API.
//
// Triggering Strategy:
// - Events are batched and triggers fire from periodic flush (every minThreshold)
// - Immediate trigger when a step finishes (completed/failed/cancelled) - no debounce
// - Step completions are critical for showing final logs, so they always fire
//
// This prevents UI flooding while ensuring step completion triggers always reach the UI.
type UnifiedLogAggregator struct {
	mu            sync.Mutex
	minThreshold  time.Duration // Minimum time between periodic triggers
	maxThreshold  time.Duration // Maximum time between triggers (for rate-adaptive)
	serviceThreshold time.Duration // Service log trigger interval
	rateThreshold int           // Logs per second considered "high rate"

	// Service logs tracking with rate monitoring
	hasServiceLogs         bool
	serviceLogCount        int       // Logs since last trigger
	serviceRateWindowStart time.Time // Start of current rate measurement window
	lastServiceTrigger     time.Time

	// Step logs tracking with rate monitoring (per-step)
	stepHasEvents       map[string]bool      // step_id -> has pending events
	stepLogCount        map[string]int       // step_id -> logs since last trigger
	stepRateWindowStart map[string]time.Time // step_id -> rate window start
	stepLastTrigger     map[string]time.Time // step_id -> last trigger time
	stepFinished        map[string]bool      // step_id -> terminal refresh already sent

	// Single callback for all triggers
	onTrigger func(ctx context.Context, trigger LogRefreshTrigger)

	logger arbor.ILogger
}

// NewUnifiedLogAggregator creates an aggregator with rate-adaptive triggering.
// The timeThreshold parameter is used as the periodic trigger interval (default: 10 seconds).
func NewUnifiedLogAggregator(
	timeThreshold time.Duration,
	onTrigger func(ctx context.Context, trigger LogRefreshTrigger),
	logger arbor.ILogger,
) *UnifiedLogAggregator {
	// Default to 10 seconds to reduce WebSocket message frequency while preserving progressive updates.
	if timeThreshold <= 0 {
		timeThreshold = 10 * time.Second
	}

	return &UnifiedLogAggregator{
		minThreshold:           timeThreshold, // Periodic trigger interval for progressive updates
		maxThreshold:           timeThreshold, // Max wait for high-rate bursts
		serviceThreshold:       2 * timeThreshold,
		rateThreshold:          100,             // Logs/sec considered "high rate" (most batches are high-rate)
		hasServiceLogs:         false,
		serviceLogCount:        0,
		serviceRateWindowStart: time.Time{},
		lastServiceTrigger:     time.Now(),
		stepHasEvents:          make(map[string]bool),
		stepLogCount:           make(map[string]int),
		stepRateWindowStart:    make(map[string]time.Time),
		stepLastTrigger:        make(map[string]time.Time),
		stepFinished:           make(map[string]bool),
		onTrigger:              onTrigger,
		logger:                 logger,
	}
}

// RecordServiceLog records that a new service log event has occurred.
// This only marks events as pending - actual triggering happens via periodic flush.
func (a *UnifiedLogAggregator) RecordServiceLog(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()

	a.hasServiceLogs = true
	a.serviceLogCount++

	// Initialize rate window if needed
	if a.serviceRateWindowStart.IsZero() {
		a.serviceRateWindowStart = now
	}
	if a.lastServiceTrigger.IsZero() {
		a.lastServiceTrigger = now
	}
	// Note: Actual triggering handled by flushPending() - no direct trigger here
}

// RecordStepEvent records that a step has new events.
// This only marks events as pending - actual triggering happens via periodic flush.
func (a *UnifiedLogAggregator) RecordStepEvent(ctx context.Context, stepID string) {
	if stepID == "" {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()

	a.stepHasEvents[stepID] = true
	a.stepLogCount[stepID]++

	// Initialize rate window if needed
	if _, exists := a.stepRateWindowStart[stepID]; !exists {
		a.stepRateWindowStart[stepID] = now
	}
	if _, exists := a.stepLastTrigger[stepID]; !exists {
		a.stepLastTrigger[stepID] = now
	}
	// Note: Actual triggering handled by flushPending() - no direct trigger here
}

// TriggerStepImmediately sends a refresh trigger for a step immediately (e.g., when step finishes)
// Step completions always trigger immediately - this is critical for showing final logs.
// No debouncing here because missing a step completion means the UI never shows its logs.
func (a *UnifiedLogAggregator) TriggerStepImmediately(ctx context.Context, stepID string) {
	if stepID == "" {
		return
	}

	a.mu.Lock()
	if a.stepFinished[stepID] {
		a.mu.Unlock()
		return
	}
	a.stepFinished[stepID] = true
	now := time.Now()
	// Always clear state and trigger - step completions are too important to debounce
	a.stepHasEvents[stepID] = false
	a.stepLogCount[stepID] = 0
	a.stepRateWindowStart[stepID] = now
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
		a.serviceLogCount = 0
		a.serviceRateWindowStart = now
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
			a.stepLogCount[stepID] = 0
			a.stepRateWindowStart[stepID] = now
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

// StartPeriodicFlush starts a background goroutine that triggers periodically.
// flushPending enforces thresholds; the check interval can be more frequent.
func (a *UnifiedLogAggregator) StartPeriodicFlush(ctx context.Context) {
	go func() {
		// Check frequently; flushPending enforces thresholds.
		ticker := time.NewTicker(1 * time.Second)
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

// flushPending triggers refresh for pending events based on rate-adaptive logic.
// Low rate producers trigger quickly, high rate producers wait longer.
// NOTE: This function must NOT log anything - logging would trigger another log_event
// which would set hasServiceLogs=true, creating an infinite loop
func (a *UnifiedLogAggregator) flushPending(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()

	// Check service logs with rate-adaptive timing
	if a.hasServiceLogs {
		timeSinceLastTrigger := now.Sub(a.lastServiceTrigger)
		shouldFlush := false

		if timeSinceLastTrigger >= a.serviceThreshold {
			shouldFlush = true
		}

		if shouldFlush {
			a.hasServiceLogs = false
			a.serviceLogCount = 0
			a.serviceRateWindowStart = now
			a.lastServiceTrigger = now
			go a.safeOnTrigger(ctx, LogRefreshTrigger{
				Scope:     "service",
				Timestamp: now,
			})
		}
	}

	// Check step logs with rate-adaptive timing
	stepIDs := make([]string, 0)
	for stepID, hasEvents := range a.stepHasEvents {
		if !hasEvents {
			continue
		}

		// Calculate rate for this step
		windowStart := a.stepRateWindowStart[stepID]
		windowDuration := now.Sub(windowStart)
		if windowDuration < 100*time.Millisecond {
			windowDuration = 100 * time.Millisecond
		}
		currentRate := float64(a.stepLogCount[stepID]) / windowDuration.Seconds()

		// Determine if we should flush based on rate
		timeSinceLastTrigger := now.Sub(a.stepLastTrigger[stepID])
		shouldFlush := false

		if currentRate < float64(a.rateThreshold) {
			// Low rate: flush after minThreshold
			if timeSinceLastTrigger >= a.minThreshold {
				shouldFlush = true
			}
		} else {
			// High rate: wait up to maxThreshold
			if timeSinceLastTrigger >= a.maxThreshold {
				shouldFlush = true
			}
		}

		if shouldFlush {
			stepIDs = append(stepIDs, stepID)
			a.stepHasEvents[stepID] = false
			a.stepLogCount[stepID] = 0
			a.stepRateWindowStart[stepID] = now
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

// CleanupStep removes tracking data for a specific step
func (a *UnifiedLogAggregator) CleanupStep(stepID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.stepHasEvents, stepID)
	delete(a.stepLogCount, stepID)
	delete(a.stepRateWindowStart, stepID)
	delete(a.stepLastTrigger, stepID)
	delete(a.stepFinished, stepID)
}
