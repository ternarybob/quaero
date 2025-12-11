# Plan: Job Logging Architecture Refactor
Type: feature | Workdir: ./docs/feature/20251211-job-logging-architecture/

## User Intent (from manifest)
Simplify and clean up the job logging, monitoring, and UI display architecture by:
1. Workers log simple messages with key/value context (job_id, step_id, worker_id)
2. Job monitor maintains job status with steps and operational metadata
3. API assembles job status + logs into structured JSON for UI
4. UI renders job status/logs from JSON, triggered by WebSocket events
5. Clear separation of concerns between Worker, Logging, Monitor, WebSocket, and UI

## Active Skills
go, frontend

## Current State Analysis

### Architecture Issues Identified:
1. **Backend drift**: Job handler has complex tree assembly logic (1375-1597 lines) with UI-specific concerns like `Expanded` boolean
2. **UI complexity**: queue.html has client-side expansion logic, status computation, and multiple state objects for tree management
3. **Blurred boundaries**: JobManager handles both CRUD and logging; tree endpoint does status assembly AND log fetching
4. **Screenshot issue**: UI uses JavaScript to determine which steps to expand based on status/logs - this should come from backend

### Desired Separation of Concerns:
```
┌─────────────────────────────────────────────────────────────┐
│ WORKER                                                       │
│ - Execute work                                               │
│ - Log with context: logger.Info(msg, "job_id", x, "step_id", y) │
│ - NO status management (just log completion/errors)          │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│ LOGGING (LogStorage)                                         │
│ - Store log entries with indexed context                     │
│ - Query logs by job_id, step_id, level                      │
│ - NO business logic, just persistence                        │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│ MONITOR (JobManager + existing metadata)                     │
│ - Track job status, step progress, child counts              │
│ - Maintain step_stats, current_step in metadata              │
│ - Publish status change events                               │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│ API (job_handler tree endpoint)                              │
│ - Assemble complete view: status + logs + expansion state    │
│ - Backend decides which steps should be expanded             │
│ - Return JSON ready for direct rendering                     │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│ WEBSOCKET                                                    │
│ - Notify UI of changes (status, new logs)                   │
│ - UI fetches fresh tree data on notification                │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│ UI (queue.html)                                              │
│ - Render tree from JSON (no status computation)             │
│ - Expansion state from backend `expanded` field             │
│ - User can override expand/collapse (local state only)      │
└─────────────────────────────────────────────────────────────┘
```

## Tasks
| # | Desc | Depends | Critical | Model | Skill |
|---|------|---------|----------|-------|-------|
| 1 | Audit and document current expansion logic issues | - | no | sonnet | - |
| 2 | Enhance tree API to include backend-driven expansion state | 1 | no | sonnet | go |
| 3 | Simplify UI to render tree from backend JSON structure | 2 | no | sonnet | frontend |
| 4 | Remove redundant client-side expansion computation | 3 | no | sonnet | frontend |
| 5 | Verify build and test | 4 | no | sonnet | go |

## Order
[1] → [2] → [3] → [4] → [5]

## Key Design Decisions

### Decision 1: Backend-Driven Expansion
The tree API response will include an `expanded` field for each step that the backend computes based on:
- Step has logs AND is running → expanded = true
- Step has logs AND is failed → expanded = true
- Step has logs AND is the current step → expanded = true
- Otherwise → expanded = false (user can toggle locally)

### Decision 2: Preserve User Toggle State
- Backend provides initial `expanded` state
- UI tracks user overrides in local Alpine state
- On tree refresh, respect user's explicit collapse (if they collapsed, don't re-expand)

### Decision 3: Minimal UI Changes
- Keep existing Alpine.js structure
- Remove/simplify auto-expand JavaScript logic
- Use backend `expanded` field as initial state
