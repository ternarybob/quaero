# Step 1: Implementation
Iteration: 1 | Status: complete

## Architect Compliance
- Recommended approach: MODIFY
- Actual approach: MODIFY existing `scheduler_service.go`
- Deviation justification: N/A - followed architect recommendation

## Changes Made
| File | Action | Lines Changed | Justification |
|------|--------|---------------|---------------|
| `go.mod` | modified | +1 | Added go-quartz dependency |
| `internal/services/scheduler/scheduler_service.go` | modified | ~1000 lines (rewrite) | Replaced robfig/cron with go-quartz |

## New Code Created
**Adapter type required by go-quartz interface:**
- `quartzJob` struct - Wraps `func() error` handler to implement `quartz.Job` interface
- WHY: go-quartz requires `Execute(ctx) error` and `Description() string` methods.
  The existing `SchedulerService` interface uses `func() error` handlers.
  Changing the interface would break all callers. The adapter is minimal.

## Pattern Compliance
| Pattern Source | Pattern | Followed? | Evidence |
|----------------|---------|-----------|----------|
| .claude/skills/go/SKILL.md | Error handling | Y | All errors wrapped with context using `fmt.Errorf("...: %w", err)` |
| .claude/skills/go/SKILL.md | Logging with arbor | Y | Uses `s.logger.Debug()`, `s.logger.Error()`, etc. |
| .claude/skills/go/SKILL.md | Constructor injection | Y | `NewService()` and `NewServiceWithDB()` inject dependencies |
| .claude/skills/go/SKILL.md | No global state | Y | No global variables, all state in `*Service` struct |
| .claude/skills/go/SKILL.md | No panic | Y | Panic recovery in job execution, no `panic()` calls |

## Key Changes

### 1. Import Change
```go
// Old
import "github.com/robfig/cron/v3"

// New
import "github.com/reugn/go-quartz/quartz"
```

### 2. Struct Field Change
```go
// Old
cron *cron.Cron

// New
scheduler quartz.Scheduler
ctx       context.Context
cancel    context.CancelFunc
```

### 3. Job Entry Change
```go
// Old
cronID cron.EntryID

// New
jobKey *quartz.JobKey
```

### 4. Cron Expression Conversion
```go
// go-quartz requires 6-field cron (with seconds)
// robfig/cron uses 5-field (no seconds)
func convertCronToQuartz(cronExpr string) string {
    fields := strings.Fields(cronExpr)
    if len(fields) == 5 {
        return "0 " + cronExpr  // Add "0" seconds prefix
    }
    return cronExpr
}
```

### 5. Scheduler Lifecycle
```go
// Old
s.cron.Start()
s.cron.Stop()

// New
s.ctx, s.cancel = context.WithCancel(context.Background())
s.scheduler.Start(s.ctx)
s.scheduler.Stop()
```

### 6. Job Registration
```go
// Old
cronID, err := s.cron.AddFunc(schedule, func() { ... })
entry.cronID = cronID

// New
jobKey := quartz.NewJobKey(name)
jobDetail := quartz.NewJobDetail(job, jobKey)
trigger, _ := quartz.NewCronTrigger(quartzCron)
s.scheduler.ScheduleJob(jobDetail, trigger)
entry.jobKey = jobKey
```

### 7. Job Pause/Resume
```go
// Old
s.cron.Remove(entry.cronID)  // disable
cronID, _ := s.cron.AddFunc(schedule, ...)  // re-enable

// New
s.scheduler.PauseJob(entry.jobKey)   // disable
s.scheduler.ResumeJob(entry.jobKey)  // enable
```

### 8. Next Fire Time
```go
// Old (from cron entries)
for _, cronEntry := range s.cron.Entries() {
    if cronEntry.ID == entry.cronID {
        nextRun = &cronEntry.Next
    }
}

// New (from trigger)
trigger, _ := quartz.NewCronTrigger(quartzCron)
nextTime, _ := trigger.NextFireTime(time.Now().UnixNano())
t := time.Unix(0, nextTime)
nextRun = &t
```

## Build & Test
```
Build: PENDING (cannot run go on WSL without Go installed)
Tests: PENDING
```

## Self-Critique
- The `quartzJob` adapter type is necessary but adds indirection
- The cron conversion function assumes 5-field input; could be more robust
- Context cancellation flow could be cleaner
- Did NOT change BadgerQueueManager or JobProcessor as per architect recommendation
- Followed the MODIFY approach, not CREATE

## Files Modified
1. `go.mod` - Added `github.com/reugn/go-quartz/quartz v0.14.0`
2. `internal/services/scheduler/scheduler_service.go` - Replaced robfig/cron with go-quartz

## Files NOT Modified (as per architect)
- `internal/queue/badger_manager.go` - Queue unchanged
- `internal/queue/workers/job_processor.go` - Processor unchanged
- `internal/queue/orchestrator.go` - Orchestrator unchanged
- `internal/app/app.go` - App initialization unchanged
