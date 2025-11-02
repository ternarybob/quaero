I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The log file analysis reveals **SQLITE_BUSY (database is locked)** errors occurring during concurrent job processing with 3 workers. The errors manifest in two critical areas:

1. **Child job creation failures** - `SaveJob()` calls fail when multiple workers attempt simultaneous database writes
2. **Queue message deletion failures** - `Delete()` operations fail after job completion

The root cause is **SQLite write contention** from concurrent worker operations. Current mitigations (WAL mode, 5-second busy timeout, mutex locks) are insufficient under high concurrency.

**Key Findings:**
- Worker staggering already implemented (lines 97-102 in `worker.go`) but spread is too narrow with 3 workers
- Default busy timeout is 5000ms (5 seconds) - needs increase
- No retry logic exists for transient SQLITE_BUSY errors
- SaveJob() is called from 10+ locations across the codebase


### Approach

Implement a **multi-layered defense strategy** against database lock contention:

1. **Reduce concurrency pressure** - Lower worker count from 3 to 2
2. **Increase lock tolerance** - Double busy timeout to 10 seconds
3. **Add retry resilience** - Exponential backoff for transient lock errors
4. **Improve worker distribution** - Verify staggered startup timing

This approach balances **immediate relief** (concurrency reduction) with **long-term resilience** (retry logic) while maintaining system throughput.


### Reasoning

Analyzed the log file to identify SQLITE_BUSY error patterns, then explored the relevant source files (`job_storage.go`, `worker.go`, `config.go`, `connection.go`, `base.go`) to understand the database operation flow, worker pool architecture, and configuration defaults. Searched for existing retry patterns and identified all SaveJob() call sites to ensure comprehensive coverage.


## Mermaid Diagram

sequenceDiagram
    participant W1 as Worker 1
    participant W2 as Worker 2
    participant Q as Queue (goqite)
    participant DB as SQLite DB
    participant JS as JobStorage

    Note over W1,W2: Staggered Startup (0ms, 500ms)
    
    W1->>Q: Receive() message
    W2->>Q: Receive() message (500ms later)
    
    W1->>JS: SaveJob() - Create child job
    activate JS
    JS->>DB: ExecContext() - Write attempt 1
    DB-->>JS: SQLITE_BUSY (locked)
    Note over JS: Retry with 100ms backoff
    JS->>DB: ExecContext() - Write attempt 2
    DB-->>JS: Success
    deactivate JS
    
    W2->>JS: SaveJob() - Create child job
    activate JS
    JS->>DB: ExecContext() - Write attempt 1
    DB-->>JS: Success (less contention)
    deactivate JS
    
    W1->>Q: Delete() message
    Q->>DB: Delete from goqite table
    DB-->>Q: SQLITE_BUSY (locked)
    Note over W1: Retry with 200ms backoff
    W1->>Q: Delete() message - attempt 2
    Q->>DB: Delete from goqite table
    DB-->>Q: Success
    
    Note over W1,W2: Reduced concurrency (2 workers)<br/>Increased busy timeout (10s)<br/>Exponential backoff retries<br/>= Fewer SQLITE_BUSY errors

## Proposed File Changes

### internal\storage\sqlite\job_storage.go(MODIFY)

**Add retry logic with exponential backoff to SaveJob() method:**

1. Create a private helper function `retryWithExponentialBackoff()` at the package level (after the `NewJobStorage` constructor, before `SaveJob` method):
   - Accept parameters: `ctx context.Context`, `operation func() error`, `maxAttempts int`, `initialDelay time.Duration`, `logger arbor.ILogger`
   - Implement exponential backoff: delay doubles on each retry (100ms → 200ms → 400ms → 800ms)
   - Only retry on SQLITE_BUSY errors (check error message contains "database is locked" or "SQLITE_BUSY")
   - Log retry attempts with attempt number and delay duration
   - Return original error after max attempts exhausted

2. Modify `SaveJob()` method (lines 54-232):
   - Keep existing mutex lock (lines 56-57) for in-process synchronization
   - Keep all validation and serialization logic (lines 59-164) outside retry loop
   - Wrap the database write operation (lines 166-192) in retry logic:
     - Extract the `ExecContext` call and error handling into a closure
     - Call `retryWithExponentialBackoff()` with: 5 max attempts, 100ms initial delay
     - Keep the existing error logging (line 190) but enhance with retry context
   - Keep validation logic (lines 194-229) after successful write

**Rationale:** Centralizing retry logic in SaveJob() benefits all 10+ call sites automatically. Exponential backoff prevents thundering herd while allowing quick recovery from transient locks.

### internal\common\config.go(MODIFY)

**Increase SQLite busy timeout and reduce queue concurrency:**

1. In `NewDefaultConfig()` function, modify the `Storage.SQLite` configuration block (lines 227-235):
   - Change `BusyTimeoutMS` from `5000` to `10000` (line 234)
   - Add inline comment explaining the increase: "10 seconds for high-concurrency job processing"

2. In `NewDefaultConfig()` function, modify the `Queue` configuration block (lines 218-224):
   - Change `Concurrency` from `3` to `2` (line 220)
   - Add inline comment explaining the reduction: "Reduced from 3 to minimize database lock contention"

**Rationale:** Doubling the busy timeout gives SQLite more time to resolve lock contention before returning SQLITE_BUSY. Reducing concurrency from 3 to 2 workers decreases simultaneous database write attempts by 33%, directly reducing contention pressure. These are conservative, production-safe changes.

### internal\queue\worker.go(MODIFY)

References: 

- internal\queue\manager.go

**Add retry logic for queue message deletion and verify staggered startup:**

1. In `worker()` function (lines 96-135):
   - Review the existing stagger calculation (lines 97-102)
   - Verify the stagger delay formula: `(PollInterval / Concurrency) * workerID`
   - With 2 workers and 1s poll interval, stagger should be 0ms and 500ms
   - Add debug logging to confirm actual stagger delays on startup (already present at lines 104-107)

2. In `processMessage()` function (lines 137-240):
   - Locate the first `Delete()` call for invalid messages (line 158)
   - Wrap in retry logic: Create inline retry loop with 3 attempts, 200ms exponential backoff
   - Check error message for "database is locked" or "SQLITE_BUSY" before retrying
   - Log retry attempts at WARN level with attempt number
   
   - Locate the second `Delete()` call for unknown job types (line 182)
   - Apply same retry logic as above
   
   - Locate the third `Delete()` call after handler failure (line 208)
   - Apply same retry logic, but log at ERROR level since this is critical path
   - If all retries fail, log the failure but don't return error (message will be redelivered by goqite)
   
   - Locate the fourth `Delete()` call after successful completion (line 231)
   - Apply same retry logic as above
   - If all retries fail, log at ERROR level and return error to trigger message redelivery

**Rationale:** Queue message deletion failures leave messages in the queue, causing duplicate job execution. Retry logic with exponential backoff handles transient SQLITE_BUSY errors gracefully. The existing stagger implementation is correct but benefits from reduced concurrency (2 workers = better distribution).

### internal\jobs\types\base.go(MODIFY)

References: 

- internal\services\jobs\manager.go
- internal\storage\sqlite\job_storage.go(MODIFY)

**Add retry logic to CreateChildJobRecord() method:**

1. In `CreateChildJobRecord()` method (lines 151-188):
   - Locate the `jobManager.UpdateJob()` call (line 174) which internally calls `SaveJob()`
   - Wrap this call in retry logic similar to the pattern used in `job_storage.go`:
     - Implement inline retry loop with 5 attempts, 100ms initial exponential backoff
     - Check error message for "database is locked" or "SQLITE_BUSY" before retrying
     - Use `b.logger.Warn()` to log retry attempts with attempt number and delay
     - Keep existing error logging (line 175) but enhance with retry context
   - Keep the success logging (lines 179-186) after successful persistence

**Rationale:** Child job creation is a critical operation that frequently fails under high concurrency (as seen in the log file). Adding retry logic here provides an additional safety layer beyond the SaveJob() retry, since this is a high-frequency operation during URL discovery. The retry logic complements the centralized SaveJob() retry for maximum resilience.