# Plan: Fix Keyword Extraction Job - Documents Not Updating

## Problem Analysis

From logs and code review:
1. **Log shows:** "No documents found for agent processing" (line 326 of quaero.2025-11-18T06-08-23.log)
2. **Root cause:** `AgentManager.queryDocuments()` filters by `jobDef.SourceType`
3. **Issue:** Job definition has `type = "custom"` but no `source_type` field
4. **Result:** Query filters by empty source_type, returns no documents

## Secondary Issue
Queue shows "0 Documents" because:
- Agent jobs don't create new documents
- They update existing documents
- No event is published for document updates
- Need to track document updates, not just creation

## Steps

### 1. Fix document query to return all documents when source_type is empty
   - Skill: @code-architect
   - Files: `internal/jobs/manager/agent_manager.go`
   - User decision: no
   - Modify `queryDocuments()` to query ALL documents when `jobDef.SourceType` is empty/unspecified
   - Remove source_type filter or make it optional

### 2. Add EventDocumentUpdated event and publish from agent executor
   - Skill: @go-coder
   - Files: `internal/interfaces/events.go`, `internal/jobs/executor/agent_executor.go`
   - User decision: no
   - Define new `EventDocumentUpdated` event type
   - Publish event after agent successfully updates document metadata
   - Include document_id, job_id, and update timestamp in payload

### 3. Update JobMonitor to track document updates
   - Skill: @go-coder
   - Files: `internal/jobs/monitor/job_monitor.go`
   - User decision: no
   - Subscribe to `EventDocumentUpdated` events
   - Increment `document_count` or new `update_count` field in job metadata
   - Track both documents created AND documents updated

### 4. Test keyword extraction job end-to-end
   - Skill: @test-writer
   - Files: `test/api/agent_job_test.go` (create new)
   - User decision: no
   - Create test that:
     - Creates sample documents
     - Runs keyword extraction job
     - Verifies documents have keywords in metadata
     - Verifies job shows correct update count

### 5. Update UI to show document update count
   - Skill: @go-coder
   - Files: `internal/handlers/job_handler.go`, job queue display logic
   - User decision: no
   - Add `documents_updated` field to job response
   - Display both "created" and "updated" counts in queue UI

## Success Criteria
- ✅ Keyword extraction job processes all documents (not zero)
- ✅ Documents have `keyword_extractor` metadata after job runs
- ✅ Queue shows document update count (not just "0 Documents")
- ✅ Tests verify end-to-end functionality
- ✅ UI displays meaningful document counts
