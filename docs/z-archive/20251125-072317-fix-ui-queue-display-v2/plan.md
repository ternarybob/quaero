# Plan: Fix UI Queue Display for Jobs V2

**Created:** 2025-11-25 07:23
**Task:** Fix UI not showing queued jobs despite job execution and statistics showing jobs exist
**Previous Attempt:** docs/features/20251125-fix-ui-queue-display (FAILED - incomplete fix)

## Problem Analysis

### Evidence from Test Failure (test/ui/queue_test.go)

**What Works:**
1. ✅ Job is triggered successfully from Jobs page
2. ✅ Job is created in backend (visible in service logs)
3. ✅ Job executes (fails due to API key issue - EXPECTED)
4. ✅ Job Statistics API shows: 1 TOTAL, 1 FAILED
5. ✅ Service logs show job creation and failure events

**What's Broken:**
1. ❌ Queue UI shows "No jobs found matching the current filters"
2. ❌ UI query `/api/jobs?parent_id=root&status=pending,running,completed,failed,cancelled` returns empty array
3. ❌ Test fails at line 160: "job not found in queue after 10s"

### Root Cause (from Screenshot + Code Analysis)

**Issue:** The UI filter chips are ALL active (Pending, Running, Completed, Failed, Cancelled), which means the query includes ALL statuses. However, the job list is still empty.

Looking at the code:
1. **internal/storage/badger/job_storage.go:144-154** - ListJobs handles `parent_id=root` filter
2. **internal/storage/badger/job_storage.go:172-186** - Status filter supports comma-separated values
3. **Previous fix attempt** tried to address this but was incomplete

**Hypothesis:** The `parent_id=root` filter is working (it should match jobs where ParentID == nil), BUT there's a mismatch between:
- How jobs are being CREATED (what ParentID value they get)
- How jobs are being QUERIED (what the UI expects)

### Critical Discovery

From service.log analysis and previous attempt docs:
- Jobs ARE being created and stored
- Jobs ARE being counted by CountJobsByStatus
- Jobs ARE NOT being returned by ListJobs with the UI's query parameters

This suggests the issue is in the **ListJobs filtering logic**, specifically around how `parent_id=root` is being handled.

## Dependency Analysis

1. **Backend Storage Query** - Parent ID filter must correctly match root jobs
2. **Job Creation** - Jobs must be created with correct ParentID value
3. **UI Query Logic** - Must correctly request root jobs
4. **Error Display** - UI must show errors when jobs fail

## Execution Strategy

### Sequential Group 1: Investigation & Root Fix
**Must run first to understand the actual bug**

#### Step 1: Deep Investigation of Job Creation and Storage
- **Skill:** @code-architect
- **Files:**
  - internal/jobs/job_definition_orchestrator.go
  - internal/jobs/managers/*.go (PlacesSearchManager specifically)
  - internal/storage/badger/job_storage.go
- **Task:**
  1. Trace how jobs are created when user triggers "Nearby Restaurants" job
  2. Verify what ParentID value is being set on created jobs
  3. Check if jobs are being saved with ParentID = nil OR ParentID = some value
  4. Review service logs more carefully to find job creation with ID
- **Output:** Document exact flow and ParentID values used
- **Depends on:** none
- **User decision:** no

#### Step 2: Fix Backend ListJobs Filter Logic
- **Skill:** @go-coder
- **Files:** internal/storage/badger/job_storage.go (ListJobs method)
- **Task:**
  1. Fix the `parent_id=root` filter matching logic
  2. Ensure it correctly matches jobs where ParentID is nil OR empty string
  3. Add debug logging to show what's being filtered
  4. Verify status filter is working correctly with comma-separated values
- **Output:** Fixed ListJobs method that returns root jobs correctly
- **Depends on:** Step 1 findings
- **User decision:** no

### Parallel Group 2: UI and Documentation
**Can run after Step 2 completes**

#### Step 3a: Verify Error Display in Queue UI
- **Skill:** @go-coder
- **Files:** pages/queue.html
- **Task:**
  1. Review error display logic (lines 313-324)
  2. Verify job.error field is being populated correctly
  3. Test that failed jobs show error message in UI
  4. Ensure error truncation works (100 char limit)
- **Output:** Confirmed error display or fixes applied
- **Depends on:** Step 2
- **User decision:** no
- **Sandbox:** worker-a

#### Step 3b: Add API Query Debugging
- **Skill:** @go-coder
- **Files:** pages/queue.html (Alpine.js jobList component)
- **Task:**
  1. Add console.log to show API query parameters
  2. Add console.log to show API response
  3. Verify filter chips are correctly building query string
  4. Check if API errors are being logged
- **Output:** Enhanced debugging for UI queries
- **Depends on:** Step 2
- **User decision:** no
- **Sandbox:** worker-b

### Sequential Group 3: Testing and Validation

#### Step 4: Run UI Test
- **Skill:** @test-writer
- **Files:** test/ui/queue_test.go
- **Task:**
  1. Run `go test -v ./test/ui/queue_test.go -run TestQueue`
  2. Verify job appears in queue UI
  3. Verify job status is displayed (failed in this case)
  4. Verify error message appears for failed job
  5. Take screenshots at each stage
- **Output:** Test passing with screenshots
- **Depends on:** Steps 2, 3a, 3b
- **User decision:** no

#### Step 5: Cleanup Redundant Code
- **Skill:** @code-architect
- **Files:** As discovered during implementation
- **Task:**
  1. Remove any debugging code added temporarily
  2. Remove redundant previous fix attempts if found
  3. Consolidate duplicate logic if any
- **Output:** Clean, production-ready code
- **Depends on:** Step 4
- **User decision:** no

## Success Criteria

1. ✅ Queue UI displays jobs immediately after creation
2. ✅ Failed jobs appear in UI with "Failed" status badge
3. ✅ Error messages are visible in UI for failed jobs
4. ✅ Test `test/ui/queue_test.go` passes completely
5. ✅ No redundant code remains
6. ✅ Breaking changes are acceptable per user requirements

## Parallel Execution Map

```
[Step 1: Investigate] ──> [Step 2: Fix Backend] ──┬──> [Step 3a: Verify UI Errors] ──┐
                                                   │                                   │
                                                   └──> [Step 3b: Add UI Debug] ──────┤
                                                                                       │
                                                                                       ├──> [Step 4: Test] ──> [Step 5: Cleanup]
```

## Risk Assessment

**Low Risk:**
- Backend filter fix is isolated change
- UI already has error display (just needs verification)
- Test provides clear pass/fail criteria

**Medium Risk:**
- May discover ParentID is set inconsistently across different job types
- May need to fix multiple manager classes if they set ParentID differently

**Mitigation:**
- Start with thorough investigation (Step 1)
- Test incrementally after each fix
- Keep previous attempt docs as reference

## Notes from Previous Attempt

Previous attempt (docs/features/20251125-fix-ui-queue-display) made changes but test still failed. Key learnings:
1. Status filter handling was updated in job_storage.go
2. UI error display was already in place
3. The fix was incomplete - likely missed the core ParentID filter issue

This attempt will focus on:
1. More thorough investigation before coding
2. Verifying the ACTUAL ParentID values in created jobs
3. Ensuring the filter logic matches reality
