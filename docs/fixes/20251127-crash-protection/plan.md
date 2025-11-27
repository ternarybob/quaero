# Plan: Service Crash Protection and Fatal Error Handling

## Analysis

### Problem Summary
The Quaero service (`./bin`) crashes silently during the News Crawler job execution while the test (`test/ui/queue_test.go -> TestNewsCrawlerCrash`) does not reproduce the crash after 10 minutes.

### Evidence from Logs
1. **Log file**: `bin/logs/quaero.2025-11-27T16-00-24.log`
2. **Last entry**: `time=16:05:22 level=INF message="Job completed"` - log ends abruptly
3. **No panic, no error, no shutdown message** - the process simply terminated
4. **Screenshot shows**: "OFFLINE" status with 62 documents processed, 42 pending, 1 running

### Key Observations
1. The service processes 62+ jobs successfully before crashing
2. The crash happens **between** "Job completed" and "Job started" (no "Job started" log for next job)
3. No panic is logged despite having panic recovery in `job_processor.go:94-107`
4. The test environment doesn't crash because it uses different configuration

### Configuration Differences

| Setting | bin/quaero.toml | test/bin/quaero.toml |
|---------|-----------------|---------------------|
| Port | 8085 | 18085 |
| Storage path | (default) | ./data/quaero.badger |
| reset_on_startup | true | true |
| Logging output | (default) | ['file', 'console'] |
| News Crawler start_urls | 2 URLs | 1 URL |
| News Crawler max_pages | 50 | 50 |

### Root Cause Hypothesis
The crash is likely occurring in one of these areas:
1. **Badger database operation** - storage corruption or compaction issue
2. **ChromeDP browser pool** - resource exhaustion or browser process crash
3. **Memory pressure** - OOM killer or memory leak accumulation
4. **Goroutine panic** in a non-recovered location (event publisher goroutines)

### Current Panic Recovery Gaps
1. `processJobs()` has panic recovery but only logs to Fatal (may not flush)
2. Event publisher goroutines (`publishCrawlerJobLog`, etc.) run async without recovery
3. No process-level crash protection (auto-restart, core dump)
4. No health monitoring or watchdog

## Dependency Graph
```
[1: Investigate crash location]
    ↓
[2: Add process-level crash protection] ──────┐
    ↓                                          │
[3: Enhance panic recovery logging] ───────────┤  (2,3,4 can run concurrently after 1)
    ↓                                          │
[4: Add goroutine panic wrappers] ─────────────┘
    ↓
[5: Integration testing] (requires 2,3,4)
```

## Execution Groups

### Group 1: Sequential (Investigation)
Must complete before implementation work.

| Task | Description | Depends | Critical | Complexity | Model |
|------|-------------|---------|----------|------------|-------|
| 1 | Investigate crash location with detailed logging | none | no | medium | Sonnet |

### Group 2: Concurrent (Implementation)
Can run in parallel after Group 1 completes.

| Task | Description | Depends | Critical | Complexity | Model |
|------|-------------|---------|----------|------------|-------|
| 2 | Add process-level crash protection | 1 | no | medium | Sonnet |
| 3 | Enhance panic recovery with crash file logging | 1 | no | medium | Sonnet |
| 4 | Add panic wrappers to async goroutines | 1 | no | low | Sonnet |

### Group 3: Sequential (Validation)
Requires all implementation tasks complete.

| Task | Description | Depends | Critical | Complexity | Model |
|------|-------------|---------|----------|------------|-------|
| 5 | Validate crash protection and run tests | 2,3,4 | no | medium | Sonnet |

## Execution Order
```
Sequential: [1]
Concurrent: [2] [3] [4]  ← can run in parallel
Sequential: [5] → [Final Review]
```

## Success Criteria
- Panic anywhere in the service gets logged to crash file before process exit
- Async goroutine panics are caught and logged without crashing the service
- Process-level crash protection creates diagnostic file on fatal crash
- TestNewsCrawlerCrash test passes
- Service can handle 100+ crawler jobs without crashing
