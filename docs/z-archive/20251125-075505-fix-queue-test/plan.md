# Plan: Fix test/ui/queue_test.go Timeout Issue

## Problem Analysis

### Current State
The test `test/ui/queue_test.go` successfully:
1. ✅ Triggers jobs via UI
2. ✅ Jobs appear in queue (PRIMARY ISSUE FIXED by QueueStorage refactor)
3. ❌ **Times out waiting for job completion** (line 172-180)

### Root Cause Investigation Needed
From the test output:
```
✓ Job triggered: Nearby Restaurants (Wheelers Hill)
✓ Job found in queue
Waiting for job completion...
Command timed out after 2m 0s
```

The test times out at line 172-180 while polling for job status to change to "Completed" or "Completed with Errors". This suggests:

**Hypothesis 1**: Jobs are created but not being executed by workers
**Hypothesis 2**: Job status updates aren't propagating to the UI
**Hypothesis 3**: Polling logic has issues (selector, timeout, update frequency)
**Hypothesis 4**: Worker/crawler service not processing jobs

## Dependency Analysis

```
Investigation (Step 1)
    ├── Check job execution in logs/storage
    ├── Check worker/crawler service status
    └── Check UI polling mechanism
    │
    ├──> Parallel Group 2 (Root Cause Specific Fixes)
    │    ├── Fix A: Worker/Crawler Service
    │    ├── Fix B: Job Status Updates
    │    └── Fix C: UI Polling Logic
    │
    └──> Integration & Verification (Step 3)
         ├── Merge all fixes
         ├── Run full test
         └── Verify completion
```

## Execution Groups

### Group 1: Investigation (Sequential)

#### Step 1: Deep Investigation of Job Execution
- **Skill:** @go-coder
- **Files:**
  - `internal/services/crawler/service.go`
  - `internal/jobs/manager.go`
  - `internal/queue/worker.go` (if exists)
  - `internal/handlers/job_handler.go`
- **Depends on:** none
- **User decision:** no
- **Actions:**
  1. Check if jobs are being picked up by workers after creation
  2. Check job status update flow (QueueJobState transitions)
  3. Check if crawler service is processing queued jobs
  4. Verify job completion detection and status persistence
  5. Check logs to see if jobs are stalling at a specific stage

**Success Criteria:** Identify which component is failing:
- Jobs not being picked up by workers?
- Jobs running but status not updating?
- Status updating but UI not refreshing?

---

### Group 2: Parallel Fixes (Based on Investigation)

These will run in parallel once we identify the issue(s):

#### Step 2a: Fix Worker/Queue Processing
- **Skill:** @go-coder
- **Files:**
  - `internal/services/crawler/service.go`
  - `internal/jobs/manager.go`
  - `internal/queue/*` (if worker files exist)
- **Depends on:** Step 1
- **User decision:** no
- **Sandbox:** worker-a
- **Actions:**
  1. Ensure jobs are being dequeued and processed
  2. Fix any blocking issues in job execution
  3. Verify worker pool is active and processing jobs
  4. Add logging for job pickup and processing

#### Step 2b: Fix Job Status Update Mechanism
- **Skill:** @go-coder
- **Files:**
  - `internal/storage/badger/queue_storage.go`
  - `internal/handlers/job_handler.go`
  - `internal/api/routes.go` (if status endpoint exists)
- **Depends on:** Step 1
- **User decision:** no
- **Sandbox:** worker-b
- **Actions:**
  1. Ensure QueueJobState status transitions are persisted
  2. Verify status updates are atomic and consistent
  3. Check API endpoint returns latest status
  4. Add real-time status update mechanism if missing

#### Step 2c: Fix UI Polling and Status Display
- **Skill:** @none (frontend/Alpine.js)
- **Files:**
  - `web/templates/queue.html` (or similar)
  - `test/ui/queue_test.go` (polling logic)
- **Depends on:** Step 1
- **User decision:** no
- **Sandbox:** worker-c
- **Actions:**
  1. Check Alpine.js component refresh mechanism
  2. Verify polling interval is reasonable
  3. Fix status selector if incorrect
  4. Add debug logging to test polling

---

### Group 3: Integration & Verification (Sequential)

#### Step 3: Merge Fixes and Run Full Test
- **Skill:** @test-writer
- **Files:** All modified files from Group 2
- **Depends on:** Steps 2a, 2b, 2c
- **User decision:** no
- **Actions:**
  1. Merge all parallel worker changes
  2. Resolve any conflicts
  3. Build and compile: `go build -o /tmp/quaero-test ./...`
  4. Run full UI test: `go test -v ./test/ui/queue_test.go -run TestQueue`
  5. Verify job completes within timeout
  6. Check for any remaining issues

**Success Criteria:**
- Test passes without timeout
- Jobs complete and status updates correctly
- No compilation errors
- No test failures

---

## Parallel Execution Map

```
[Step 1: Investigation] ────────────────────────────┐
     Identify root cause(s)                         │
                                                     │
     ┌───────────────────────────────────────────┐ │
     │ Group 2: Parallel Fixes (simultaneous)    │ │
     │                                            │ │
     │  [2a: Worker Processing] ──┐              │ │
     │                             │              │ │
     │  [2b: Status Updates]      ├── 3-5min ────┼─┤
     │                             │              │ │
     │  [2c: UI Polling]          ──┘             │ │
     └───────────────────────────────────────────┘ │
                                                     │
[Step 3: Integration & Test] ───────────────────────┘
     Merge + Verify
```

**Estimated Timeline:**
- Step 1 (Investigation): 5-7 minutes
- Step 2 (Parallel Fixes): 3-5 minutes (concurrent)
- Step 3 (Integration): 3-5 minutes
- **Total: ~15 minutes** (vs. 25+ minutes sequential)

---

## Success Criteria

### Must Have:
1. ✅ Test `test/ui/queue_test.go` passes completely
2. ✅ Jobs complete within 120s timeout (test line 177)
3. ✅ Job status updates to "Completed" or "Completed with Errors"
4. ✅ No compilation errors
5. ✅ Job statistics are captured correctly

### Nice to Have:
- Improved logging for debugging job execution
- Better error messages in test output
- Faster job execution (if possible without external API changes)

---

## Known Constraints

1. **External APIs**: Places job uses external API (Google Places) which may be rate-limited
2. **Test Environment**: Running in headless Chrome with isolated database
3. **Time Limits**: Test has 300s total timeout (line 25), job polling has 120s timeout (line 177)
4. **Previous Fix**: QueueStorage refactor already fixed the "job not appearing" issue

---

## Risk Assessment

**Low Risk:**
- Investigation (read-only)
- UI polling fix (test-only change)

**Medium Risk:**
- Status update mechanism (could affect other components)
- Worker processing changes (core execution path)

**Mitigation:**
- Use sandboxed workers for parallel changes
- Test each fix independently before merging
- Maintain backward compatibility with existing job types

---

## Assumptions

1. The QueueStorage refactor correctly saves jobs to storage
2. The job trigger mechanism works (confirmed by test output)
3. The UI can display jobs (confirmed by "✓ Job found in queue")
4. The issue is specifically with job execution or status updates

---

## Next Steps After Plan Approval

1. Execute Step 1 (Investigation) to identify root cause
2. Based on findings, spawn parallel workers for Group 2 fixes
3. Validate each fix independently
4. Merge and run integration test
5. Document findings and solutions
