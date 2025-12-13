# Queue Logging Architecture

> **For AI Agents:** This document describes the logging flow for queue jobs.
> Read this before modifying any logging-related code in the queue path.

## Overview

Queue logging provides real-time visibility into job execution. Logs flow from workers through the JobManager to both persistent storage and WebSocket broadcast.

## Logging Flow

```
Worker executes job
    ↓
jobMgr.AddJobLog(ctx, jobID, level, message)
    ↓
resolveJobContext(ctx, jobID) → stepName, managerID
    ↓
Create LogEntry with timestamp, level, message, step_name, originator
    ↓
┌─────────────────────────────────────────────────────────────────┐
│                    PARALLEL OPERATIONS                          │
├─────────────────────────────────────────────────────────────────┤
│  1. Store to BadgerDB (job_logs table)                          │
│  2. Publish EventJobLog to EventService                         │
└─────────────────────────────────────────────────────────────────┘
    ↓
EventService broadcasts to subscribers
    ↓
WebSocketHandler receives event
    ↓
Broadcasts to connected clients via WebSocket
    ↓
UI receives and displays log
```

## Log Entry Schema

```go
type LogEntry struct {
    Index      int       `json:"index"`      // Auto-incrementing for stable ordering
    Timestamp  time.Time `json:"timestamp"`  // Full timestamp for storage
    Level      string    `json:"level"`      // debug, info, warn, error
    Message    string    `json:"message"`    // Log message content
    StepName   string    `json:"step_name"`  // Step context (e.g., "import_files")
    Originator string    `json:"originator"` // Source: "step", "worker", or ""
}
```

## Logging Methods

### JobManager Methods

| Method | Use Case | Originator |
|--------|----------|------------|
| `AddJobLog(ctx, jobID, level, msg)` | General logging, auto-resolves context | Auto-detected |
| `AddJobLogWithOriginator(ctx, jobID, level, msg, originator)` | Explicit originator | Specified |
| `AddJobLogWithContext(ctx, jobID, level, msg, stepName, originator)` | Full context control | Specified |

### Originator Values

- `"step"` - StepMonitor logs (e.g., "Starting workers", "Step completed")
- `"worker"` - Worker execution logs (e.g., "Processing URL", "Document saved")
- `""` (empty) - System/monitor logs (e.g., "Child job X → completed")

### Context Resolution

The `resolveJobContext()` function walks the parent chain to find step context:

```go
func (m *Manager) resolveJobContext(ctx context.Context, jobID string) (stepName, managerID string) {
    job := m.GetJob(ctx, jobID)
    
    // Check job metadata for step_name
    if stepName, ok := job.Metadata["step_name"].(string); ok {
        return stepName, job.ManagerID
    }
    
    // Walk parent chain if needed
    if job.ParentID != "" {
        return m.resolveJobContext(ctx, job.ParentID)
    }
    
    return "", job.ID
}
```

## WebSocket Events

### EventJobLog

Published when a log entry is added:

```json
{
    "type": "job_log",
    "payload": {
        "job_id": "abc123",
        "manager_id": "xyz789",
        "step_name": "import_files",
        "entry": {
            "index": 42,
            "timestamp": "2025-12-12T17:30:00Z",
            "level": "info",
            "message": "Processing file: main.go",
            "step_name": "import_files",
            "originator": "worker"
        }
    }
}
```

### refresh_logs Event

Sent when UI should refetch logs (e.g., after step completion):

```json
{
    "type": "refresh_logs",
    "payload": {
        "job_id": "step123",
        "manager_id": "xyz789"
    }
}
```

## Log Retrieval API

### GET /api/jobs/{id}/logs

Fetches logs for a specific job.

**Query Parameters:**
- `limit` - Max logs to return (default: 100)
- `offset` - Pagination offset
- `level` - Filter by level (debug, info, warn, error)

### GET /api/jobs/{id}/logs/aggregated

Fetches logs for parent job and all children (merged view).

**Query Parameters:**
- `limit` - Max logs to return (default: 1000)
- `cursor` - Pagination cursor (opaque base64 string)
- `order` - Sort order: "asc" (oldest first) or "desc" (newest first)
- `include_children` - Include child job logs (default: true)

## UI Log Display

### Log Fetching Strategy

The UI uses **trigger-based fetching** rather than direct WebSocket streaming:

1. **Initial Load:** Fetch logs via REST API when step is expanded
2. **Real-time Updates:** WebSocket `job_log` events trigger incremental fetch
3. **Pagination:** "Show earlier logs" button fetches with offset

### Log Line Numbering

**CRITICAL:** Log lines MUST:
- Start at line 1 (not 0, not 5)
- Increment sequentially (1, 2, 3, ...) not jumping (5, 10, 15, ...)
- Use the `index` field from LogEntry for stable ordering

### Step Log Isolation

Each step displays only its own logs:
- Filter by `step_name` matching the step being viewed
- Or filter by `job_id` for step-specific logs

## Best Practices

### For Workers

```go
// Good: Use context-aware logging
jobMgr.AddJobLog(ctx, jobID, "info", "Processing document")

// Good: Explicit originator when needed
jobMgr.AddJobLogWithOriginator(ctx, jobID, "info", "Starting batch", "worker")

// Avoid: Logging without job context
logger.Info().Msg("Processing...")  // Won't appear in job logs
```

### For Monitors

```go
// StepMonitor should use "step" originator
jobMgr.AddJobLogWithOriginator(ctx, stepID, "info", "All workers completed", "step")

// JobMonitor should use empty originator (system)
jobMgr.AddJobLogWithOriginator(ctx, managerID, "info", "Job completed", "")
```

## Known Issues

1. **API Call Count:** UI may make too many API calls for logs (should be < 10 per step)
2. **Log Line Numbering:** Some steps may not follow 1 → N sequential pattern
3. **Context Resolution:** Walking parent chain can be expensive for deep hierarchies

## Related Documents

- **Manager/Worker Architecture:** `docs/architecture/manager_worker_architecture.md`
- **UI Architecture:** `docs/architecture/QUEUE_UI.md`
- **Workers Reference:** `docs/architecture/workers.md`

