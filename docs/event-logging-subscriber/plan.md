---
task: "All events, as they are received centrally need to be logged via the configured logger. This may be achieved with a logger subscriber"
folder: event-logging-subscriber
complexity: low
estimated_steps: 3
---

# Implementation Plan

## Step 1: Create logger subscriber implementation

**Why:** The EventService needs a generic logger subscriber that logs all events as they are published. Currently, the EventService logs during Publish/PublishSync methods, but there's no dedicated subscriber for comprehensive event logging.

**Depends on:** none

**Validation:** code_compiles, follows_conventions

**Creates/Modifies:**
- `internal/services/events/logger_subscriber.go` (new file)

**Risk:** low

## Step 2: Register logger subscriber during EventService initialization

**Why:** The logger subscriber needs to be attached to all event types during app initialization so all events are logged centrally.

**Depends on:** 1

**Validation:** code_compiles, follows_conventions

**Creates/Modifies:**
- `internal/services/events/service.go` (modify NewService)

**Risk:** low

## Step 3: Add tests for logger subscriber

**Why:** Verify that the logger subscriber correctly logs events for all event types without interfering with existing event handlers.

**Depends on:** 1, 2

**Validation:** tests_must_pass, code_compiles, follows_conventions

**Creates/Modifies:**
- `internal/services/events/logger_subscriber_test.go` (new file)

**Risk:** low

---

## Constraints
- Must not modify existing event logging in Publish/PublishSync (keep existing logs)
- Must subscribe to ALL event types defined in interfaces.EventType
- Must not block or interfere with other event handlers
- Must use structured logging with arbor.ILogger
- Must follow existing event service patterns

## Success Criteria
- Logger subscriber logs all published events with type and payload
- Existing event service functionality unchanged
- Tests pass showing logger subscription works
- Code compiles without errors
- Follows project conventions (stateless subscriber, interface-based, constructor DI)
