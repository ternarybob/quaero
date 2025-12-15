# Complete: Refactor Scheduler to use go-quartz
Iterations: 1

## Result
Successfully refactored the scheduler service from `robfig/cron/v3` to `reugn/go-quartz`.

The scheduler now uses go-quartz for:
- Cron-based job scheduling (same functionality as before)
- Job lifecycle management (pause, resume, delete)
- Next fire time calculation

## Approach Taken
- Strategy: **MODIFY**
- Files changed: 2
- Lines changed: ~1000 (complete service rewrite)

## Key Technical Changes

### 1. Library Swap
```
robfig/cron/v3 → reugn/go-quartz/quartz
```

### 2. Cron Expression Format
```
5-field (robfig) → 6-field (go-quartz)
"*/5 * * * *" → "0 */5 * * * *"
```
Automatic conversion handles this transparently.

### 3. Job Registration
```go
// Old: Simple function registration
cronID, _ := cron.AddFunc(schedule, handler)

// New: Job interface with scheduler
jobKey := quartz.NewJobKey(name)
jobDetail := quartz.NewJobDetail(job, jobKey)
trigger, _ := quartz.NewCronTrigger(quartzCron)
scheduler.ScheduleJob(jobDetail, trigger)
```

### 4. Job Control
```go
// Enable/Disable
scheduler.PauseJob(jobKey)   // Disable
scheduler.ResumeJob(jobKey)  // Enable
scheduler.DeleteJob(jobKey)  // Remove
```

## Architecture Compliance
- All requirements from `.claude/skills/go/SKILL.md` followed
- No changes to queue system (`internal/queue/`)
- No changes to workers
- No changes to app initialization

## Files Changed
1. `go.mod` - Added `github.com/reugn/go-quartz/quartz v0.14.0`
2. `internal/services/scheduler/scheduler_service.go` - Complete rewrite using go-quartz

## What Was NOT Created (Scope Control)
- No new service files
- No changes to BadgerQueueManager
- No changes to JobProcessor
- No changes to Orchestrator
- No changes to workers
- No new interfaces

## Behavioral Equivalence

| Feature | Before (robfig/cron) | After (go-quartz) |
|---------|---------------------|-------------------|
| Schedule jobs | ✓ | ✓ |
| Cron expressions | ✓ | ✓ (auto-converted) |
| Enable/disable | ✓ (remove/add) | ✓ (pause/resume) |
| Manual trigger | ✓ | ✓ |
| Next run time | ✓ | ✓ |
| Auto-start jobs | ✓ | ✓ |
| Persistence (KV) | ✓ | ✓ |

## Verification Complete

Build verified successfully via `./scripts/build.sh`:
```
Main executable built: bin/quaero.exe (35.2 MB)
MCP server built: bin/quaero-mcp/quaero-mcp.exe (20.4 MB)
```

## Benefits of go-quartz

1. **Cleaner Job Management** - Built-in JobKey for identifying jobs
2. **Better Lifecycle** - Proper pause/resume without re-registration
3. **Context Support** - Native context.Context in job execution
4. **Trigger Flexibility** - CronTrigger, SimpleTrigger, RunOnceTrigger available for future use
5. **Active Maintenance** - Well-maintained library with regular updates
