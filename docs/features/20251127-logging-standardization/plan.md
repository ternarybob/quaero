# Plan: Logging Level Standardization

## Analysis

The codebase has 106 files with logging statements. Current log level usage is inconsistent:

| Level | Current Count | Purpose (After Standardization) |
|-------|--------------|--------------------------------|
| INFO | 271 | Significant events: process start/end, summaries |
| DEBUG | 235 | Interim updates during processes |
| TRACE | 80 | Detailed process tracing |
| WARN | 254 | User warnings (nothing broken) |
| ERROR | 442 | Actual errors |

### Rules for Standardization:
1. **INF (Info)**: Significant updates/summaries, process start/end only
2. **DBG (Debug)**: Interim updates to a process
3. **VRB (Verbose/Trace)**: Detailed process tracing
4. **WRN (Warn)**: Warning to user, but nothing is broken
5. **ERR/FTL**: Actual errors/fatal conditions

### Key Issues Found:
- Many `.Info()` calls are interim progress updates that should be `.Debug()`
- Application startup has excessive Info logging - each service init is Info
- Progress-style logging ("Processing X of Y") should be Debug, not Info
- Many Debug calls are detailed tracing that should be Trace
- Some Warn calls are for errors that should use Error instead

## Dependency Graph
```
[1: Core Workers]
    |
[2: Queue System] ------+
    |                   |
[3: Services]     ------+  (2,3,4 can run concurrently after 1)
    |                   |
[4: Handlers]     ------+
    |
[5: App/Main/Storage/Common] (requires 2,3,4)
```

## Execution Groups

### Group 1: Sequential (Foundation)
Must complete before any concurrent work.

| Task | Description | Depends | Critical |
|------|-------------|---------|----------|
| 1 | Standardize queue workers (job_processor, crawler_worker, agent_worker, github_log_worker, database_maintenance_worker) | none | no |

### Group 2: Concurrent
Can run in parallel after Group 1 completes.

| Task | Description | Depends | Critical | Can Parallelize |
|------|-------------|---------|----------|-----------------|
| 2 | Standardize queue system (orchestrator, managers, state/monitor) | 1 | no | Yes |
| 3 | Standardize services (crawler, agents, scheduler, events, etc.) | 1 | no | Yes |
| 4 | Standardize handlers (job, document, websocket, auth, etc.) | 1 | no | Yes |

### Group 3: Sequential (Integration)
Requires all concurrent tasks complete.

| Task | Description | Depends | Critical |
|------|-------------|---------|----------|
| 5 | Standardize app.go, main.go, storage layer, and common utilities | 2,3,4 | no |

## Execution Order
```
Sequential: [1]
Concurrent: [2] [3] [4]  <- can run in parallel
Sequential: [5] -> [Final Review]
```

## Success Criteria
- All Info logs are significant events (start/end of major processes, summaries)
- All Debug logs are interim process updates
- All Trace logs are detailed tracing
- Warn logs indicate non-breaking issues
- Error/Fatal logs indicate actual failures
- Code compiles successfully
- Startup logs are condensed to key milestones only
