# Plan: Service Crash Detection and Protection for News Crawler Job

## Analysis

### Problem Summary
The service is crashing silently during News Crawler job execution with no error logs, panics, or notifications. The UI freezes showing "Running" state with 51 pending, 1 running, 54 completed but the job never completes.

### Evidence from Logs
- Log file: `bin/logs/quaero.2025-11-27T12-32-30.log` (316 lines)
- Last log entry: `time=12:37:32 level=INF message="Job started" ... job_id=be561cb7-ba86-440d-a514-613b8e781372`
- No corresponding "Job completed" entry - service crashed mid-execution
- No panic, error, or fatal message logged before crash
- Screenshot shows: 3 total jobs, 1 pending, 2 running, 0 completed

### Root Cause Hypothesis
The crash occurs after the badger update, likely in one of these areas:
1. **Badger storage operations** - Silent panic during DB operations not caught by defer recover()
2. **ChromeDP browser operations** - Context cancellation or browser crash
3. **Memory exhaustion** - Badger GC or large page rendering

### Key Files Involved
- `internal/queue/workers/job_processor.go` - Has defer recover() but may not catch all crashes
- `internal/queue/workers/crawler_worker.go` - ChromeDP rendering, document storage
- `internal/storage/badger/queue_storage.go` - Badger operations that may panic
- `test/ui/queue_test.go` - Existing queue tests to extend

## Dependency Graph
```
[1: Create crash test]
    ↓
[2: Add JobProcessor protection] ←──┐
    ↓                              │ (2,3 can run concurrently after 1)
[3: Add CrawlerWorker protection] ─┘
    ↓
[4: Add BadgerDB protection]
    ↓
[5: Validation & Integration]
```

## Execution Groups

### Group 1: Sequential (Test Foundation)
Must complete before implementation work.

| Task | Description | Depends | Critical | Complexity | Model |
|------|-------------|---------|----------|------------|-------|
| 1 | Create news crawler crash detection test | none | no | medium | Sonnet |

### Group 2: Concurrent (Protection Implementation)
Can run in parallel after Group 1 completes.

| Task | Description | Depends | Critical | Complexity | Model |
|------|-------------|---------|----------|------------|-------|
| 2 | Add JobProcessor fatal protection & logging | 1 | no | medium | Sonnet |
| 3 | Add CrawlerWorker protection & logging | 1 | no | medium | Sonnet |

### Group 3: Sequential (Storage Protection)
Requires understanding of crash points from concurrent tasks.

| Task | Description | Depends | Critical | Complexity | Model |
|------|-------------|---------|----------|------------|-------|
| 4 | Add BadgerDB storage protection | 2,3 | no | medium | Sonnet |

### Group 4: Sequential (Validation)
Requires all implementation complete.

| Task | Description | Depends | Critical | Complexity | Model |
|------|-------------|---------|----------|------------|-------|
| 5 | Run tests and validate fixes | 4 | no | low | Sonnet |

## Execution Order
```
Sequential: [1]
Concurrent: [2] [3]
Sequential: [4] → [5] → [Final Review]
```

## Success Criteria
- Test `TestNewsCrawlerCrashDetection` in `test/ui/queue_test.go` passes
- Service logs fatal errors before termination instead of silent crash
- Job processor recovers from panics in storage layer
- Crawler worker handles ChromeDP context cancellation gracefully
- Badger operations have panic recovery with proper logging
