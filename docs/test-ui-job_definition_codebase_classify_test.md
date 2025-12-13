# UI test: `TestJobDefinitionCodebaseClassify`

## Command

```powershell
go test .\test\ui -run TestJobDefinitionCodebaseClassify -count=1 -v
```

## Status

PASS (as of `job-20251213-125834`)

## Original Failure (Fixed)

The test **failed** (timed out at ~10 minutes) while the UI still showed the job as **running**.

Artifacts captured under:
- `test\results\ui\job-20251213-111523\TestJobDefinitionCodebaseClassify\test.log`
- `test\results\ui\job-20251213-111523\TestJobDefinitionCodebaseClassify\service.log`
- screenshots: `test\results\ui\job-20251213-111523\TestJobDefinitionCodebaseClassify\*.png`

## What actually happened

- The job definition `codebase_classify.toml` started normally: `code_map` and `import_files` completed and then the `rule_classify_files` agent step began spawning many `agent` child jobs.
- Shortly after (around `11:15:49`), the service crashed with a Go runtime fatal error.
- After the crash, the browser session kept showing the job as `running` and WebSocket `refresh_logs` messages stopped (stuck at 4), so the UI test’s monitoring loop never observed completion.

## Critical error in `service.log`

In `test\results\ui\job-20251213-111523\TestJobDefinitionCodebaseClassify\service.log`:

```
fatal error: concurrent map read and map write
...
github.com/ternarybob/arbor.(*logEvent).writeLog
...
github.com/ternarybob/quaero/internal/storage/badger.(*QueueStorage).UpdateJobStatus
  C:/development/quaero/internal/storage/badger/queue_storage.go:470
...
```

## Likely root cause

This is consistent with a **logger context mutation race**:

- `arbor`’s `WithCorrelationId()` **mutates the logger in-place** by writing into `logger.contextData` (a `map[string]string`).
- Quaero frequently does `jobLogger := w.logger.WithCorrelationId(parentID)` from multiple worker goroutines (agent/crawler/db-maintenance/monitors).
- That means multiple goroutines are **reading and writing the same `contextData` map concurrently**, and `arbor.(*logEvent).writeLog()` reads `le.logger.contextData[...]` while other goroutines write to it, triggering the runtime fatal error.

Evidence from arbor implementation (module cache):
- `github.com/ternarybob/arbor@v1.4.65/logger.go`: `WithCorrelationId()` calls `l.WithContext(...)` and returns `l` (no copy).
- `github.com/ternarybob/arbor@v1.4.65/logevent.go`: `writeLog()` reads `le.logger.contextData[...]`.

## Suggested code updates (no test changes)

### 1) Stop mutating shared loggers in-place

Make `WithCorrelationId()` usage **always operate on a per-job copy** of the logger.

Options:
- **Preferred (local fix):** update all call sites to do:
  - `jobLogger := w.logger.Copy().WithCorrelationId(parentID)`
- **Alternative (library fix):** change `arbor` so `WithCorrelationId()` performs `Copy()` internally (or make `WithContext()` copy-on-write / mutex-protected).

### 2) Fix the known hot call sites

Search results show in-tree usage that should be switched to copy-first (at minimum):
- `internal\queue\workers\agent_worker.go` (creates `jobLogger`)
- `internal\queue\workers\crawler_worker.go`
- `internal\queue\workers\database_maintenance_worker.go`
- `internal\queue\state\monitor.go`
- `internal\queue\state\step_monitor.go`

### 3) Add a focused concurrency regression test (optional but high value)

A small unit test should verify that calling `logger.WithCorrelationId(...)` concurrently does not crash and that correlation IDs don’t bleed across goroutines. (There’s already a related test: `test\unit\arbor_channel_test.go`.)

## Why this blocks the UI test

`TestJobDefinitionCodebaseClassify` depends on the service staying alive long enough to:
- run and complete the job
- keep streaming WebSocket updates (`refresh_logs`)

The logger race crashes the service mid-job, leaving the UI in a "running forever" state.

## Fixes Applied

### 1) Library fix (completed)

- Quaero updated to `github.com/ternarybob/arbor v1.4.66` which makes `With*` “fork” (no shared map mutation), eliminating the `concurrent map read and map write` crash.
- Ran `go mod tidy` to add the missing `go.sum` entry for `arbor v1.4.66` (required before tests would run).

### 2) UI fixes (implemented in Quaero)

The test then exposed UI behavior issues (steps/logs not reliably visible for fast steps). Fixes were made in `pages/queue.html`:

- Auto-expand **all** steps on tree load (avoids races where fast steps complete before WS updates are processed).
- Make `fetchStepLogs(..., immediate=true)` return a Promise and track in-flight fetches via `_stepFetchPromises`, so callers can reliably await log availability.
- On parent job terminal state, force-expand all steps and `await` immediate log fetches so assertions that run immediately after completion see log lines.

## Latest Test Run (PASS)

Command:

```powershell
go test .\test\ui -run TestJobDefinitionCodebaseClassify -count=1 -v
```

Artifacts:
- `test\results\ui\job-20251213-125834\TestJobDefinitionCodebaseClassify\test.log`
- `test\results\ui\job-20251213-125834\TestJobDefinitionCodebaseClassify\service.log`
