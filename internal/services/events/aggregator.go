package events

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ternarybob/arbor"
)

// StepEventAggregator batches step events and triggers UI refresh on a time interval.
// Instead of pushing each event, it triggers the UI to fetch the latest events from the API.
// Triggers occur:
// - Every timeThreshold (default 1 second) for steps with pending events
// - Immediately when a step finishes (completed/failed/cancelled)
type StepEventAggregator struct {
	mu            sync.Mutex
	timeThreshold time.Duration

	// Per-step tracking
	stepHasEvents   map[string]bool      // step_id -> has pending events
	stepLastTrigger map[string]time.Time // step_id -> last trigger time

	// Callback to send WebSocket trigger (stepIDs, finished flag)
	onTrigger func(ctx context.Context, stepIDs []string, finished bool)

	logger arbor.ILogger
}

// NewStepEventAggregator creates an aggregator with time-based triggering
func NewStepEventAggregator(
	timeThreshold time.Duration,
	onTrigger func(ctx context.Context, stepIDs []string, finished bool),
	logger arbor.ILogger,
) *StepEventAggregator {
	if timeThreshold <= 0 {
		timeThreshold = time.Second // Default 1 second
	}

	return &StepEventAggregator{
		timeThreshold:   timeThreshold,
		stepHasEvents:   make(map[string]bool),
		stepLastTrigger: make(map[string]time.Time),
		onTrigger:       onTrigger,
		logger:          logger,
	}
}

// RecordEvent records that a step has new events (will be included in next periodic trigger)
func (a *StepEventAggregator) RecordEvent(ctx context.Context, stepID string) {
	if stepID == "" {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.stepHasEvents[stepID] = true

	// Initialize last trigger time if not set
	if _, exists := a.stepLastTrigger[stepID]; !exists {
		a.stepLastTrigger[stepID] = time.Now()
	}
}

// TriggerImmediately sends a refresh trigger for a step immediately (e.g., when step finishes)
func (a *StepEventAggregator) TriggerImmediately(ctx context.Context, stepID string) {
	if stepID == "" {
		return
	}

	a.mu.Lock()
	// Reset tracking for this step
	a.stepHasEvents[stepID] = false
	a.stepLastTrigger[stepID] = time.Now()
	a.mu.Unlock()

	a.logger.Debug().
		Str("step_id", stepID).
		Msg("Step event aggregator: immediate trigger (step finished)")

	// Fire trigger with finished=true since step is complete
	a.onTrigger(ctx, []string{stepID}, true)
}

// FlushAll triggers refresh for all pending steps (used on shutdown/cleanup)
func (a *StepEventAggregator) FlushAll(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	stepIDs := make([]string, 0, len(a.stepHasEvents))
	for stepID, hasEvents := range a.stepHasEvents {
		if hasEvents {
			stepIDs = append(stepIDs, stepID)
			a.stepHasEvents[stepID] = false
			a.stepLastTrigger[stepID] = time.Now()
		}
	}

	if len(stepIDs) > 0 {
		a.logger.Debug().
			Int("step_count", len(stepIDs)).
			Msg("Step event aggregator flushing all pending events")
		go a.safeOnTrigger(ctx, stepIDs, false)
	}
}

// safeOnTrigger wraps onTrigger with panic recovery to prevent crashes
func (a *StepEventAggregator) safeOnTrigger(ctx context.Context, stepIDs []string, finished bool) {
	defer func() {
		if r := recover(); r != nil {
			a.logger.Error().
				Str("panic", fmt.Sprintf("%v", r)).
				Int("step_count", len(stepIDs)).
				Bool("finished", finished).
				Msg("PANIC in StepEventAggregator.onTrigger - recovered")
		}
	}()
	a.onTrigger(ctx, stepIDs, finished)
}

// StartPeriodicFlush starts a background goroutine that triggers every timeThreshold
func (a *StepEventAggregator) StartPeriodicFlush(ctx context.Context) {
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

// flushPending triggers refresh for all steps with pending events
func (a *StepEventAggregator) flushPending(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
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
		a.logger.Debug().
			Int("step_count", len(stepIDs)).
			Msg("Step event aggregator: periodic trigger")
		go a.safeOnTrigger(ctx, stepIDs, false)
	}
}

// Cleanup removes tracking data for a specific step
func (a *StepEventAggregator) Cleanup(stepID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.stepHasEvents, stepID)
	delete(a.stepLastTrigger, stepID)
}
