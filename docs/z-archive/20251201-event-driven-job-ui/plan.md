# Plan: Event-Driven Job UI with Unified Process Model
Type: feature | Workdir: ./docs/feature/20251201-event-driven-job-ui/

## Analysis

### Current State
1. **Job hierarchy**: Parent jobs with child jobs via `ParentID` pointer
2. **Event system**: EventService publishes events, WebSocket broadcasts to UI
3. **Job logs**: Stored per-job in `JobLogStorage`, API at `/api/jobs/{id}/logs`
4. **UI**: Complex nested structure with step rows, child job rows, multiple expansion levels
5. **Workers**: Each worker type handles execution differently, inconsistent event publishing

### Target State (per prompt)
1. **2-level hierarchy only**: Parent + Jobs (no deeper nesting in UI)
2. **Event-driven UI**: Log/event display under each job, driven by WebSocket
3. **Unified process**: All job types publish events/logs to parent
4. **Real-time**: WebSocket during execution, poll after completion

### Gap Analysis
1. **Backend**: Workers need to consistently publish events to parent job logs
2. **API**: Need endpoint for aggregated parent+child logs with streaming
3. **WebSocket**: Need job-specific log subscription (filter by job_id)
4. **UI**: Simplify from multi-level expansion to flat log display per job

## Tasks
| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Add job event logging to all workers (consistent format) | - | no | sonnet |
| 2 | Create aggregated logs API endpoint for parent jobs | 1 | no | sonnet |
| 3 | Add WebSocket job log subscription (real-time per job) | 2 | no | sonnet |
| 4 | Simplify UI to event/log display model | 3 | yes:architectural-change | opus |
| 5 | Update tests for new UI model | 4 | no | sonnet |

## Order
[1] → [2] → [3] → [4] → [5]

## Key Design Decisions

### Event Format (Task 1)
All workers will use consistent log format:
```
{timestamp} [{level}] {message}
```
With metadata:
- `job_id`: The child job ID
- `parent_id`: The parent job ID (for aggregation)
- `step_name`: The step that created this job
- `event_type`: status_change | progress | document_created | error

### UI Simplification (Task 4)
From:
```
Parent Job Card
├── Step 1 Row [expand]
│   └── Child jobs...
└── Step 2 Row [expand]
    └── Child jobs...
```

To:
```
Parent Job Card [expand]
├── Header: "2 Jobs scheduled"
│   - Step 1: search_nearby_restaurants (places_search)
│   - Step 2: extract_keywords (agent)
├── Status: Running
└── Events (live stream):
    - [10:30:01] Step 1 started
    - [10:30:02] Searched for restaurants near Wheelers Hill
    - [10:30:03] Created 20 documents
    - [10:30:04] Step 1 completed
    - [10:30:05] Step 2 started
    - [10:30:06] Processing document doc_xxx
    - [10:30:07] Extracted keywords for doc_xxx
    ...
```

### WebSocket Protocol (Task 3)
Client subscribes with: `{"type": "subscribe_job_logs", "job_id": "xxx"}`
Server sends: `{"type": "job_log", "job_id": "xxx", "entry": {...}}`
