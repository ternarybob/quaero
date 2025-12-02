# Logging Architecture Review

**Status:** 📝 REVIEW | **Date:** 2025-12-02

## User Concerns

1. **Test `TestStepEventsDisplay`** - Pass condition is "> 2 events" but test sometimes fails
2. **Context-bound logging** - Worker logs should only log worker-specific context
3. **Pub/Sub architecture** - Collectors should subscribe to loggers for centralized distribution
4. **Centralized collection** - JobManager/StepManager should not directly publish; use a central collector
5. **Database storage** - All logs by level (job/step/worker)
6. **UI filtering** - JavaScript-based filtering of job logs for clean step display
7. **Page refresh support** - Logs should persist and reload correctly at any point

---

## Current Architecture Analysis

### Log Flow (Current)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           CURRENT LOG FLOW                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Workers (Crawler, Agent, Places, etc.)                                     │
│       │                                                                      │
│       ▼                                                                      │
│  jobMgr.AddJobLog(ctx, jobID, level, message)                               │
│       │                                                                      │
│       ├──► JobLogStorage.AppendLog()  ──► BadgerDB (persistent)             │
│       │                                                                      │
│       └──► EventService.Publish(EventJobLog) ──► WebSocket ──► UI           │
│                     │                                                        │
│                     └──► resolveJobContext() ──► Adds step_name, manager_id │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Issues Identified

| Issue | Location | Impact |
|-------|----------|--------|
| **1. Tight coupling** | `JobManager.AddJobLog()` | Workers directly call JobManager, coupling execution and logging |
| **2. Magic context resolution** | `resolveJobContext()` | Walks parent chain to find step context - fragile |
| **3. Multiple event types** | `log_event` vs `job_log` | Two separate event types for logs cause confusion |
| **4. No log level hierarchy** | DB storage | All logs stored flat, no job→step→worker hierarchy |
| **5. WebSocket filtering by level** | `shouldPublishLogLevel()` | Only INFO+ published, but DB has all - UI can't show DEBUG |
| **6. Page refresh loses real-time events** | UI | Events received before page load are missed |

---

## Test Failure Analysis: `TestStepEventsDisplay`

### Root Cause
The test checks for events in the UI after a page refresh. Events published **before** the refresh are lost because:

1. WebSocket events are transient (not stored for replay)
2. On refresh, UI loads logs from DB via API, but:
   - `jobLogs` are stored per-job, not aggregated by step
   - Step event panels query `managerLogs` but filtering by step_name is inconsistent

### Current Test Logic (lines 2162-2227)
```javascript
// Looks for step-events-panel with Events button
// Counts events from data-events-count attribute
// Falls back to managerLogs if step panel empty
```

### Fix Required
The test should:
1. Wait for job completion first
2. Then verify logs are loaded from database (not just WebSocket)
3. Check that step_name filtering works correctly

---

## Proposed Architecture: Centralized Log Collector


### Proposed Log Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          PROPOSED LOG FLOW                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Workers (Crawler, Agent, Places, etc.)                                     │
│       │                                                                      │
│       ▼                                                                      │
│  JobLogger.Log(ctx, level, message)  ◄── context.Value("job_context")       │
│       │                                                                      │
│       ▼                                                                      │
│  LogCollector (Central Service)                                              │
│       │                                                                      │
│       ├──► JobLogStorage.AppendLog()  ──► BadgerDB                          │
│       │         │                                                            │
│       │         └──► Schema: job_id, step_id, worker_type, level, message   │
│       │                                                                      │
│       └──► EventService.Publish(EventJobLog)                                │
│                   │                                                          │
│                   ▼                                                          │
│            WebSocketHandler (subscriber)                                     │
│                   │                                                          │
│                   ▼                                                          │
│            UI (client-side filtering by job_id, step_name)                  │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Key Changes

1. **JobLogger Interface** (new)
   ```go
   type JobLogger interface {
       Debug(ctx context.Context, message string)
       Info(ctx context.Context, message string)
       Warn(ctx context.Context, message string)
       Error(ctx context.Context, message string)
   }
   ```

2. **JobContext** (stored in context.Context)
   ```go
   type JobContext struct {
       JobID      string // Leaf job ID (worker job)
       StepID     string // Step job ID (parent of worker job)
       ManagerID  string // Manager job ID (root)
       StepName   string // Step name for UI filtering
       WorkerType string // Worker type for logging
   }
   ```

3. **LogCollector** (new central service)
   ```go
   type LogCollector struct {
       storage      interfaces.JobLogStorage
       eventService interfaces.EventService
       logger       arbor.ILogger
   }

   func (c *LogCollector) Log(ctx context.Context, level, message string) error {
       jobCtx := GetJobContext(ctx)
       // Store with full hierarchy
       // Publish to EventService
   }
   ```

---

## Redundant Code to Remove

| File | Code | Reason |
|------|------|--------|
| `internal/queue/job_manager.go` | `resolveJobContext()` | Magic context resolution - replace with explicit context |
| `internal/queue/job_manager.go` | `shouldPublishLogLevel()` | Move to LogCollector or make configurable |
| `internal/queue/logging/context_logger.go` | Entire file | Incomplete implementation, replace with LogCollector |
| `internal/logs/consumer.go` | `publishLogEvent()` | Duplicate of job_log publishing |

---

## Database Schema Enhancement

### Current: `job_logs` table
```
job_id (key) → []JobLogEntry{timestamp, level, message}
```

### Proposed: Add hierarchy fields
```go
type JobLogEntry struct {
    JobID       string    `json:"job_id"`        // The job that created this log
    StepID      string    `json:"step_id"`       // Parent step job ID
    ManagerID   string    `json:"manager_id"`    // Root manager job ID
    StepName    string    `json:"step_name"`     // For UI filtering
    WorkerType  string    `json:"worker_type"`   // crawler, agent, places, etc.
    Level       string    `json:"level"`         // debug, info, warn, error
    Message     string    `json:"message"`
    Timestamp   time.Time `json:"timestamp"`
}
```

This enables:
- Query all logs for a manager job (including all steps/workers)
- Filter by step_name for step-specific display
- Filter by worker_type for debugging

---

## UI Changes Required

### 1. Load Logs on Page Refresh
```javascript
// On page load, fetch logs from API
async function loadJobLogs(managerJobId) {
    const response = await fetch(`/api/jobs/${managerJobId}/logs`);
    const logs = await response.json();
    // Store in Alpine.js state
    this.jobLogs[managerJobId] = logs;
}
```

### 2. Client-Side Filtering
```javascript
// Filter logs for a specific step
getStepLogs(managerJobId, stepName) {
    return (this.jobLogs[managerJobId] || [])
        .filter(log => log.step_name === stepName);
}
```

### 3. Real-Time Updates (Merge)
```javascript
// On WebSocket job_log event, merge with existing logs
handleJobLogEvent(event) {
    const { manager_id, step_name, level, message, timestamp } = event.payload;
    if (!this.jobLogs[manager_id]) {
        this.jobLogs[manager_id] = [];
    }
    // Add to logs array (dedup by timestamp if needed)
    this.jobLogs[manager_id].push({ step_name, level, message, timestamp });
}
```

---

## Test Fix: `TestStepEventsDisplay`

### Current Issue
Test sometimes passes, sometimes fails because:
1. Events counted during job execution (transient)
2. Page refresh loses pre-refresh events
3. Post-refresh only sees DB-loaded logs

### Proposed Fix
```go
// After job completes, verify logs from database
// Don't rely on real-time WebSocket events alone

// 1. Wait for job to complete
waitForJobCompletion(jobName)

// 2. Fetch logs via API (not WebSocket)
logs := fetchJobLogs(managerJobId)

// 3. Filter by step and verify count
stepLogs := filterByStep(logs, stepName)
assert(len(stepLogs) > 2, "Expected at least 2 step events")
```

---

## Implementation Priority

1. **P1: Fix Test** - Update test to verify DB logs, not WebSocket events
2. **P2: Add job context to logs** - Store step_id, manager_id in JobLogEntry
3. **P3: UI page refresh** - Load logs from API on page load
4. **P4: LogCollector** - Centralize log distribution (optional refactor)

---

## Next Steps

1. [ ] Update `TestStepEventsDisplay` to verify DB logs after completion
2. [ ] Add `step_id`, `manager_id` fields to `JobLogEntry`
3. [ ] Add API endpoint `GET /api/jobs/{id}/logs`
4. [ ] Update UI to load logs on page refresh
5. [ ] Consider LogCollector refactor for future cleanup
