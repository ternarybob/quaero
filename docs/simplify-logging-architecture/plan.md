---
task: "Simplify logging architecture - remove complexity from internal/logs/service.go and internal/common/logger.go"
folder: simplify-logging-architecture
complexity: high
estimated_steps: 5
---

# Implementation Plan

## Current State Analysis

### Problems Identified:
1. **logs.Service** is overly complex (719 lines) - mixing log persistence, event publishing, and transformation
2. **common.logger.go** handles global logger management which is simple and correct
3. **Duplicate concerns:** LogService has both storage delegation AND event publishing
4. **Channel complexity:** LogService manages its own consumer goroutine for arbor channel
5. **MinEventLevel filtering** happens in LogService instead of during logger configuration

### Correct Architecture:
1. **ONE global arbor logger** configured at startup in `main.go`
2. **Logger configured ONCE** with all writers (console, file, memory, context channel)
3. **Single consumer goroutine** processes all logs from arbor's context channel
4. **Consumer publishes to EventService** based on TOML config (min_event_level)
5. **LogService simplified** to only handle storage operations (no consumer, no event publishing)

## Step 1: Remove consumer/channel logic from LogService

**Why:** LogService should only handle log storage operations, not event publishing or channel consumption

**Depends on:** none

**Validation:** code_compiles

**Creates/Modifies:**
- `internal/logs/service.go` - Remove Start(), Stop(), consumer(), publishLogEvent(), transformEvent(), GetChannel(), minEventLevel field

**Risk:** medium

## Step 2: Create dedicated log consumer in common package

**Why:** Centralize log consumption from arbor's context channel in one place

**Depends on:** 1

**Validation:** code_compiles, follows_conventions

**Creates/Modifies:**
- `internal/common/log_consumer.go` (new file) - Single consumer that publishes to EventService

**Risk:** medium

## Step 3: Update app initialization to wire consumer correctly

**Why:** App.New() must start the consumer and configure arbor's context channel

**Depends on:** 2

**Validation:** code_compiles

**Creates/Modifies:**
- `internal/app/app.go` - Update LogService initialization, start consumer
- `internal/logs/service.go` - Update NewService signature (no EventService, no minEventLevel)

**Risk:** high

## Step 4: Remove redundant storage delegation methods from LogService

**Why:** LogService currently has pass-through methods that just delegate to storage - these should be removed and callers should use storage directly

**Depends on:** 3

**Validation:** code_compiles, follows_conventions

**Creates/Modifies:**
- `internal/logs/service.go` - Keep only GetAggregatedLogs (complex logic), remove simple delegations
- Call sites throughout codebase - Update to use storage directly where appropriate

**Risk:** medium

## Step 5: Update tests to reflect simplified architecture

**Why:** Tests need to be updated to match new initialization flow

**Depends on:** 4

**Validation:** tests_must_pass, code_compiles

**Creates/Modifies:**
- Test files that use LogService
- Integration tests for log consumer

**Risk:** low

---

## Constraints
- Breaking changes are acceptable (per user requirement)
- ONE global arbor logger configured at startup
- Logger has context channel that pushes to consumer
- Consumer publishes events based on min_event_level from config
- LogService simplified to storage operations only

## Success Criteria
- Single arbor logger configured once in main.go
- Single consumer goroutine in common package
- LogService reduced to ~200 lines (from 719)
- No duplicate log transformation/publishing logic
- All tests pass
- Code compiles
- Follows CLAUDE.md conventions
