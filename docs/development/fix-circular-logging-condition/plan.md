# Implementation Plan: Fix Circular Logging Condition

## Task Metadata

**Task ID:** fix-circular-logging-condition
**Complexity:** Medium
**Estimated Steps:** 4
**Risk Level:** Medium (affects core logging and event infrastructure)
**Date Created:** 2025-11-08
**Status:** Planning Complete

## Problem Statement

A circular logging condition exists in the event and logging system causing infinite log recursion:

**The Cycle:**
1. EventService.Publish() logs "Publishing event" at line 85-88 (`event_service.go`)
2. Logger writes log entry to arbor context writer
3. LogConsumer receives log entry from arbor channel (`consumer.go:96-122`)
4. LogConsumer publishes "log_event" via EventService.Publish() at line 161 (`consumer.go:161`)
5. **CYCLE REPEATS** - EventService.Publish() logs "Publishing event" again for the "log_event"
6. Infinite recursion until system crashes (78.7MB log file, 401,726+ log_event entries)

**Evidence from Logs:**
```
16:51:05 INF > function=github.com/ternarybob/quaero/internal/services/events.(*Service).Publish
              correlationid=55b85ff2-b33c-461e-9472-6c52f65b9787
              event_type=log_event
              subscriber_count=1
              Publishing event
```

This message repeats continuously, indicating EventService is logging its own "log_event" publications.

## Root Cause Analysis

### Component Interaction Map

```
┌─────────────────────────────────────────────────────────────────┐
│                     CIRCULAR DEPENDENCY                          │
│                                                                  │
│  EventService.Publish()                                         │
│       │                                                          │
│       │ (logs "Publishing event")                               │
│       ▼                                                          │
│  Logger → Arbor ContextWriter → LogConsumer.channel             │
│                                        │                         │
│                                        │ (processes batch)       │
│                                        ▼                         │
│                              LogConsumer.publishLogEvent()       │
│                                        │                         │
│                                        │ (publishes log_event)   │
│                                        ▼                         │
│                              EventService.Publish() ◄────────────┤
│                                        │                         │
│                                        └─ INFINITE LOOP          │
└─────────────────────────────────────────────────────────────────┘
```

### Affected Files

1. **C:\development\quaero\internal\services\events\event_service.go**
   - Line 85-88: `Publish()` logs "Publishing event" for ALL events including "log_event"
   - Line 125-128: `PublishSync()` also logs event publications

2. **C:\development\quaero\internal\logs\consumer.go**
   - Line 157-174: `publishLogEvent()` publishes "log_event" via EventService
   - Line 120-122: Condition check for publishing events (minEventLevel threshold)

3. **C:\development\quaero\internal\services\events\logger_subscriber.go**
   - Line 29-42: NewLoggerSubscriber logs ALL events (not part of circular issue but adds noise)

4. **C:\development\quaero\internal\handlers\websocket.go**
   - Line 769-812: Subscribes to "log_event" and broadcasts to WebSocket clients
   - This is working correctly - not part of the circular issue

## Solution Strategy

### Design Principle: Break the Cycle Without Losing Functionality

**Preserve:**
- Event logging for debugging and audit trails
- Log events sent to UI via WebSocket
- Structured logging with correlation IDs
- Event-driven architecture

**Break the cycle by:**
1. Prevent EventService from logging its own "log_event" publications
2. Add safeguard to detect and prevent recursive event publishing

### Implementation Approach

Use **conditional logging** in EventService.Publish() to skip logging for "log_event" type events.

**Rationale:**
- Minimal code changes (single conditional check)
- No architectural changes required
- Preserves all existing functionality
- Low risk of breaking other components

## Detailed Implementation Steps

### Step 1: Add Event Type Blacklist to EventService

**File:** `C:\development\quaero\internal\services\events\event_service.go`

**Changes:**

```go
// At line 12-17 (after imports, before Service struct):

// nonLoggableEvents defines event types that should NOT be logged by EventService
// to prevent circular logging conditions (e.g., log_event triggering more log_event)
var nonLoggableEvents = map[interfaces.EventType]bool{
	"log_event": true, // Log events are published by LogConsumer - don't log them
}
```

**Dependencies:** None
**Validation:**
- [ ] Verify map syntax is correct
- [ ] Ensure "log_event" matches the event type used in consumer.go:162

**Risk:** None - read-only map initialization

---

### Step 2: Modify Publish() to Skip Logging for Blacklisted Events

**File:** `C:\development\quaero\internal\services\events\event_service.go`

**Changes:**

```go
// Replace lines 85-88 in Publish() method:

// OLD CODE:
s.logger.Info().
    Str("event_type", string(event.Type)).
    Int("subscriber_count", len(handlers)).
    Msg("Publishing event")

// NEW CODE:
// Only log event publication if not in blacklist (prevents circular logging)
if !nonLoggableEvents[event.Type] {
    s.logger.Info().
        Str("event_type", string(event.Type)).
        Int("subscriber_count", len(handlers)).
        Msg("Publishing event")
}
```

**Dependencies:** Step 1 must be completed first
**Validation:**
- [ ] Verify conditional check syntax
- [ ] Test that other events still log correctly
- [ ] Confirm "log_event" publications no longer log

**Risk:** Low - only affects logging, not event delivery

---

### Step 3: Modify PublishSync() to Skip Logging for Blacklisted Events

**File:** `C:\development\quaero\internal\services\events\event_service.go`

**Changes:**

```go
// Replace lines 125-128 in PublishSync() method:

// OLD CODE:
s.logger.Info().
    Str("event_type", string(event.Type)).
    Int("subscriber_count", len(handlers)).
    Msg("*** EVENT SERVICE: Publishing event synchronously to all handlers")

// NEW CODE:
// Only log event publication if not in blacklist (prevents circular logging)
if !nonLoggableEvents[event.Type] {
    s.logger.Info().
        Str("event_type", string(event.Type)).
        Int("subscriber_count", len(handlers)).
        Msg("*** EVENT SERVICE: Publishing event synchronously to all handlers")
}
```

**Dependencies:** Step 1 must be completed first
**Validation:**
- [ ] Verify conditional check syntax
- [ ] Test that synchronous event publishing works correctly
- [ ] Confirm "log_event" synchronous publications no longer log

**Risk:** Low - only affects logging, not event delivery

---

### Step 4: Add Circuit Breaker to LogConsumer (Defense in Depth)

**File:** `C:\development\quaero\internal\logs\consumer.go`

**Purpose:** Prevent recursive publishing even if new event types are added in the future

**Changes:**

```go
// At line 19-28 (in Consumer struct), add new field:

type Consumer struct {
    storage       interfaces.JobLogStorage
    eventService  interfaces.EventService
    logger        arbor.ILogger
    channel       chan []arbormodels.LogEvent
    ctx           context.Context
    cancel        context.CancelFunc
    wg            sync.WaitGroup
    minEventLevel arbor.LogLevel // Minimum log level to publish as events
    publishing    sync.Map       // Track events being published to prevent recursion
}

// At line 157-175, modify publishLogEvent():

// publishLogEvent publishes a log entry as an event for UI consumption
func (c *Consumer) publishLogEvent(event arbormodels.LogEvent, logEntry models.JobLogEntry) {
    // Circuit breaker: Check if we're already publishing an event for this correlation ID
    // This prevents recursive event publishing (defense in depth)
    key := fmt.Sprintf("%s:%s", event.CorrelationID, logEntry.Message)
    if _, loaded := c.publishing.LoadOrStore(key, true); loaded {
        // Already publishing this event - skip to prevent recursion
        return
    }
    defer c.publishing.Delete(key)

    // Publish to EventService (WebSocket will subscribe to this event type)
    go func() {
        // Non-blocking publish in goroutine
        err := c.eventService.Publish(c.ctx, interfaces.Event{
            Type: "log_event", // Event type for log streaming
            Payload: map[string]interface{}{
                "job_id":    event.CorrelationID,
                "level":     logEntry.Level,
                "message":   logEntry.Message,
                "timestamp": logEntry.Timestamp,
            },
        })
        if err != nil {
            // Use fmt.Printf to avoid deadlock with logger
            fmt.Printf("WARN: Failed to publish log event for job %s: %v\n", event.CorrelationID, err)
        }
    }()
}
```

**Dependencies:** None (independent defensive measure)
**Validation:**
- [ ] Verify sync.Map usage is correct
- [ ] Test that circuit breaker doesn't block legitimate duplicate log entries
- [ ] Confirm recursion is prevented even under edge cases

**Risk:** Low - adds safety without changing core behavior

---

## Success Criteria

### Functional Requirements

1. **No Circular Logging:**
   - [ ] Log file does not grow infinitely
   - [ ] "Publishing event" messages for "log_event" type do not appear in logs
   - [ ] Log file size remains reasonable during normal operation

2. **Preserved Event Functionality:**
   - [ ] "log_event" events still published to EventService
   - [ ] WebSocket still receives log events for UI display
   - [ ] Other event types still logged correctly (job_created, job_started, etc.)

3. **Preserved Logging Functionality:**
   - [ ] Application logs still written to files
   - [ ] Job-specific logs still stored in database
   - [ ] Correlation IDs still tracked correctly

### Testing Checklist

**Unit Tests:**
- [ ] Test EventService.Publish() with "log_event" type (should not log)
- [ ] Test EventService.Publish() with other event types (should log)
- [ ] Test LogConsumer circuit breaker prevents recursion

**Integration Tests:**
- [ ] Start application and verify no circular logging in logs
- [ ] Trigger crawler job and verify logs appear in UI
- [ ] Check log file size after 5 minutes of operation (should be < 10MB)
- [ ] Verify WebSocket still receives log events

**Regression Tests:**
- [ ] Verify existing event subscribers still work (WebSocket, EventSubscriber)
- [ ] Verify job logging still persists to database
- [ ] Verify global logger still writes to console and file

## Constraints

### Must Not Change:
- Event-driven architecture pattern
- WebSocket log streaming to UI
- Database log persistence
- Event subscription mechanism
- Existing event types and payloads

### Must Preserve:
- All existing event subscribers (WebSocket, EventSubscriber)
- Log consumer functionality (database writes, event publishing)
- Event service functionality (pub/sub pattern)
- Logger functionality (arbor integration)

## Rollback Plan

If issues occur after implementation:

1. **Immediate Rollback:**
   ```powershell
   git revert <commit-hash>
   .\scripts\build.ps1 -Run
   ```

2. **Alternative Fix (if conditional logging fails):**
   - Remove "log_event" from LogConsumer.publishLogEvent()
   - Implement direct WebSocket broadcasting from LogConsumer
   - Bypass EventService for log events entirely

3. **Emergency Mitigation:**
   - Disable LogConsumer.publishLogEvent() temporarily
   - UI will lose real-time logs but application won't crash

## Post-Implementation Monitoring

### Metrics to Watch:

1. **Log File Size:**
   - Monitor `bin/logs/quaero.*.log` file sizes
   - Alert if any log file > 50MB within 1 hour

2. **Event Counts:**
   - Monitor "log_event" publication count
   - Should correlate with actual application log volume

3. **Performance:**
   - Monitor memory usage (should not grow unbounded)
   - Monitor CPU usage (should not spike from log processing)

### Log Patterns to Check:

```bash
# Check for "Publishing event" for log_event (should be 0):
grep "event_type=log_event" bin/logs/quaero.*.log | grep "Publishing event" | wc -l

# Check for normal event logging (should still exist):
grep "event_type=job_created" bin/logs/quaero.*.log | grep "Publishing event" | wc -l

# Check for log_event deliveries to WebSocket (should exist):
grep "log_event" bin/logs/quaero.*.log | grep "Event published" | wc -l
```

## Edge Cases and Error Handling

### Edge Case 1: New Event Types Added
**Risk:** Future developers add new event types that trigger circular logging
**Mitigation:** Circuit breaker in Step 4 provides defense in depth
**Documentation:** Add comment in event_service.go explaining the blacklist

### Edge Case 2: LogConsumer Fails to Publish
**Current Behavior:** Logs to fmt.Printf (line 172 in consumer.go)
**No Change Needed:** Already handles errors without logger recursion

### Edge Case 3: Multiple LogConsumers
**Risk:** If multiple consumers exist, circuit breaker per-instance might not prevent all recursion
**Mitigation:** Application only initializes one LogConsumer (verified in app initialization)

### Edge Case 4: High-Frequency Log Events
**Risk:** Circuit breaker might block legitimate high-frequency duplicate messages
**Mitigation:** Circuit breaker uses correlation ID + message key, so different messages won't block
**Alternative:** Use time-based circuit breaker (allow same message after 100ms)

## Documentation Updates Required

1. **C:\development\quaero\CLAUDE.md:**
   - Document the nonLoggableEvents blacklist pattern
   - Explain why "log_event" is excluded from logging
   - Add guidance for future event type additions

2. **C:\development\quaero\internal\services\events\event_service.go:**
   - Add inline comment explaining blacklist purpose
   - Document circular logging risk for future maintainers

3. **C:\development\quaero\internal\logs\consumer.go:**
   - Document circuit breaker purpose
   - Explain sync.Map usage for recursion prevention

## References

### Related Files:
- `internal/services/events/event_service.go` (EventService.Publish)
- `internal/logs/consumer.go` (LogConsumer.publishLogEvent)
- `internal/handlers/websocket.go` (WebSocket log_event subscriber)
- `internal/interfaces/event.go` (Event type definitions)

### Related Documentation:
- CLAUDE.md - Architecture Overview → Event-Driven Architecture section
- CLAUDE.md - Code Conventions → Logging section

### Issue Evidence:
- Log file: `bin/logs/quaero.2025-11-08T16-48-02.log` (78.7MB, 401,726 log_event entries)
- Correlation ID: `55b85ff2-b33c-461e-9472-6c52f65b9787` (stuck in infinite loop)

---

## Implementation Timeline

**Estimated Time:** 2-3 hours

1. **Step 1-3 (EventService Changes):** 30 minutes
   - Add blacklist
   - Modify Publish() and PublishSync()
   - Test event logging

2. **Step 4 (Circuit Breaker):** 45 minutes
   - Add sync.Map to Consumer
   - Implement circuit breaker logic
   - Test recursion prevention

3. **Testing:** 1 hour
   - Unit tests for EventService
   - Integration tests with full application
   - Monitor log files during test runs

4. **Documentation:** 30 minutes
   - Update CLAUDE.md
   - Add inline code comments
   - Update this plan with results

---

## Approval and Sign-off

**Plan Created By:** Claude Agent 1 (Planner)
**Date:** 2025-11-08
**Status:** Ready for Implementation

**Next Steps:**
1. Review plan with Agent 2 (Implementer)
2. Execute Steps 1-4 sequentially
3. Run validation tests
4. Agent 3 (Validator) verifies fix

---

## Appendix: Alternative Solutions Considered

### Alternative 1: Remove Event Logging Entirely
**Rejected Reason:** Loses valuable debugging and audit trail information

### Alternative 2: Separate Logger for EventService
**Rejected Reason:** Adds complexity, doesn't solve root cause

### Alternative 3: Move Log Events to Different Channel
**Rejected Reason:** Breaks event-driven architecture pattern

### Alternative 4: Rate-Limit Log Events
**Rejected Reason:** Treats symptom, not root cause - logs would still grow unbounded

### Selected Solution: Conditional Logging + Circuit Breaker
**Reason:** Minimal changes, preserves all functionality, defense in depth approach
