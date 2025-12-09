# Plan: Simplify Worker and Step Manager Logging

Type: fix | Workdir: ./docs/fix/20251202-logging-simplification/

## User Intent (from manifest)
Simplify the logging API by merging `AddJobLog` and `AddJobLogWithEvent` into a single method that:
1. Always stores logs to the database
2. Always publishes logs to WebSocket for real-time UI updates
3. Auto-resolves step context (stepName, managerID) from the job's parent chain
4. Workers just call one simple method without choosing or passing options

## Current State Analysis

**Two methods exist:**
- `AddJobLog(jobID, level, message)` - Stores to DB only, no WebSocket event
- `AddJobLogWithEvent(jobID, level, message, opts)` - Stores to DB AND publishes to WebSocket

**Problems:**
1. Workers must choose which method to use
2. Workers must build `JobLogOptions` struct with step context
3. Each worker duplicates `buildJobLogOptions()` logic
4. Some logs go to DB but not UI, causing inconsistency

**Workers affected:**
- `agent_worker.go` - uses both methods + `buildJobLogOptions`
- `crawler_worker.go` - uses both methods + `buildJobLogOptions` + `logWithEvent`
- `github_log_worker.go` - uses `AddJobLogWithEvent` + `buildJobLogOptions`
- `github_repo_worker.go` - uses `AddJobLogWithEvent` + `buildJobLogOptions`
- `places_worker.go` - uses `AddJobLogWithEvent`
- `web_search_worker.go` - uses `AddJobLogWithEvent`
- `database_maintenance_worker.go` - uses `AddJobLog`

**Monitors affected:**
- `state/monitor.go` - uses `AddJobLog`
- `state/step_monitor.go` - uses `AddJobLog`
- `state/runtime.go` - uses `AddJobLog`

## Tasks

| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Modify Manager.AddJobLog to auto-resolve context and publish events | - | no | sonnet |
| 2 | Remove AddJobLogWithEvent and JobLogOptions | 1 | no | sonnet |
| 3 | Update workers to use simplified AddJobLog | 2 | no | sonnet |
| 4 | Update monitors to use simplified AddJobLog | 2 | no | sonnet |
| 5 | Build and test | 3,4 | no | sonnet |

## Order
[1] → [2] → [3,4] → [5]

## Implementation Details

### Task 1: Modify Manager.AddJobLog

The new `AddJobLog` will:
1. Store log to database (existing behavior)
2. Auto-resolve step context from job's parent chain
3. Publish to WebSocket if level >= INFO (filter debug/trace)

```go
func (m *Manager) AddJobLog(ctx context.Context, jobID, level, message string) error {
    now := time.Now()

    // 1. Store log entry
    entry := models.JobLogEntry{...}
    if err := m.jobLogStorage.AppendLog(ctx, jobID, entry); err != nil {
        return err
    }

    // 2. Auto-resolve step context from job hierarchy
    stepName, managerID, parentID := m.resolveJobContext(ctx, jobID)

    // 3. Publish to WebSocket if INFO+ level
    if m.eventService != nil && shouldPublishToUI(level) {
        payload := map[string]interface{}{
            "job_id":        jobID,
            "parent_job_id": parentID,
            "manager_id":    managerID,
            "step_name":     stepName,
            "level":         level,
            "message":       message,
            "timestamp":     now.Format(time.RFC3339),
        }
        event := interfaces.Event{Type: interfaces.EventJobLog, Payload: payload}
        go m.eventService.Publish(ctx, event)
    }

    return nil
}
```

### Task 2: Remove AddJobLogWithEvent and JobLogOptions

- Delete `JobLogOptions` struct
- Delete `AddJobLogWithEvent` method
- Delete `shouldPublishLogToUI` (move logic into AddJobLog)

### Task 3: Update Workers

Remove from each worker:
- `buildJobLogOptions()` method
- `logWithEvent()` helper method
- All `AddJobLogWithEvent` calls → replace with `AddJobLog`
- Remove `queue.JobLogOptions` usage

Workers to update:
- agent_worker.go
- crawler_worker.go
- github_log_worker.go
- github_repo_worker.go
- places_worker.go
- web_search_worker.go
- database_maintenance_worker.go (already uses AddJobLog)

### Task 4: Update Monitors

Monitors already use `AddJobLog`, so minimal changes needed:
- state/monitor.go - no changes needed
- state/step_monitor.go - no changes needed
- state/runtime.go - no changes needed

### Task 5: Build and Test

- Run `go build ./...`
- Run tests if available
