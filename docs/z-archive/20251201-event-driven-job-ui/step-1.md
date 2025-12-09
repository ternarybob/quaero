# Step 1: Unified Event Logging Implementation

## Tasks Completed
- **Task 1**: Add unified event logging to all workers
- **Task 3**: Add WebSocket job log subscription

## Changes Made

### 1. Added EventJobLog Event Type
**File**: `internal/interfaces/event_service.go`

Added new unified event type for all job types:
```go
EventJobLog EventType = "job_log"
```

Payload structure:
- `job_id`: string - ID of the job that logged the event
- `parent_job_id`: string - parent job ID for aggregation
- `level`: string - log level (debug, info, warn, error)
- `message`: string - log message
- `step_name`: string - optional step name
- `source_type`: string - worker type (agent, places_search, web_search)
- `metadata`: map - optional additional context
- `timestamp`: string - RFC3339 formatted timestamp

### 2. Added AddJobLogWithEvent Method to Manager
**File**: `internal/queue/manager.go`

Added `JobLogOptions` struct and `AddJobLogWithEvent` method:
- Stores log entry in job_logs table for the job
- Also stores to parent job if parent_job_id differs
- Publishes EventJobLog event for real-time WebSocket streaming

```go
type JobLogOptions struct {
    ParentJobID string
    StepName    string
    SourceType  string
    Metadata    map[string]interface{}
}

func (m *Manager) AddJobLogWithEvent(ctx context.Context, jobID, level, message string, opts *JobLogOptions) error
```

### 3. Updated PlacesWorker
**File**: `internal/queue/workers/places_worker.go`

- Added `jobMgr *queue.Manager` field
- Added `logJobEvent` helper method using unified system
- Logs step start and completion events

### 4. Updated WebSearchWorker
**File**: `internal/queue/workers/web_search_worker.go`

- Added `jobMgr *queue.Manager` field
- Added `logJobEvent` helper method using unified system
- Logs step start, errors, and completion events

### 5. Updated AgentWorker
**File**: `internal/queue/workers/agent_worker.go`

- Refactored `publishAgentJobLog` to use unified `AddJobLogWithEvent`
- Now stores logs AND publishes events (previously only published events)
- Uses "agent" as source_type

### 6. Updated App Initialization
**File**: `internal/app/app.go`

- Updated `NewManager` call to pass logger
- Updated `NewPlacesWorker` call to pass jobMgr
- Updated `NewWebSearchWorker` call to pass jobMgr

### 7. Added WebSocket Subscription for EventJobLog
**File**: `internal/handlers/websocket.go`

Added subscription to `interfaces.EventJobLog`:
- Broadcasts `job_log` messages to all connected clients
- Includes all payload fields for client-side filtering
- Respects allowedEvents whitelist if configured

## Verification
- [x] Build compiles without errors: `go build ./...`
- [x] All workers log events with consistent format
- [x] Events include step_name and source_type metadata
- [x] WebSocket broadcasts job_log messages

## Files Modified
1. `internal/interfaces/event_service.go` - Added EventJobLog
2. `internal/queue/manager.go` - Added AddJobLogWithEvent
3. `internal/queue/workers/places_worker.go` - Added logJobEvent
4. `internal/queue/workers/web_search_worker.go` - Added logJobEvent
5. `internal/queue/workers/agent_worker.go` - Updated publishAgentJobLog
6. `internal/app/app.go` - Updated worker initialization
7. `internal/handlers/websocket.go` - Added EventJobLog subscription
