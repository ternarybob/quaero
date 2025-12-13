# Plan: Fix Document Count Display for Agent Jobs

## Problem
The UI shows "0 Documents" for Keyword Extraction job even though it processes 20 documents (12 completed + 8 failed). The Places job correctly shows "20 Documents".

## Root Cause Analysis
The agent worker publishes `EventDocumentUpdated` **asynchronously** in a goroutine with `Publish()`:
- Line 239-243 in `agent_worker.go`: Uses `go func()` and `Publish()` (non-blocking)
- The job completes BEFORE the async increment operations finish
- Document count in metadata stays at 0

In contrast, the places manager uses `PublishSync()` which waits for event handlers to complete.

## Dependency Analysis
Step 1 must complete before Step 2 can verify the fix.

## Execution Groups

### Group 1 (Sequential - Fix)

1. **Fix agent_worker.go to use PublishSync instead of async Publish**
   - Skill: @go-coder
   - Files: internal/jobs/worker/agent_worker.go
   - Complexity: low
   - Critical: no
   - Depends on: none
   - User decision: no

### Group 2 (Sequential - Verification)

2. **Update queue test to verify document count equals processed count**
   - Skill: @test-writer
   - Files: test/ui/queue_test.go
   - Complexity: low
   - Critical: no
   - Depends on: Step 1
   - User decision: no

3. **Verify build and run tests**
   - Skill: @go-coder
   - Files: none
   - Complexity: low
   - Critical: no
   - Depends on: Step 1, 2
   - User decision: no

## Parallel Execution Map
```
[Step 1: Fix agent_worker.go] ──> [Step 2: Update test] ──> [Step 3: Verify]
```

## Success Criteria
- Agent jobs correctly increment document count synchronously
- Test validates document count matches completed + failed count
- Build passes, tests pass
