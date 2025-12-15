# Architect Analysis
Date: 2025-12-15
Request: "Refactor the internal\queue process and workers to use go-quartz scheduler"

## User Intent
Replace the custom Badger-backed polling queue with go-quartz scheduler for job scheduling and execution, while:
1. **Preserving the Manager->Step->Worker hierarchy** (controlled by TOML orchestration)
2. **Maintaining job logging context** (CRITICAL for UI updates via WebSocket)
3. **Ensuring proper job cancellation** propagates to all children
4. **Simplifying code** where possible without breaking architecture

## Challenge: Is This Refactor Actually Needed?

**Current System Analysis:**
- `BadgerQueueManager` (internal/queue/badger_manager.go) - Custom polling-based queue
- `JobProcessor` (internal/queue/workers/job_processor.go) - Worker pool with polling loop
- **Existing Scheduler Service** (`internal/services/scheduler/scheduler_service.go`) - Uses `robfig/cron/v3`
- Polling interval with exponential backoff (100ms -> 5s)
- Manual concurrency control via goroutine pool

**Existing Scheduler Service (`robfig/cron/v3`):**
- Already handles cron-based scheduling
- Job definitions with schedules loaded from storage
- Job execution via registered handlers
- Persistence via KV storage
- Auto-start jobs feature

**What go-quartz Offers vs Current `robfig/cron/v3`:**

| Feature | robfig/cron (current) | go-quartz | Benefit |
|---------|----------------------|-----------|---------|
| Cron scheduling | Yes | Yes | None |
| One-time triggers | No | RunOnceTrigger | NEW |
| Simple intervals | Limited | SimpleTrigger | Cleaner API |
| Job keys/management | Manual | Built-in JobKey | Cleaner API |
| Distributed mode | No | Custom JobQueue interface | Potential |
| Pause/Resume jobs | Manual | Built-in | Simpler |

**CRITICAL ASSESSMENT:**

The existing `robfig/cron/v3` scheduler already provides:
- Cron expression parsing and scheduling
- Job registration and execution
- Enable/disable jobs
- Next run time calculation

**go-quartz adds:**
- More flexible trigger types (RunOnceTrigger, SimpleTrigger)
- Built-in job key management
- Pause/Resume without re-registration
- Cleaner lifecycle (Start/Stop/Wait)

**VERDICT: REFACTOR EXISTING SCHEDULER TO USE go-quartz**

Replace `robfig/cron/v3` with `reugn/go-quartz` in the existing scheduler service.
This is a MODIFY operation, not CREATE.

**Why NOT replace BadgerQueueManager:**
1. **Persistence** - go-quartz is in-memory by default, Badger provides durability
2. **Message visibility timeout** - go-quartz doesn't have this concept
3. **Dead letter handling** - go-quartz doesn't handle failed job retry
4. **Queue semantics** - BadgerQueueManager is a queue (FIFO), go-quartz is a scheduler

## Existing Code Analysis

| Purpose | Existing Code | Can Extend? | Notes |
|---------|--------------|-------------|-------|
| Scheduler service | `internal/services/scheduler/scheduler_service.go` | **MODIFY** | Replace robfig/cron with go-quartz |
| Cron parsing | Uses robfig/cron parser | REPLACE | go-quartz has CronTrigger |
| Job registration | `RegisterJob()` line 246 | MODIFY | Use scheduler.ScheduleJob() |
| Job enable/disable | `EnableJob()/DisableJob()` lines 289,326 | MODIFY | Use scheduler.PauseJob()/ResumeJob() |
| Job execution | `executeJob()` line 533 | KEEP | Handler wrapping still needed |
| Job status | `GetJobStatus()` line 476 | MODIFY | Use scheduler.GetJobKeys() |
| Scheduler lifecycle | `Start()/Stop()` lines 86,139 | MODIFY | Use scheduler.Start()/Stop() |

## Recommended Approach: **MODIFY**

Modify the existing `internal/services/scheduler/scheduler_service.go` to use `reugn/go-quartz` instead of `robfig/cron/v3`.

### Files to Modify:
1. **`internal/services/scheduler/scheduler_service.go`**
   - Replace `github.com/robfig/cron/v3` import with `github.com/reugn/go-quartz/quartz`
   - Replace `*cron.Cron` with `quartz.Scheduler`
   - Replace `cron.EntryID` with `*quartz.JobDetail`
   - Update `RegisterJob()` to use `scheduler.ScheduleJob()`
   - Update `EnableJob()/DisableJob()` to use `PauseJob()/ResumeJob()`
   - Update `Start()/Stop()` to use scheduler lifecycle

2. **`go.mod`**
   - Add `github.com/reugn/go-quartz` dependency
   - Keep `github.com/robfig/cron/v3` (still used by common.ValidateJobSchedule)

### What Will NOT Change:
- `BadgerQueueManager` - Queue semantics are different from scheduler
- `JobProcessor` - Handles queue-based execution, not scheduling
- `Orchestrator` - Orchestrates job definitions, not scheduling
- All workers - Unchanged
- Job logging - Unchanged
- App initialization flow - Mostly unchanged

## Anti-Patterns Check
| Anti-Pattern | Risk | Mitigation |
|--------------|------|------------|
| Creating parallel structure | NO | Modifying existing scheduler service |
| Duplicating existing logic | NO | Replacing, not duplicating |
| Ignoring existing patterns | NO | Following existing service patterns |
| Breaking job cancellation | LOW | Cancellation uses EventService, not scheduler |
| Breaking logging | NO | Logging untouched |

## Success Criteria (MEASURABLE)
- [ ] Build passes: `go build ./...`
- [ ] Scheduler service starts without errors
- [ ] Jobs can be registered with cron schedules
- [ ] Jobs execute at scheduled times
- [ ] Jobs can be enabled/disabled
- [ ] Jobs can be triggered manually
- [ ] Scheduler stops gracefully
- [ ] No regression in existing functionality

## Architecture Requirements (from docs)
| Doc | Section | Requirement | Applicable? |
|-----|---------|-------------|-------------|
| manager_worker_architecture.md | Core Components | Manager->Step->Worker hierarchy | N - Scheduler is separate |
| QUEUE_LOGGING.md | Logging Flow | AddJobLog flow | N - Scheduler logs differently |
| QUEUE_SERVICES.md | Service Init | Dependency order | Y - Scheduler init order |
| .claude/skills/go/SKILL.md | Error Handling | Wrap errors with context | Y |
| .claude/skills/go/SKILL.md | DI | Constructor injection | Y |
| .claude/skills/go/SKILL.md | Logging | Use arbor structured logging | Y |
| .claude/skills/go/SKILL.md | Anti-patterns | No global state, no panic | Y |

## Files to Modify (Minimum Viable Change)

1. **MODIFY**: `internal/services/scheduler/scheduler_service.go`
   - Replace cron library with go-quartz
   - Update job entry structure
   - Update registration/enable/disable methods
   - Update lifecycle methods

2. **MODIFY**: `go.mod`
   - Add go-quartz dependency

## What Will NOT Be Changed (Scope Control)
- `internal/queue/*` - Queue system unchanged
- `internal/queue/workers/*` - Workers unchanged
- `internal/app/app.go` - Minimal changes (if any)
- All handlers - Unchanged
- All tests - May need updates for scheduler behavior

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Breaking scheduled jobs | MEDIUM | HIGH | Test all job types |
| Breaking enable/disable | LOW | MEDIUM | Test UI controls |
| Breaking manual trigger | LOW | MEDIUM | Test trigger button |
| go-quartz bugs | LOW | MEDIUM | Use stable version |
| API incompatibility | MEDIUM | LOW | Adapter methods |
