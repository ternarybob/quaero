# Task 3: Refactor WebSocket Handler

Skill: go | Status: pending | Depends: task-2

## Objective
Integrate event aggregator into WebSocket handler. Replace direct step_progress broadcast with trigger-based approach.

## Changes

### File: `internal/handlers/websocket.go`

1. Add aggregator field to Handler struct:
```go
type Handler struct {
    // ... existing fields ...
    stepEventAggregator *events.StepEventAggregator
}
```

2. Initialize aggregator in NewHandler or subscription setup:
```go
// In SubscribeToCrawlerEvents or constructor
h.stepEventAggregator = events.NewStepEventAggregator(
    h.config.WebSocket.EventCountThreshold,
    timeThreshold, // parsed from config.WebSocket.TimeThreshold
    h.broadcastStepRefreshTrigger,
    h.logger,
)
h.stepEventAggregator.StartPeriodicFlush(ctx)
```

3. Add new broadcast method for refresh trigger:
```go
// broadcastStepRefreshTrigger sends a trigger to UI to fetch step events
func (h *Handler) broadcastStepRefreshTrigger(ctx context.Context, stepIDs []string) {
    msg := WSMessage{
        Type: "refresh_step_events",
        Payload: map[string]interface{}{
            "step_ids":  stepIDs,
            "timestamp": time.Now().Format(time.RFC3339),
        },
    }
    h.BroadcastMessage(msg)
}
```

4. Modify step_progress subscription (around line 1134-1179):
```go
h.eventService.Subscribe(interfaces.EventStepProgress, func(ctx context.Context, event interfaces.Event) error {
    payload, ok := event.Payload.(map[string]interface{})
    if !ok {
        return nil
    }

    // Extract step_id for aggregation
    stepID, _ := payload["step_id"].(string)
    if stepID == "" {
        return nil
    }

    // Record event in aggregator instead of broadcasting directly
    h.stepEventAggregator.RecordEvent(ctx, stepID)
    return nil
})
```

5. Keep manager_progress and job_step_progress subscriptions unchanged (they have lower volume and provide overall status).

## New WebSocket Message Type

```json
{
    "type": "refresh_step_events",
    "payload": {
        "step_ids": ["step-uuid-1", "step-uuid-2"],
        "timestamp": "2025-12-09T12:34:56Z"
    }
}
```

## Validation
- Build compiles successfully
- WebSocket connections still work
- New message type is broadcast when thresholds are reached
