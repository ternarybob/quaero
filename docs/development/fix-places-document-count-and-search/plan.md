# Plan: Fix Places Document Count and Search Relevance

## Problem 1: Document Count Not Updated
The screenshot shows "0 Documents" for a completed places job. The root cause is that `PlacesSearchStepExecutor` saves documents directly without publishing the `EventDocumentSaved` event. The existing architecture relies on this event to trigger `ParentJobExecutor.IncrementDocumentCount()`.

**Current Flow (Broken):**
```
PlacesSearchStepExecutor.ExecuteStep()
  → documentService.SaveDocument()
  → ❌ NO EVENT PUBLISHED
  → ❌ ParentJobExecutor never increments count
  → ❌ UI shows "0 Documents"
```

**Required Flow:**
```
PlacesSearchStepExecutor.ExecuteStep()
  → documentService.SaveDocument()
  → ✅ Publish EventDocumentSaved with parent_job_id
  → ✅ ParentJobExecutor.IncrementDocumentCount()
  → ✅ UI shows "1 Document"
```

## Problem 2: Search Relevance
User reports places search returns irrelevant results. Need to investigate the search query passed to Google Places API.

## Steps

1. **Add EventService to PlacesSearchStepExecutor**
   - Skill: @code-architect
   - Files: `internal/jobs/executor/places_search_step_executor.go`
   - User decision: no
   - Add EventService dependency to struct
   - Update constructor to accept eventService parameter
   - Update app.go to pass eventService during initialization

2. **Publish EventDocumentSaved after document save**
   - Skill: @go-coder
   - Files: `internal/jobs/executor/places_search_step_executor.go`
   - User decision: no
   - After successful `documentService.SaveDocument()`, publish event
   - Event payload must include: `parent_job_id`, `document_id`, `job_id`, `source_type`
   - Follow existing pattern from `document_persister.go`
   - Handle event publishing errors (log but don't fail)

3. **Add logging for search query debugging**
   - Skill: @go-coder
   - Files: `internal/services/places/service.go`
   - User decision: no
   - Log the actual search query sent to Google Places API
   - Log the number of results returned
   - Log sample place names for verification

4. **Update app.go dependency injection**
   - Skill: @code-architect
   - Files: `internal/app/app.go`
   - User decision: no
   - Update `NewPlacesSearchStepExecutor()` call to include EventService
   - Verify EventService is already initialized before executor creation

5. **Write API test for document count**
   - Skill: @test-writer
   - Files: `test/api/places_job_document_test.go`
   - User decision: no
   - Update existing test to verify document count in job metadata
   - Add test to verify EventDocumentSaved is published
   - Verify count increments properly

## Success Criteria
- Places job shows "1 Document" in Queue UI after completion
- Document count tracked in job metadata (`document_count` field)
- EventDocumentSaved published with correct parent_job_id
- Search query logged for debugging relevance issues
- Tests verify end-to-end document counting flow
- Architecture is job-type agnostic (works for any executor that saves documents)

## Design Note
The document counting system is intentionally generic and job-type agnostic. ANY executor that saves documents can participate by:
1. Publishing `EventDocumentSaved` with `parent_job_id` in payload
2. ParentJobExecutor automatically increments count for ANY document save event with a valid parent_job_id
3. No places-specific code needed in the counting logic
