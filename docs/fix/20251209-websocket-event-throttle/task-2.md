# Task 2: Create Event Aggregator Service

Skill: go | Status: pending | Depends: task-1

## Objective
Create an in-memory event aggregator that batches step events and triggers UI refresh.

## New File: `internal/services/events/aggregator.go`

```go
package events

import (
    "context"
    "sync"
    "time"

    "github.com/ternarybob/arbor"
    "github.com/ternarybob/quaero/internal/interfaces"
)

// StepEventAggregator batches step events and triggers UI refresh
// when count threshold or time threshold is reached.
type StepEventAggregator struct {
    mu                  sync.Mutex
    eventCountThreshold int
    timeThreshold       time.Duration

    // Per-step tracking
    stepCounts    map[string]int       // step_id -> event count
    stepLastEvent map[string]time.Time // step_id -> last event time
    stepLastTrigger map[string]time.Time // step_id -> last trigger time

    // Callback to send WebSocket trigger
    onTrigger func(ctx context.Context, stepIDs []string)

    logger *arbor.Logger
}

// NewStepEventAggregator creates an aggregator with configurable thresholds
func NewStepEventAggregator(
    eventCountThreshold int,
    timeThreshold time.Duration,
    onTrigger func(ctx context.Context, stepIDs []string),
    logger *arbor.Logger,
) *StepEventAggregator {
    return &StepEventAggregator{
        eventCountThreshold: eventCountThreshold,
        timeThreshold:       timeThreshold,
        stepCounts:          make(map[string]int),
        stepLastEvent:       make(map[string]time.Time),
        stepLastTrigger:     make(map[string]time.Time),
        onTrigger:           onTrigger,
        logger:              logger,
    }
}

// RecordEvent records a step event and checks if trigger threshold is reached
func (a *StepEventAggregator) RecordEvent(ctx context.Context, stepID string) {
    a.mu.Lock()
    defer a.mu.Unlock()

    now := time.Now()
    a.stepCounts[stepID]++
    a.stepLastEvent[stepID] = now

    // Check thresholds
    count := a.stepCounts[stepID]
    lastTrigger := a.stepLastTrigger[stepID]
    timeSinceLastTrigger := now.Sub(lastTrigger)

    shouldTrigger := count >= a.eventCountThreshold || timeSinceLastTrigger >= a.timeThreshold

    if shouldTrigger {
        // Reset and trigger
        a.stepCounts[stepID] = 0
        a.stepLastTrigger[stepID] = now

        // Fire trigger in goroutine to not block
        go a.onTrigger(ctx, []string{stepID})
    }
}

// FlushAll triggers refresh for all pending steps (used on shutdown/cleanup)
func (a *StepEventAggregator) FlushAll(ctx context.Context) {
    a.mu.Lock()
    defer a.mu.Unlock()

    stepIDs := make([]string, 0, len(a.stepCounts))
    for stepID, count := range a.stepCounts {
        if count > 0 {
            stepIDs = append(stepIDs, stepID)
            a.stepCounts[stepID] = 0
            a.stepLastTrigger[stepID] = time.Now()
        }
    }

    if len(stepIDs) > 0 {
        go a.onTrigger(ctx, stepIDs)
    }
}

// StartPeriodicFlush starts a background goroutine that flushes stale events
func (a *StepEventAggregator) StartPeriodicFlush(ctx context.Context) {
    go func() {
        ticker := time.NewTicker(a.timeThreshold)
        defer ticker.Stop()

        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                a.flushStale(ctx)
            }
        }
    }()
}

// flushStale triggers refresh for steps with pending events past time threshold
func (a *StepEventAggregator) flushStale(ctx context.Context) {
    a.mu.Lock()
    defer a.mu.Unlock()

    now := time.Now()
    stepIDs := make([]string, 0)

    for stepID, count := range a.stepCounts {
        if count == 0 {
            continue
        }

        lastTrigger := a.stepLastTrigger[stepID]
        if now.Sub(lastTrigger) >= a.timeThreshold {
            stepIDs = append(stepIDs, stepID)
            a.stepCounts[stepID] = 0
            a.stepLastTrigger[stepID] = now
        }
    }

    if len(stepIDs) > 0 {
        go a.onTrigger(ctx, stepIDs)
    }
}
```

## Integration Point
The aggregator will be instantiated in WebSocketHandler and used when step_progress events are received.

## Validation
- Build compiles successfully
- Unit tests for aggregator logic
