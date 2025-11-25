# Plan: Fix UI Queue Display for Jobs

## Problem Summary
The UI queue page is not displaying jobs that have been created and executed. The test shows:
- Job is triggered successfully from the Jobs page
- Job is created in the backend (ID: 9469b848-05c6-4c6f-8ab9-b0dc23c23f81)
- Job executes and fails (expected - API key error in test environment)
- **BUT**: The job never appears in the Queue UI
- API call `/api/jobs?parent_id=root&status=pending,running,completed,failed,cancelled` returns empty array

## Root Cause Hypothesis
Based on logs and architecture review:
1. Jobs are created with `ParentID = nil` (root jobs)
2. UI queries with `parent_id=root` filter
3. The storage layer or JobManager may not be properly handling `parent_id=root` to match jobs with `ParentID == nil`
4. OR: Jobs are not being persisted correctly to BadgerDB

## Steps

1. **Investigate JobManager.ListJobs implementation**
   - Skill: @code-architect
   - Files: `internal/jobs/manager/job_manager.go`, `internal/storage/badger_job_storage.go`
   - User decision: no
   - Task: Find how `parent_id=root` filter is translated to storage query and whether it correctly matches `ParentID == nil`

2. **Verify job persistence in JobDefinitionOrchestrator**
   - Skill: @go-coder
   - Files: `internal/jobs/job_definition_orchestrator.go`
   - User decision: no
   - Task: Confirm jobs are being saved to BadgerDB correctly with proper parent_id field

3. **Fix parent_id filter handling**
   - Skill: @go-coder
   - Files: `internal/jobs/manager/job_manager.go` or `internal/storage/badger_job_storage.go`
   - User decision: no
   - Task: Ensure `parent_id=root` query parameter correctly matches jobs where `ParentID == nil` or `ParentID == ""`

4. **Ensure failed jobs are properly saved**
   - Skill: @go-coder
   - Files: `internal/jobs/job_definition_orchestrator.go`
   - User decision: no
   - Task: Verify that when job execution fails, the job record is updated and saved with failed status

5. **Add error display in queue UI**
   - Skill: @go-coder
   - Files: `pages/queue.html`
   - User decision: no
   - Task: Ensure job cards display error messages when jobs fail, extracting error from job logs or metadata

6. **Update queue UI to show all job states**
   - Skill: @go-coder
   - Files: `pages/queue.html`
   - User decision: no
   - Task: Verify UI displays jobs in all states (pending, running, completed, failed) and add visual indicators for failed jobs

7. **Run UI test to verify fix**
   - Skill: @test-writer
   - Files: `test/ui/queue_test.go`
   - User decision: no
   - Task: Run the queue test to confirm jobs now appear in UI and error states are displayed

8. **Clean up redundant code**
   - Skill: @code-architect
   - Files: Various (as discovered during implementation)
   - User decision: no
   - Task: Remove any redundant or dead code discovered during the fix

## Success Criteria
- Queue UI displays jobs immediately after creation
- Failed jobs appear in the UI with error status
- Error messages are visible in the UI (progress or error section)
- Test `test/ui/queue_test.go` passes successfully
- Breaking changes are acceptable per user requirements
- No redundant code remains after cleanup
