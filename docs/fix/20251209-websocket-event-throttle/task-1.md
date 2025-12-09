# Task 1: Add Event Aggregator Configuration

Skill: go | Status: pending

## Objective
Add configurable thresholds for the event aggregator to `WebSocketConfig`.

## Changes

### File: `internal/common/config.go`

1. Add new fields to `WebSocketConfig` struct (around line 132):
```go
type WebSocketConfig struct {
    MinLevel          string            `toml:"min_level"`
    ExcludePatterns   []string          `toml:"exclude_patterns"`
    AllowedEvents     []string          `toml:"allowed_events"`
    ThrottleIntervals map[string]string `toml:"throttle_intervals"`
    // Event aggregator settings for trigger-based UI updates
    EventCountThreshold int    `toml:"event_count_threshold"` // Trigger refresh after N events (default: 100)
    TimeThreshold       string `toml:"time_threshold"`        // Trigger refresh after duration (default: "1s")
}
```

2. Add defaults in `NewDefaultConfig()` (around line 239):
```go
WebSocket: WebSocketConfig{
    // ... existing fields ...
    EventCountThreshold: 100,   // Trigger UI refresh after 100 step events
    TimeThreshold:       "1s",  // Or after 1 second, whichever comes first
},
```

3. Add env var support in `applyEnvOverrides()`:
```go
if eventCountThreshold := os.Getenv("QUAERO_WEBSOCKET_EVENT_COUNT_THRESHOLD"); eventCountThreshold != "" {
    if ect, err := strconv.Atoi(eventCountThreshold); err == nil {
        config.WebSocket.EventCountThreshold = ect
    }
}
if timeThreshold := os.Getenv("QUAERO_WEBSOCKET_TIME_THRESHOLD"); timeThreshold != "" {
    if _, err := time.ParseDuration(timeThreshold); err == nil {
        config.WebSocket.TimeThreshold = timeThreshold
    }
}
```

4. Add to `DeepCloneConfig()` if needed (primitive types auto-copy, no action needed)

## Validation
- Build compiles successfully
- Existing tests pass
