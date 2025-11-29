# Task 4: Add Panic Wrappers to Async Goroutines

## Metadata
- **ID:** 4
- **Group:** 2
- **Mode:** concurrent
- **Skill:** @go-coder
- **Complexity:** low
- **Model:** claude-sonnet-4-5-20250929
- **Critical:** no
- **Depends:** 1
- **Blocks:** 5

## Paths
```yaml
sandbox: /tmp/3agents/task-4/
source: C:/development/quaero/
output: C:/development/quaero/docs/fixes/20251127-crash-protection/
```

## Files to Modify
- `internal/queue/workers/crawler_worker.go` - Add panic wrappers to event publishers
- `internal/common/goroutine.go` - New utility for safe goroutine launching

## Requirements
Add panic recovery to all async goroutines that could crash the service:

1. **Create safe goroutine utility** (`internal/common/goroutine.go`):
   ```go
   // SafeGo runs a function in a goroutine with panic recovery
   // Panics are logged but don't crash the service
   func SafeGo(logger arbor.ILogger, name string, fn func())

   // SafeGoWithContext runs with context support
   func SafeGoWithContext(ctx context.Context, logger arbor.ILogger, name string, fn func())
   ```

2. **Wrap event publisher goroutines in crawler_worker.go**:
   - `publishCrawlerJobLog` (line 1385-1389)
   - `publishCrawlerProgressUpdate` (line 1429-1433)
   - `publishJobSpawnEvent` (line 1488-1492)

3. **Search for other `go func()` patterns**:
   - Find all `go func()` in the codebase
   - Document which ones need wrapping
   - Prioritize those in the queue/workers path

4. **Pattern for safe async publishing**:
   ```go
   // Before:
   go func() {
       if err := w.eventService.Publish(ctx, event); err != nil {
           w.logger.Warn().Err(err).Msg("Failed to publish")
       }
   }()

   // After:
   common.SafeGo(w.logger, "publishCrawlerJobLog", func() {
       if err := w.eventService.Publish(ctx, event); err != nil {
           w.logger.Warn().Err(err).Msg("Failed to publish")
       }
   })
   ```

## Acceptance Criteria
- [ ] goroutine.go utility created
- [ ] SafeGo and SafeGoWithContext implemented
- [ ] Event publisher goroutines wrapped
- [ ] Other dangerous goroutines identified
- [ ] Panics in goroutines logged not crash service
- [ ] Compiles successfully

## Context
The event publisher goroutines at lines 1385, 1429, and 1488 run without panic recovery. If any of these panic (e.g., due to nil event service or closed channel), they will crash the entire service.

## Dependencies Input
From Task 1: List of async goroutine spawn points

## Output for Dependents
- SafeGo utility for other components
- Pattern for safe async operations
