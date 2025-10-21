# Implementation Plan for Verification Comments 3-5

## Overview
This document outlines the detailed implementation plan for the remaining three verification comments related to async polling improvements in the job executor.

---

## Comment 3: Defer Job Completion Event Until Polling Finishes

### Current Problem
- `Execute()` publishes "completed" event at the end of execution
- Background polling goroutine is still running and processing crawl jobs
- Job appears "completed" in events/UI while actual work is ongoing
- Race condition: polling goroutine also publishes "completed" when done

### Desired Behavior
- Don't publish final job-level "completed" event if background polling is running
- Let polling goroutine publish final completion event when all crawl jobs finish
- Clear event semantics: "completed" means ALL work is done (including async polling)

### Implementation Approach

#### Option A: Track Background Tasks with Flag (Recommended)
```go
// In Execute() method
var hasAsyncPolling bool

// When launching polling goroutine:
if step.Action == "crawl" && extractBool(step.Config, "wait_for_completion", true) {
    hasAsyncPolling = true
    go func() {
        // ... polling logic ...
        // Publish final completion here
        e.publishProgressEvent(pollingCtx, definition, len(definition.Steps)-1, "", "", "completed", "")
    }()
}

// At end of Execute():
if !hasAsyncPolling {
    // Only publish completion if no async polling launched
    e.publishProgressEvent(ctx, definition, len(definition.Steps)-1, "", "", "completed", "")
}
```

**Pros:**
- Simple flag-based approach
- Minimal changes to existing code
- Clear ownership: polling goroutine owns final completion

**Cons:**
- Only works for single crawl step per job
- Need to adjust for multiple crawl steps

#### Option B: WaitGroup-based Tracking
```go
// Add to JobExecutor struct
type JobExecutor struct {
    // ... existing fields ...
    pendingPollers sync.WaitGroup
}

// When launching polling:
e.pendingPollers.Add(1)
go func() {
    defer e.pendingPollers.Done()
    // ... polling logic ...
}()

// At end of Execute():
// Wait with timeout for pollers to complete
done := make(chan struct{})
go func() {
    e.pendingPollers.Wait()
    close(done)
}()

select {
case <-done:
    // All pollers finished
    e.publishProgressEvent(ctx, definition, len(definition.Steps)-1, "", "", "completed", "")
case <-time.After(1 * time.Second):
    // Still running - skip final completion event
    e.logger.Info().Msg("Async polling in progress, deferring completion event")
}
```

**Pros:**
- Handles multiple async polling tasks
- More robust tracking mechanism
- Can wait for completion with timeout

**Cons:**
- More complex implementation
- Potential blocking in Execute() (though with timeout)
- Shared state across executions

### Recommended Implementation: Option A with Enhancement

**Step-by-Step Plan:**

1. **Add tracking field to Execute() scope**
   ```go
   // Track if any async polling was launched for this execution
   asyncPollingLaunched := false
   ```

2. **Set flag when launching polling goroutine**
   ```go
   if step.Action == "crawl" && extractBool(step.Config, "wait_for_completion", true) {
       if len(jobIDs) > 0 {
           asyncPollingLaunched = true
           // ... launch goroutine ...
       }
   }
   ```

3. **Let polling goroutine publish final completion**
   ```go
   go func() {
       // ... polling logic ...
       if pollErr != nil {
           e.publishProgressEvent(pollingCtx, definition, len(definition.Steps)-1, "", "", "failed", pollErr.Error())
       } else {
           e.publishProgressEvent(pollingCtx, definition, len(definition.Steps)-1, "", "", "completed", "")
       }
   }()
   ```

4. **Skip final completion in Execute() if polling launched**
   ```go
   // At end of Execute(), before return
   if !asyncPollingLaunched {
       // Publish completion event
       e.publishProgressEvent(ctx, definition, len(definition.Steps)-1, "", "", "completed", "")
   } else {
       e.logger.Info().
           Str("job_id", definition.ID).
           Msg("Async polling in progress - completion event deferred to polling goroutine")
   }
   ```

5. **Handle error cases in polling goroutine**
   - On timeout: publish "failed" with timeout message
   - On context cancel: publish "cancelled"
   - On job failures: respect ErrorStrategy (fail vs continue)

**Files to Modify:**
- `internal/services/jobs/executor.go` - Execute() method and pollCrawlJobs goroutine launch

---

## Comment 4: Replace context.Background with Cancellable Context

### Current Problem
- Polling goroutine uses `context.WithTimeout(context.Background(), timeout)`
- No parent context from executor
- Can't cancel polling if:
  - Executor shuts down
  - Service stops
  - Job is manually cancelled

### Desired Behavior
- Executor maintains a cancellable context
- All polling derives from executor context
- Shutdown/stop cancels all ongoing polling

### Implementation Approach

#### Add Executor-Level Context Management

**Step-by-Step Plan:**

1. **Add context fields to JobExecutor struct**
   ```go
   type JobExecutor struct {
       registry       *JobTypeRegistry
       sourceService  *sources.Service
       eventService   interfaces.EventService
       crawlerService interface{ GetJobStatus(jobID string) (interface{}, error) }
       logger         arbor.ILogger

       // Context for lifecycle management
       ctx    context.Context
       cancel context.CancelFunc
   }
   ```

2. **Initialize context in NewJobExecutor**
   ```go
   func NewJobExecutor(...) (*JobExecutor, error) {
       // ... validation ...

       ctx, cancel := context.WithCancel(context.Background())

       executor := &JobExecutor{
           // ... existing fields ...
           ctx:    ctx,
           cancel: cancel,
       }

       return executor, nil
   }
   ```

3. **Add Shutdown method**
   ```go
   // Shutdown gracefully stops the executor and cancels all background tasks
   func (e *JobExecutor) Shutdown() {
       e.logger.Info().Msg("Shutting down job executor - cancelling background tasks")
       e.cancel()
   }
   ```

4. **Use executor context as parent for polling**
   ```go
   // In Execute() when launching polling goroutine:
   go func() {
       // Derive timeout context from executor context (not Background)
       pollingCtx, cancel := context.WithTimeout(e.ctx, pollingTimeout)
       defer cancel()

       // Poll until all jobs complete
       pollErr := e.pollCrawlJobs(pollingCtx, definition, stepIndex, step, jobIDs)
       // ... handle errors ...
   }()
   ```

5. **Update app.go to call Shutdown on cleanup**
   ```go
   // In app.go shutdown logic:
   if a.JobExecutor != nil {
       a.JobExecutor.Shutdown()
   }
   ```

6. **Handle cancellation in pollCrawlJobs**
   ```go
   // Already handled via ctx.Done() check in select statement
   case <-ctx.Done():
       if ctx.Err() == context.DeadlineExceeded {
           return fmt.Errorf("polling timeout exceeded")
       }
       if ctx.Err() == context.Canceled {
           e.logger.Info().Msg("Polling cancelled via executor shutdown")
           return fmt.Errorf("polling cancelled")
       }
       return ctx.Err()
   ```

**Files to Modify:**
- `internal/services/jobs/executor.go` - Add context fields, Shutdown method, use in polling
- `internal/app/app.go` - Call executor.Shutdown() on app cleanup

**Benefits:**
- Clean shutdown of all background tasks
- Prevents goroutine leaks
- Proper context hierarchy (executor → polling)
- Can be cancelled externally via executor.Shutdown()

---

## Comment 5: Strengthen Type Safety for GetJobStatus

### Current Problem
- `crawlerService.GetJobStatus(jobID)` returns `interface{}`
- Requires type assertion to `*crawler.CrawlJob` in pollCrawlJobs
- Type assertion can fail at runtime
- No compile-time type safety
- Interface is too generic

### Desired Behavior
- Explicit interface that returns `*crawler.CrawlJob`
- No runtime type assertions
- Compile-time type safety
- Clear contract between executor and crawler service

### Implementation Approach

**Step-by-Step Plan:**

1. **Create CrawlerService interface in internal/interfaces/**
   ```go
   // internal/interfaces/crawler_service.go
   package interfaces

   import "github.com/ternarybob/quaero/internal/services/crawler"

   // CrawlerService provides access to crawler job status and management
   type CrawlerService interface {
       // GetJobStatus retrieves the current status of a crawl job
       GetJobStatus(jobID string) (*crawler.CrawlJob, error)
   }
   ```

2. **Update JobExecutor struct**
   ```go
   // internal/services/jobs/executor.go
   type JobExecutor struct {
       registry       *JobTypeRegistry
       sourceService  *sources.Service
       eventService   interfaces.EventService
       crawlerService interfaces.CrawlerService  // Changed from anonymous interface
       logger         arbor.ILogger
       ctx            context.Context
       cancel         context.CancelFunc
   }
   ```

3. **Update NewJobExecutor signature**
   ```go
   func NewJobExecutor(
       registry *JobTypeRegistry,
       sourceService *sources.Service,
       eventService interfaces.EventService,
       crawlerService interfaces.CrawlerService,  // Changed type
       logger arbor.ILogger,
   ) (*JobExecutor, error) {
       // ... validation remains same ...
   }
   ```

4. **Remove type assertion in pollCrawlJobs**
   ```go
   // Before:
   result, err := e.crawlerService.GetJobStatus(jobID)
   if err != nil { /* ... */ }

   cj, ok := result.(*crawler.CrawlJob)
   if !ok {
       e.logger.Warn().Msg("Unexpected result type")
       continue
   }

   // After:
   cj, err := e.crawlerService.GetJobStatus(jobID)
   if err != nil {
       consecutiveErrors[jobID]++
       // ... existing error handling ...
       continue
   }

   // Reset consecutive error count on success
   consecutiveErrors[jobID] = 0

   // cj is already *crawler.CrawlJob - no type assertion needed!
   statusStr := string(cj.Status)
   ```

5. **Verify crawler.Service implements interface**
   ```go
   // internal/services/crawler/service.go
   // Add compile-time assertion
   var _ interfaces.CrawlerService = (*Service)(nil)

   // Method already exists with correct signature:
   func (s *Service) GetJobStatus(jobID string) (*CrawlJob, error) {
       // ... existing implementation ...
   }
   ```

6. **Update app.go initialization**
   ```go
   // internal/app/app.go
   // No change needed - CrawlerService already returns *crawler.Service
   // which implements interfaces.CrawlerService

   a.JobExecutor, err = jobs.NewJobExecutor(
       a.JobRegistry,
       a.SourceService,
       a.EventService,
       a.CrawlerService,  // Still passes *crawler.Service, now via interface
       a.Logger,
   )
   ```

7. **Update test mocks**
   ```go
   // internal/services/jobs/executor_test.go

   // Update mockCrawlerService
   type mockCrawlerService struct {
       jobs map[string]map[string]interface{}
   }

   func (m *mockCrawlerService) GetJobStatus(jobID string) (*crawler.CrawlJob, error) {
       if job, ok := m.jobs[jobID]; ok {
           // Convert map to CrawlJob (for legacy tests)
           return &crawler.CrawlJob{
               ID:     jobID,
               Status: crawler.JobStatus(job["status"].(string)),
               // ... populate from map ...
           }, nil
       }
       return createCrawlJob(jobID, "completed", 100, 0, "jira"), nil
   }

   // Update statefulMockCrawlerService
   func (m *statefulMockCrawlerService) GetJobStatus(jobID string) (*crawler.CrawlJob, error) {
       states, ok := m.states[jobID]
       if !ok {
           return createCrawlJob(jobID, "completed", 100, 0, "jira"), nil
       }

       callCount := m.callCounts[jobID]
       m.callCounts[jobID]++

       if callCount >= len(states) {
           return states[len(states)-1].(*crawler.CrawlJob), nil
       }
       return states[callCount].(*crawler.CrawlJob), nil
   }
   ```

**Files to Create:**
- `internal/interfaces/crawler_service.go` - New CrawlerService interface

**Files to Modify:**
- `internal/services/jobs/executor.go` - Use interfaces.CrawlerService, remove type assertion
- `internal/services/jobs/executor_test.go` - Update mock implementations
- `internal/services/crawler/service.go` - Add compile-time interface assertion

**Benefits:**
- **Compile-time type safety** - errors caught at compile time, not runtime
- **No panic risk** - eliminates type assertion failure
- **Clear contract** - interface documents expected behavior
- **Better IDE support** - autocomplete knows exact return type
- **Easier testing** - mocks have well-defined interface to implement
- **Follows Go best practices** - accept interfaces, return concrete types

---

## Implementation Order

Recommended order to minimize conflicts and enable incremental testing:

### Phase 1: Type Safety (Comment 5)
1. Create `internal/interfaces/crawler_service.go`
2. Update `JobExecutor` struct and `NewJobExecutor`
3. Remove type assertion in `pollCrawlJobs`
4. Update test mocks
5. **Test:** Run unit tests, verify compilation

### Phase 2: Context Management (Comment 4)
1. Add context fields to `JobExecutor`
2. Initialize in `NewJobExecutor`
3. Add `Shutdown()` method
4. Use executor context in polling goroutine
5. Update `app.go` to call `Shutdown()`
6. **Test:** Verify graceful shutdown behavior

### Phase 3: Deferred Completion (Comment 3)
1. Add `asyncPollingLaunched` flag in `Execute()`
2. Set flag when launching polling
3. Move final completion event to polling goroutine
4. Skip completion in `Execute()` if flag set
5. **Test:** Verify completion events are published once and at correct time

---

## Testing Strategy

### Unit Tests
- Test executor with type-safe interface
- Test context cancellation propagates to polling
- Test completion event deferred when polling launched
- Test completion event published immediately when no polling

### Integration Tests
- Test shutdown cancels ongoing polling
- Test timeout handling with new context
- Test multiple jobs with mixed outcomes
- Test completion events in correct order

### Manual Testing
- Start crawl job, verify completion event timing
- Shutdown service during polling, verify cleanup
- Monitor logs for proper context cancellation messages

---

## Risk Assessment

### Low Risk
- **Comment 5 (Type Safety)**: Purely additive change, improves safety
- **Comment 4 (Context)**: Well-established pattern, minimal disruption

### Medium Risk
- **Comment 3 (Deferred Completion)**: Changes event semantics
  - **Mitigation**: Thorough testing of event order
  - **Mitigation**: Document new behavior clearly

### Potential Issues
1. **Multiple crawl steps**: Current plan assumes one crawl step per job
   - **Solution**: Track async polling per step, not per job
2. **Event consumers**: UI/clients may expect immediate completion
   - **Solution**: Document new semantics, add "running" status visibility
3. **Orphaned goroutines**: Shutdown might leave goroutines running
   - **Solution**: Proper context cancellation (Comment 4 addresses this)

---

## Rollback Plan

If issues arise after implementation:

1. **Comment 5**: Revert to `interface{}` return type (single file change)
2. **Comment 4**: Remove context fields, use `context.Background()` again
3. **Comment 3**: Re-enable final completion event in `Execute()`

Each comment is relatively independent, allowing selective rollback.

---

## Success Criteria

### Comment 3 - Deferred Completion
- [ ] No "completed" event published from Execute() when async polling launched
- [ ] Polling goroutine publishes final "completed" event
- [ ] Events appear in correct order: start → running → [progress...] → completed
- [ ] No duplicate completion events

### Comment 4 - Context Management
- [ ] Executor has context and cancel function
- [ ] Shutdown() method cancels all background tasks
- [ ] Polling inherits from executor context
- [ ] Context cancellation stops polling gracefully
- [ ] No goroutine leaks after shutdown

### Comment 5 - Type Safety
- [ ] CrawlerService interface defined in interfaces package
- [ ] JobExecutor uses typed interface
- [ ] No type assertions in pollCrawlJobs
- [ ] All tests pass with new interface
- [ ] Crawler service implements interface (compile-time verified)

---

## Estimated Implementation Time

- **Comment 5 (Type Safety)**: 30 minutes
- **Comment 4 (Context Management)**: 45 minutes
- **Comment 3 (Deferred Completion)**: 1 hour
- **Testing & Verification**: 1 hour
- **Total**: ~3 hours

---

## Next Steps

1. Review this plan for completeness
2. Confirm approach for each comment
3. Begin implementation in recommended order (5 → 4 → 3)
4. Test incrementally after each phase
5. Update async polling tests to verify new behavior
