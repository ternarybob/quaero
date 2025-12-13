# Concurrency Fix Plan

## Problem Statement
The `concurrency = 5` setting in job definitions (e.g., `bin/job-definitions/news-crawler.toml`) does not work. Only 1 child job runs at a time, as shown in the screenshot where Progress shows "64 pending, 1 running, 40 completed, 0 failed" despite `concurrency = 5` in the config.

## Root Cause Analysis

### Finding: Global Limitation in JobProcessor

The issue is that **the JobProcessor starts only a SINGLE goroutine** to process jobs from the queue, regardless of any concurrency settings.

**Evidence from `internal/queue/workers/job_processor.go:76-78`:**
```go
// Start a single goroutine to process jobs
jp.wg.Add(1)
go jp.processJobs()
```

### Confusion Between Two Different "Concurrency" Settings

There are TWO separate concurrency configurations that do different things:

1. **`Queue.Concurrency` (config.go:175)** - Global queue workers setting
   - Default: `2`
   - Purpose: Number of concurrent job processing goroutines
   - Location: `internal/common/config.go:44-45`
   - **NOT CURRENTLY USED** by JobProcessor

2. **`steps.config.concurrency` (job definition)** - Per-job crawler concurrency
   - Default: `5` (in crawler_manager.go:158)
   - Purpose: Number of concurrent HTTP requests when crawling URLs within a single job
   - Location: `internal/queue/managers/crawler_manager.go:179-182`
   - **WORKS CORRECTLY** for HTTP request parallelism

### Why Only 1 Child Job Runs at a Time

1. The JobProcessor.Start() method starts exactly ONE processing goroutine
2. This single goroutine processes jobs sequentially in a loop
3. The `Queue.Concurrency = 2` setting exists but is **never passed to or used by** the JobProcessor
4. The job definition's `concurrency = 5` only affects HTTP request parallelism within a crawler job, NOT the number of child jobs processed in parallel

## Solution

### Option 1: Use Global Queue.Concurrency (Recommended)
Modify JobProcessor to accept a concurrency parameter and start multiple processing goroutines based on `Config.Queue.Concurrency`.

**Changes Required:**

1. **Update `NewJobProcessor` signature** to accept concurrency:
   ```go
   func NewJobProcessor(queueMgr interfaces.QueueManager, jobMgr *queue.Manager, logger arbor.ILogger, concurrency int) *JobProcessor
   ```

2. **Update `JobProcessor.Start()` method** to start multiple goroutines:
   ```go
   func (jp *JobProcessor) Start() {
       // ...
       for i := 0; i < jp.concurrency; i++ {
           jp.wg.Add(1)
           go jp.processJobs(i)  // Pass worker ID for logging
       }
   }
   ```

3. **Update `app.go`** to pass config value:
   ```go
   jobProcessor := workers.NewJobProcessor(queueMgr, jobMgr, a.Logger, a.Config.Queue.Concurrency)
   ```

### Files to Modify

1. `internal/queue/workers/job_processor.go`
   - Add `concurrency` field to struct
   - Update `NewJobProcessor()` constructor
   - Update `Start()` to spawn multiple goroutines
   - Update `processJobs()` to accept worker ID for logging

2. `internal/app/app.go`
   - Pass `a.Config.Queue.Concurrency` to NewJobProcessor

### Configuration

- Default: `Queue.Concurrency = 2` (in config.go)
- Users can override via TOML config:
  ```toml
  [queue]
  concurrency = 5
  ```

### Test Plan

1. Create a new test `TestJobConcurrency` that:
   - Creates multiple child jobs (e.g., 10 jobs)
   - Verifies multiple jobs run in parallel
   - Checks that the number of concurrent jobs matches config

2. Test scenarios:
   - Default concurrency (2) - should run 2 jobs in parallel
   - Custom concurrency (5) - should run 5 jobs in parallel
   - Concurrency = 1 - should run jobs sequentially

## Implementation Steps

1. [x] Update `internal/queue/workers/job_processor.go`:
   - Add `concurrency int` field to JobProcessor struct
   - Update NewJobProcessor to accept concurrency parameter
   - Modify Start() to spawn multiple processing goroutines
   - Add worker ID to processJobs for logging

2. [x] Update `internal/app/app.go`:
   - Pass `a.Config.Queue.Concurrency` to NewJobProcessor

3. [x] Create test `internal/queue/workers/job_processor_test.go`:
   - TestJobProcessorConcurrencyField: Tests concurrency field is set correctly
   - TestJobProcessorStartsMultipleGoroutines: Tests goroutines are spawned
   - TestJobProcessorStartStop: Tests start/stop lifecycle
   - TestJobProcessorConcurrentJobExecution: Tests concurrent receivers up to 5

4. [x] Validate:
   - Run `go build ./...` - PASSED
   - Run `go vet ./internal/queue/workers/... ./internal/app/...` - PASSED
   - Run `go test ./internal/queue/workers/...` - PASSED (all 11 tests pass)

## Expected Outcome

After the fix:
- Job Statistics should show RUNNING = N+1 (parent + N children based on concurrency)
- Progress bar should show multiple jobs running simultaneously
- The `Queue.Concurrency` config value will control parallel job execution
