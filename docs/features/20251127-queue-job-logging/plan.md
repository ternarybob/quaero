# Plan: Add Info-Level Start/End Logging for Queued Jobs

## Analysis
The queue job system currently lacks proper Info-level logging at the start and end of job execution. According to the logging standardization rules:
- **Info**: Significant updates/summaries, process start/end only
- **Debug**: Interim updates to processes
- **Trace**: Detailed process tracing

Currently, the `job_processor.go` only logs at Trace level for individual job processing, and the workers (`crawler_worker.go`, `agent_worker.go`) log at Debug/Trace but lack **Info-level summaries** at job start/end.

### Current State:
- `job_processor.go:142` - Trace: "Processing job from queue"
- `job_processor.go:235` - Trace: "Job execution completed successfully"
- `crawler_worker.go` - Debug/Trace throughout, no Info summary
- `agent_worker.go` - Debug/Trace throughout, no Info summary

### Required Changes:
Add Info-level logs for:
1. Job start: "Job started: {job_type} {job_id}"
2. Job end (success): "Job completed: {job_type} {job_id} in {duration}"
3. Job end (failure): Already at Error level (correct)

## Dependency Graph
```
[1: job_processor.go] ──┐
                        │
[2: crawler_worker.go] ─┼─ (can run concurrently)
                        │
[3: agent_worker.go] ───┘
        ↓
[4: Build verification]
```

## Execution Groups

### Group 1: Concurrent (Worker Updates)
Can run in parallel - no interdependencies.

| Task | Description | Depends | Critical |
|------|-------------|---------|----------|
| 1 | Add Info logs to job_processor.go | none | no |
| 2 | Add Info summary logs to crawler_worker.go | none | no |
| 3 | Add Info summary logs to agent_worker.go | none | no |

### Group 2: Sequential (Verification)
Requires all worker updates complete.

| Task | Description | Depends | Critical |
|------|-------------|---------|----------|
| 4 | Build verification | 1,2,3 | no |

## Execution Order
```
Concurrent: [1] [2] [3]  ← can run in parallel
Sequential: [4] → [Final Review]
```

## Success Criteria
- All queued jobs log at Info level when they start
- All queued jobs log at Info level when they complete (with duration)
- Failed jobs continue to log at Error level (already correct)
- Build passes successfully
