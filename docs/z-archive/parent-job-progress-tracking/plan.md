# Parent Job Progress Tracking - Implementation Plan

## Task Metadata

**Task ID**: parent-job-progress-tracking
**Priority**: High
**Complexity**: Medium
**Estimated Steps**: 8
**Agent**: Agent 1 (Planner)
**Date**: 2025-11-08

## Problem Statement

The UI screenshot (`C:/Users/bobmc/Pictures/Screenshots/ksnip_20251108-173620.png`) shows a "News Crawler" parent job with status "Orchestrating" and progress display showing:
- **Progress**: 66 pending, 1 running, 41 completed

However, the current implementation lacks real-time event-driven updates for parent job progress. The `ParentJobExecutor` polls child job stats every 5 seconds, but doesn't publish formatted progress strings for WebSocket consumption.

### Current State Analysis

**✅ What's Working:**
1. `ParentJobExecutor` monitors child jobs every 5 seconds (lines 127-174 in `parent_job_executor.go`)
2. `GetChildJobStats()` method exists in `Manager` (lines 1543-1574 in `manager.go`)
3. `child_job_stats` event is published with statistics (lines 255-284 in `parent_job_executor.go`)
4. WebSocket handler has `JobStatusUpdate` struct with progress fields (lines 176-197 in `websocket.go`)

**❌ What's Missing:**
1. No subscription to child job status change events
2. Progress text format doesn't match requirement ("X pending, Y running, Z completed, W failed")
3. No event published when individual child jobs change status
4. No logging of status changes to job logs
5. WebSocket doesn't receive job-specific progress updates keyed by `[job-id]`

## Requirements

### Functional Requirements

1. **Parent Job Monitoring** (✅ Partial)
   - Parent executor already monitors children via polling
   - **Need**: Event-driven monitoring on child status changes

2. **Event Publishing** (✅ Partial)
   - `child_job_stats` event exists
   - **Need**: Publish on every child status change, not just polling interval

3. **Event Triggers** (❌ Missing)
   - **Need**: Subscribe to child job status change events
   - Trigger stats calculation when queue status changes

4. **Overall Status Calculation** (✅ Exists)
   - Already calculated in `checkChildJobProgress()` (lines 177-219)
   - **Need**: Trigger calculation on child status changes

5. **Event-Driven Status Updates** (❌ Missing)
   - **Need**: Subscribe to `EventJobCompleted`, `EventJobFailed`, `EventJobStarted`
   - Parent should react to child events

6. **Logging** (⚠️ Partial)
   - Job logs exist via `AddJobLog()`
   - **Need**: Log child status changes

7. **WebSocket Output** (⚠️ Partial)
   - `child_job_stats` event broadcasts to WebSocket
   - **Need**: Format as "X pending, Y running, Z completed, W failed"
   - **Need**: Progress keyed by job ID for UI consumption

## Architecture Analysis

### Event Flow (Current)

```
┌─────────────────────────────────────────────────────┐
│ ParentJobExecutor (Polling)                         │
│ - Monitors every 5 seconds                          │
│ - Calls GetChildJobStats()                          │
│ - Publishes "child_job_stats" event                 │
└─────────────────┬───────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────┐
│ EventService                                        │
│ - Publishes "child_job_stats"                       │
└─────────────────┬───────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────┐
│ WebSocket Handler                                   │
│ - No subscription to "child_job_stats"              │
│ - ❌ No real-time updates                           │
└─────────────────────────────────────────────────────┘
```

### Event Flow (Proposed)

```
┌─────────────────────────────────────────────────────┐
│ Child Job Executor (Any)                            │
│ - Completes/Fails/Starts job                        │
│ - Calls UpdateJobStatus()                           │
└─────────────────┬───────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────┐
│ Manager.UpdateJobStatus()                           │
│ - Updates database                                  │
│ - Publishes "job_status_change" event               │
│   - Includes parent_id in payload                   │
└─────────────────┬───────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────┐
│ ParentJobExecutor (Subscriber)                      │
│ - Subscribes to "job_status_change"                 │
│ - Filters events by parent_id                       │
│ - Calculates child stats                            │
│ - Publishes "parent_job_progress" event             │
└─────────────────┬───────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────┐
│ EventService                                        │
│ - Publishes "parent_job_progress"                   │
└─────────────────┬───────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────┐
│ WebSocket Handler                                   │
│ - Subscribes to "parent_job_progress"               │
│ - Formats progress string                           │
│ - Broadcasts to UI with job_id key                  │
└─────────────────────────────────────────────────────┘
```

## Implementation Steps

### Step 1: Add Event Type for Job Status Changes

**File**: `C:\development\quaero\internal\interfaces\event_service.go`

**Action**: Add new event type constant

```go
// EventJobStatusChange represents a job status transition
// Published when any job changes status (pending → running → completed/failed/cancelled)
// Used by ParentJobExecutor to track child job progress
EventJobStatusChange EventType = "job_status_change"
```

**Rationale**:
- Centralized event type definition
- Follows existing pattern (`EventJobCreated`, `EventJobCompleted`, etc.)
- Clear semantic meaning for status transitions

**Success Criteria**:
- Event type added to interfaces
- No compilation errors

**Lines**: After line 162 in `event_service.go`

---

### Step 2: Publish Status Change Events from Manager

**File**: `C:\development\quaero\internal\jobs\manager.go`

**Action**: Modify `UpdateJobStatus()` to publish events

**Current Code** (lines 471-495):
```go
func (m *Manager) UpdateJobStatus(ctx context.Context, jobID, status string) error {
    now := time.Now()
    nowUnix := timeToUnix(now)

    query := "UPDATE jobs SET status = ?, last_heartbeat = ?"
    args := []interface{}{status, nowUnix}

    if status == "running" {
        query += ", started_at = ?"
        args = append(args, nowUnix)
    } else if status == "completed" || status == "failed" || status == "cancelled" {
        query += ", completed_at = ?"
        args = append(args, nowUnix)
    }

    query += " WHERE id = ?"
    args = append(args, jobID)

    return retryOnBusy(ctx, func() error {
        _, err := m.db.ExecContext(ctx, query, args...)
        return err
    })
}
```

**New Code**:
```go
func (m *Manager) UpdateJobStatus(ctx context.Context, jobID, status string) error {
    // Get job details before update to access parent_id and job_type
    var parentID sql.NullString
    var jobType string
    err := m.db.QueryRowContext(ctx, `
        SELECT parent_id, job_type FROM jobs WHERE id = ?
    `, jobID).Scan(&parentID, &jobType)

    if err != nil && err != sql.ErrNoRows {
        return fmt.Errorf("failed to get job details: %w", err)
    }

    now := time.Now()
    nowUnix := timeToUnix(now)

    query := "UPDATE jobs SET status = ?, last_heartbeat = ?"
    args := []interface{}{status, nowUnix}

    if status == "running" {
        query += ", started_at = ?"
        args = append(args, nowUnix)
    } else if status == "completed" || status == "failed" || status == "cancelled" {
        query += ", completed_at = ?"
        args = append(args, nowUnix)
    }

    query += " WHERE id = ?"
    args = append(args, jobID)

    err = retryOnBusy(ctx, func() error {
        _, err := m.db.ExecContext(ctx, query, args...)
        return err
    })

    if err != nil {
        return err
    }

    // Publish job status change event for parent job monitoring
    // Only publish if eventService is available (optional dependency)
    if m.eventService != nil {
        payload := map[string]interface{}{
            "job_id":     jobID,
            "status":     status,
            "job_type":   jobType,
            "timestamp":  now.Format(time.RFC3339),
        }

        // Include parent_id if this is a child job
        if parentID.Valid {
            payload["parent_id"] = parentID.String
        }

        event := interfaces.Event{
            Type:    interfaces.EventJobStatusChange,
            Payload: payload,
        }

        // Publish asynchronously to avoid blocking status updates
        go func() {
            if err := m.eventService.Publish(ctx, event); err != nil {
                // Log error but don't fail the status update
                // EventService will handle logging via its subscribers
            }
        }()
    }

    return nil
}
```

**Changes Required**:
1. Add `eventService` field to `Manager` struct
2. Modify `NewManager()` constructor to accept `EventService` parameter
3. Add status change event publishing after successful update

**Rationale**:
- Event published AFTER database update ensures consistency
- Async publishing prevents blocking status updates
- Parent ID included in payload enables filtering
- Graceful degradation if eventService is nil

**Success Criteria**:
- Status change events published on every `UpdateJobStatus()` call
- Events include job_id, parent_id, status, job_type
- No performance degradation (async publishing)

**Lines**: Modify lines 471-495, add eventService field to struct at line 21

---

### Step 3: Add EventService Dependency to Manager

**File**: `C:\development\quaero\internal\jobs\manager.go`

**Action**: Add EventService field and update constructor

**Current Struct** (lines 18-23):
```go
type Manager struct {
    db    *sql.DB
    queue *queue.Manager
}

func NewManager(db *sql.DB, queue *queue.Manager) *Manager {
    return &Manager{
        db:    db,
        queue: queue,
    }
}
```

**New Struct**:
```go
type Manager struct {
    db           *sql.DB
    queue        *queue.Manager
    eventService interfaces.EventService // Optional: may be nil for testing
}

func NewManager(db *sql.DB, queue *queue.Manager, eventService interfaces.EventService) *Manager {
    return &Manager{
        db:           db,
        queue:        queue,
        eventService: eventService,
    }
}
```

**Rationale**:
- EventService is optional (can be nil) for backward compatibility
- Allows Manager to publish events without tight coupling
- Follows dependency injection pattern

**Success Criteria**:
- Manager can publish events when eventService is not nil
- No breaking changes to existing tests
- All Manager creation sites updated

**Breaking Change**: Yes - requires updating all `NewManager()` calls in:
- `C:\development\quaero\internal\app\app.go`
- Test files

**Lines**: Modify lines 18-30

---

### Step 4: Subscribe to Status Changes in ParentJobExecutor

**File**: `C:\development\quaero\internal\jobs\processor\parent_job_executor.go`

**Action**: Add subscription to `job_status_change` events

**Add New Method**:
```go
// SubscribeToChildStatusChanges subscribes to child job status change events
// This enables real-time progress tracking without polling
func (e *ParentJobExecutor) SubscribeToChildStatusChanges() {
    if e.eventService == nil {
        return
    }

    // Subscribe to all job status changes
    e.eventService.Subscribe(interfaces.EventJobStatusChange, func(ctx context.Context, event interfaces.Event) error {
        payload, ok := event.Payload.(map[string]interface{})
        if !ok {
            e.logger.Warn().Msg("Invalid job_status_change payload type")
            return nil
        }

        // Extract event data
        jobID := getStringFromPayload(payload, "job_id")
        parentID := getStringFromPayload(payload, "parent_id")
        status := getStringFromPayload(payload, "status")
        jobType := getStringFromPayload(payload, "job_type")

        // Only process child job status changes
        if parentID == "" {
            return nil // Not a child job, ignore
        }

        // Log the status change
        e.logger.Debug().
            Str("job_id", jobID).
            Str("parent_id", parentID).
            Str("status", status).
            Str("job_type", jobType).
            Msg("Child job status changed")

        // Get fresh child job stats for the parent
        stats, err := e.jobMgr.GetChildJobStats(ctx, parentID)
        if err != nil {
            e.logger.Error().Err(err).
                Str("parent_id", parentID).
                Msg("Failed to get child job stats after status change")
            return nil // Don't fail the event handler
        }

        // Generate progress text in required format
        progressText := e.formatProgressText(stats)

        // Add job log for parent job
        e.jobMgr.AddJobLog(ctx, parentID, "info",
            fmt.Sprintf("Child job %s → %s. %s",
                jobID[:8], // Short job ID for readability
                status,
                progressText))

        // Publish parent job progress update
        e.publishParentJobProgressUpdate(ctx, parentID, stats, progressText)

        return nil
    })

    e.logger.Info().Msg("ParentJobExecutor subscribed to child job status changes")
}

// formatProgressText generates the required progress format
// Example: "66 pending, 1 running, 41 completed, 0 failed"
func (e *ParentJobExecutor) formatProgressText(stats *jobs.ChildJobStats) string {
    return fmt.Sprintf("%d pending, %d running, %d completed, %d failed",
        stats.PendingChildren,
        stats.RunningChildren,
        stats.CompletedChildren,
        stats.FailedChildren)
}

// publishParentJobProgressUpdate publishes progress update for WebSocket consumption
func (e *ParentJobExecutor) publishParentJobProgressUpdate(
    ctx context.Context,
    parentJobID string,
    stats *jobs.ChildJobStats,
    progressText string) {

    if e.eventService == nil {
        return
    }

    // Calculate overall status based on child states
    overallStatus := e.calculateOverallStatus(stats)

    payload := map[string]interface{}{
        "job_id":             parentJobID,
        "status":             overallStatus,
        "total_children":     stats.TotalChildren,
        "pending_children":   stats.PendingChildren,
        "running_children":   stats.RunningChildren,
        "completed_children": stats.CompletedChildren,
        "failed_children":    stats.FailedChildren,
        "cancelled_children": stats.CancelledChildren,
        "progress_text":      progressText, // "X pending, Y running, Z completed, W failed"
        "timestamp":          time.Now().Format(time.RFC3339),
    }

    event := interfaces.Event{
        Type:    "parent_job_progress",
        Payload: payload,
    }

    // Publish asynchronously
    go func() {
        if err := e.eventService.Publish(ctx, event); err != nil {
            e.logger.Warn().Err(err).
                Str("parent_job_id", parentJobID).
                Msg("Failed to publish parent job progress event")
        }
    }()
}

// calculateOverallStatus determines parent job status from child statistics
func (e *ParentJobExecutor) calculateOverallStatus(stats *jobs.ChildJobStats) string {
    // If no children yet, status is determined by parent job state (handled elsewhere)
    if stats.TotalChildren == 0 {
        return "running" // Waiting for children to spawn
    }

    // If any children are running, parent is "Running"
    if stats.RunningChildren > 0 {
        return "running"
    }

    // If any children are pending, parent is "Pending" or "Running"
    if stats.PendingChildren > 0 {
        return "running" // Still orchestrating
    }

    // All children in terminal state
    terminalCount := stats.CompletedChildren + stats.FailedChildren + stats.CancelledChildren
    if terminalCount >= stats.TotalChildren {
        // All children complete - determine success/failure
        if stats.FailedChildren > 0 {
            return "failed" // At least one child failed
        }
        if stats.CancelledChildren == stats.TotalChildren {
            return "cancelled" // All children cancelled
        }
        return "completed" // All children succeeded
    }

    return "running" // Default state
}

// Helper function to safely extract string from payload
func getStringFromPayload(payload map[string]interface{}, key string) string {
    if val, ok := payload[key]; ok {
        if str, ok := val.(string); ok {
            return str
        }
    }
    return ""
}
```

**Integration Point**:
Call `SubscribeToChildStatusChanges()` from `NewParentJobExecutor()` constructor:

```go
func NewParentJobExecutor(
    jobMgr *jobs.Manager,
    eventService interfaces.EventService,
    logger arbor.ILogger,
) *ParentJobExecutor {
    executor := &ParentJobExecutor{
        jobMgr:       jobMgr,
        eventService: eventService,
        logger:       logger,
    }

    // Subscribe to child job status changes for real-time progress tracking
    executor.SubscribeToChildStatusChanges()

    return executor
}
```

**Rationale**:
- Event-driven approach eliminates polling delay
- Progress updates happen immediately when child jobs change status
- Formatted progress text matches UI requirement exactly
- Overall status calculation centralizes business logic

**Success Criteria**:
- Parent job progress updates published on every child status change
- Progress text format: "X pending, Y running, Z completed, W failed"
- Job logs show child status transitions
- No duplicate progress updates

**Lines**: Add after line 284 in `parent_job_executor.go`

---

### Step 5: Subscribe to Progress Events in WebSocket Handler

**File**: `C:\development\quaero\internal\handlers\websocket.go`

**Action**: Add subscription to `parent_job_progress` event

**Location**: In `SubscribeToCrawlerEvents()` method (after line 995)

**Add Code**:
```go
// Subscribe to parent job progress events for real-time monitoring
h.eventService.Subscribe("parent_job_progress", func(ctx context.Context, event interfaces.Event) error {
    payload, ok := event.Payload.(map[string]interface{})
    if !ok {
        h.logger.Warn().Msg("Invalid parent_job_progress event payload type")
        return nil
    }

    // Check whitelist (empty allowedEvents = allow all)
    if len(h.allowedEvents) > 0 && !h.allowedEvents["parent_job_progress"] {
        return nil
    }

    // Extract job_id and progress_text for WebSocket message
    jobID := getString(payload, "job_id")
    progressText := getString(payload, "progress_text")
    status := getString(payload, "status")

    // Create simplified WebSocket message with job_id key
    // UI will use job_id to update specific job row
    wsPayload := map[string]interface{}{
        "job_id":        jobID,
        "progress_text": progressText, // "66 pending, 1 running, 41 completed, 0 failed"
        "status":        status,
        "timestamp":     getString(payload, "timestamp"),

        // Include child statistics for advanced UI features
        "total_children":     getInt(payload, "total_children"),
        "pending_children":   getInt(payload, "pending_children"),
        "running_children":   getInt(payload, "running_children"),
        "completed_children": getInt(payload, "completed_children"),
        "failed_children":    getInt(payload, "failed_children"),
        "cancelled_children": getInt(payload, "cancelled_children"),
    }

    // Broadcast to all clients
    msg := WSMessage{
        Type:    "parent_job_progress",
        Payload: wsPayload,
    }

    data, err := json.Marshal(msg)
    if err != nil {
        h.logger.Error().Err(err).Msg("Failed to marshal parent job progress message")
        return nil
    }

    h.mu.RLock()
    clients := make([]*websocket.Conn, 0, len(h.clients))
    mutexes := make([]*sync.Mutex, 0, len(h.clients))
    for conn := range h.clients {
        clients = append(clients, conn)
        mutexes = append(mutexes, h.clientMutex[conn])
    }
    h.mu.RUnlock()

    for i, conn := range clients {
        mutex := mutexes[i]
        mutex.Lock()
        err := conn.WriteMessage(websocket.TextMessage, data)
        mutex.Unlock()

        if err != nil {
            h.logger.Warn().Err(err).Msg("Failed to send parent job progress to client")
        }
    }

    return nil
})
```

**Rationale**:
- WebSocket broadcasts progress keyed by job_id for UI targeting
- Progress text is pre-formatted on backend ("X pending, Y running...")
- Raw statistics included for advanced UI features
- Follows existing WebSocket event subscription pattern

**Success Criteria**:
- WebSocket receives `parent_job_progress` events
- Message includes job_id and progress_text
- UI can consume progress updates by job_id

**Lines**: Add after line 995 in `websocket.go`

---

### Step 6: Update App Initialization

**File**: `C:\development\quaero\internal\app\app.go`

**Action**: Pass EventService to Manager constructor

**Current Code** (approximate line numbers):
```go
// Create jobs manager
jobManager := jobs.NewManager(db, queueManager)
```

**New Code**:
```go
// Create jobs manager with event service for status change publishing
jobManager := jobs.NewManager(db, queueManager, eventService)
```

**Rationale**:
- Manager needs EventService to publish status change events
- Follows existing dependency injection pattern
- EventService already initialized earlier in app.go

**Success Criteria**:
- Manager receives EventService dependency
- No compilation errors
- All tests pass

**Lines**: Find and update `NewManager()` call in `app.go`

---

### Step 7: Add Logging for Status Changes

**File**: `C:\development\quaero\internal\jobs\manager.go`

**Action**: Log status changes to job_logs table

**Modify**: `UpdateJobStatus()` method (after Step 2 changes)

**Add Before Event Publishing**:
```go
// Add job log for status change
logMessage := fmt.Sprintf("Status changed: %s", status)
if err := m.AddJobLog(ctx, jobID, "info", logMessage); err != nil {
    // Log error but don't fail the status update
    // (logging is non-critical)
}
```

**Rationale**:
- Provides audit trail of status changes
- Helps debugging job execution issues
- Non-blocking (errors don't fail status update)

**Success Criteria**:
- Every status change logged to job_logs
- Logs visible in UI job detail view
- No performance impact

**Lines**: Add in `UpdateJobStatus()` after database update

---

### Step 8: Remove Polling from ParentJobExecutor (Optional Optimization)

**File**: `C:\development\quaero\internal\jobs\processor\parent_job_executor.go`

**Action**: Modify `monitorChildJobs()` to rely on events instead of polling

**Current Code** (lines 127-174):
```go
// Start monitoring loop
ticker := time.NewTicker(5 * time.Second) // Check every 5 seconds
defer ticker.Stop()
```

**Proposed Optimization**:
```go
// Start monitoring loop with longer interval (event-driven updates handle real-time)
// This polling is now a backup mechanism for missed events
ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds as backup
defer ticker.Stop()
```

**Rationale**:
- Event-driven updates provide real-time progress
- Polling reduced to backup mechanism for missed events
- Reduces database query load
- Maintains safety (still polls, just less frequently)

**Success Criteria**:
- Real-time updates still work
- Backup polling catches edge cases
- Reduced database load

**Lines**: Modify line 127

**Note**: This is optional and can be implemented after validating event-driven approach works correctly.

---

## Edge Cases & Considerations

### 1. Race Conditions
**Scenario**: Multiple children complete simultaneously
**Mitigation**:
- Database transactions ensure consistency
- Event publishing is async (doesn't block)
- Latest stats always queried from database

### 2. Parent Completes Before All Children
**Scenario**: Parent job marked complete but children still running
**Current Behavior**: `checkChildJobProgress()` waits for all terminal states
**No Change Needed**: Existing logic is correct

### 3. Child Job Failures
**Scenario**: Multiple children fail
**Behavior**:
- Each failure triggers status change event
- Progress text shows failed count
- Overall status calculated as "failed" if any child fails

### 4. Memory Management
**Scenario**: Long-running jobs with thousands of children
**Mitigation**:
- Event payloads are small (< 1KB)
- No in-memory accumulation of stats
- Stats queried from database on-demand

### 5. Event Service Unavailable
**Scenario**: EventService is nil or fails
**Mitigation**:
- Manager checks `if m.eventService != nil` before publishing
- Polling backup in ParentJobExecutor still works
- System degrades gracefully (no real-time updates, but polling works)

### 6. WebSocket Disconnection
**Scenario**: UI disconnects and reconnects
**Behavior**:
- Missed events not replayed
- UI should query latest stats on reconnect (GET /api/jobs/:id)
- Future enhancement: Event replay buffer

## Testing Strategy

### Unit Tests Required

1. **Manager.UpdateJobStatus()**
   - Test event publishing with parent_id
   - Test event publishing without parent_id (root job)
   - Test with nil eventService (no crash)

2. **ParentJobExecutor.SubscribeToChildStatusChanges()**
   - Test event filtering (only child jobs processed)
   - Test progress text formatting
   - Test overall status calculation

3. **WebSocket Handler**
   - Test parent_job_progress subscription
   - Test message format

### Integration Tests Required

1. **End-to-End Progress Flow**
   - Create parent job
   - Spawn child jobs
   - Update child job status
   - Verify WebSocket receives progress update
   - Verify progress text format

2. **Concurrent Child Updates**
   - Update multiple children simultaneously
   - Verify no lost events
   - Verify stats accuracy

### Manual Testing Checklist

- [ ] Create crawler job in UI
- [ ] Observe real-time progress updates in "Progress" column
- [ ] Verify format: "X pending, Y running, Z completed, W failed"
- [ ] Check job logs show child status transitions
- [ ] Verify WebSocket DevTools shows `parent_job_progress` messages
- [ ] Test with multiple concurrent jobs

## Breaking Changes

### API Changes
**None** - All changes are internal

### Database Schema Changes
**None** - Uses existing tables

### Configuration Changes
**None** - No new config required

### Backward Compatibility
- ✅ Manager constructor signature changes (requires updating call sites)
- ✅ Graceful degradation if EventService is nil
- ✅ Polling backup mechanism remains functional

## Rollback Plan

If issues arise:

1. **Immediate**: Set `EventService = nil` in Manager initialization
   - System falls back to polling-only mode
   - No real-time updates, but functional

2. **Quick Fix**: Disable event subscription in ParentJobExecutor
   - Comment out `SubscribeToChildStatusChanges()` call
   - Revert to polling-only

3. **Full Rollback**: Revert all changes
   - Remove EventService from Manager
   - Remove subscription code from ParentJobExecutor
   - System works as before

## Success Metrics

### Performance
- Real-time progress updates (< 1 second latency)
- Database query reduction (polling every 30s instead of 5s)
- No WebSocket message flooding (events only on status changes)

### Functional
- ✅ Progress format matches requirement exactly
- ✅ WebSocket receives updates keyed by job_id
- ✅ Job logs show status transitions
- ✅ Overall status calculated correctly

### User Experience
- UI "Progress" column updates in real-time
- No page refresh required
- Status changes visible immediately

## Dependencies

### Required Files Modified
1. `internal/interfaces/event_service.go` - Add event type
2. `internal/jobs/manager.go` - Add EventService, publish events
3. `internal/jobs/processor/parent_job_executor.go` - Subscribe to events
4. `internal/handlers/websocket.go` - Subscribe to progress events
5. `internal/app/app.go` - Update Manager initialization

### No New Dependencies
All required packages already imported

## Timeline Estimate

- **Step 1**: 15 minutes (add event type)
- **Step 2**: 45 minutes (modify Manager)
- **Step 3**: 15 minutes (add EventService to Manager)
- **Step 4**: 90 minutes (subscribe in ParentJobExecutor)
- **Step 5**: 30 minutes (WebSocket subscription)
- **Step 6**: 10 minutes (app initialization)
- **Step 7**: 15 minutes (logging)
- **Step 8**: 10 minutes (optional polling reduction)

**Total**: ~3.5 hours

## Next Steps for Agent 2 (Implementer)

1. Read this plan thoroughly
2. Implement steps 1-7 in order (skip step 8 initially)
3. Run unit tests after each step
4. Create validation tests in `test/api/` or `test/ui/`
5. Document any deviations from plan
6. Report to Agent 3 for validation

## Notes

- **Breaking Changes Acceptable**: User confirmed migration not required
- **UI Updates NOT Included**: Backend only (UI will follow separately)
- **Event-Driven Architecture**: Aligns with existing event system patterns
- **Existing Infrastructure**: Leverages EventService, WebSocket, and job system

## References

### Key Files Analyzed
- `C:\development\quaero\internal\handlers\websocket.go` (lines 176-240)
- `C:\development\quaero\internal\jobs\processor\parent_job_executor.go` (full file)
- `C:\development\quaero\internal\jobs\manager.go` (lines 471-495, 1543-1574)
- `C:\development\quaero\internal\services\events\event_service.go` (full file)

### Screenshot Analysis
**File**: `C:/Users/bobmc/Pictures/Screenshots/ksnip_20251108-173620.png`
**Observed**:
- Job name: "News Crawler"
- Status badge: "Orchestrating" (blue)
- Progress text: "66 pending, 1 running, 41 completed"
- UI shows real-time progress expectation

**Implementation Matches Requirements**: ✅

---

**Plan Complete** - Ready for Agent 2 Implementation
