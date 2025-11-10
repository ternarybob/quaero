# Progress: Fix Places Document Count and Search Relevance

- ✅ Step 1: Add EventService to PlacesSearchStepExecutor [@code-architect] - Done
- ✅ Step 2: Publish EventDocumentSaved after document save [@go-coder] - Done
- ✅ Step 3: Add logging for search query debugging [@go-coder] - Done
- ✅ Step 4: Update app.go dependency injection [@code-architect] - Done
- ✅ Step 5: Write API test for document count [@test-writer] - Done

## Implementation Details

### Step 1 & 4: EventService Dependency Injection

**Modified Files:**
- `internal/jobs/executor/places_search_step_executor.go:19` - Added `eventService interfaces.EventService` field
- `internal/jobs/executor/places_search_step_executor.go:27` - Updated constructor to accept eventService parameter
- `internal/app/app.go:360` - Updated PlacesSearchStepExecutor initialization to pass EventService

**Pattern:**
```go
type PlacesSearchStepExecutor struct {
    placesService   interfaces.PlacesService
    documentService interfaces.DocumentService
    eventService    interfaces.EventService  // New
    logger          arbor.ILogger
}
```

### Step 2: EventDocumentSaved Publishing

**Modified File:**
- `internal/jobs/executor/places_search_step_executor.go:157-186` - Added event publishing after document save

**Implementation:**
- Publishes `EventDocumentSaved` asynchronously after successful `SaveDocument` call
- Event payload includes: `job_id`, `parent_job_id`, `document_id`, `source_type`, `timestamp`
- Error handling: Logs warnings but doesn't fail the job
- Follows the same pattern used in `document_persister.go` for crawler documents

**Pattern:**
```go
// Publish document_saved event for parent job document count tracking
if e.eventService != nil && parentJobID != "" {
    payload := map[string]interface{}{
        "job_id":        parentJobID,
        "parent_job_id": parentJobID,
        "document_id":   doc.ID,
        "source_type":   "places",
        "timestamp":     time.Now().Format(time.RFC3339),
    }
    event := interfaces.Event{
        Type:    interfaces.EventDocumentSaved,
        Payload: payload,
    }
    go func() {
        if err := e.eventService.Publish(context.Background(), event); err != nil {
            e.logger.Warn().
                Err(err).
                Str("document_id", doc.ID).
                Str("parent_job_id", parentJobID).
                Msg("Failed to publish document_saved event")
        }
    }()
}
```

### Step 3: Search Query Logging

**Modified File:**
- `internal/services/places/service.go:158-171` - Enhanced textSearch logging
- `internal/services/places/service.go:230-246` - Enhanced nearbySearch logging

**Implementation:**
- Logs actual search query sent to Google Places API
- Logs number of results returned
- Logs first 3 place names for manual relevance verification
- Changed log level from Debug to Info for better visibility

**Example Output:**
```
INFO Google Places Text Search completed - verify relevance
  search_query="restaurants near Sydney"
  results_count=5
  status="OK"
  sample_places=["Restaurant A", "Restaurant B", "Restaurant C"]
```

### Step 5: API Test Enhancement

**Modified File:**
- `test/api/places_job_document_test.go:378-401` - Enhanced TestPlacesJobDocumentCount

**Implementation:**
- Added verification of `document_count` field in job's `metadata_json`
- This confirms that `EventDocumentSaved` was published and processed by `ParentJobExecutor`
- Tests both `result_count` (legacy) and `metadata.document_count` (new event-driven approach)
- Clear error message if event not published: "This indicates EventDocumentSaved was not published or processed"

## Architecture Pattern

This implementation follows a **job-type agnostic event-driven architecture**:

1. **ANY executor** that saves documents can publish `EventDocumentSaved`
2. `ParentJobExecutor` subscribes to this event (in `parent_job_executor.go:364`)
3. When event received, calls `JobManager.IncrementDocumentCount()` to update job metadata
4. Document count stored in `metadata_json.document_count` field
5. No places-specific code in the counting logic

**Flow:**
```
PlacesSearchStepExecutor.ExecuteStep()
  → documentService.SaveDocument()
  → eventService.Publish(EventDocumentSaved) with parent_job_id
  → ParentJobExecutor receives event
  → jobManager.IncrementDocumentCount(parent_job_id)
  → metadata.document_count incremented
  → UI shows "1 Document"
```

## Compilation Status

✅ All code compiles successfully:
- Main application: `go build ./cmd/quaero`
- Tests: `go test -c test/api/places_job_document_test.go`

## Testing Status

- ✅ Tests created: Enhanced `TestPlacesJobDocumentCount` with metadata verification
- ⏳ Tests not run yet: Requires Google Places API key in config
- 📋 Manual test plan:
  1. Configure Google Places API key in `quaero.toml`
  2. Run nearby-restaurants-places job from UI
  3. Verify logs show: "Published document_saved event for parent job document count"
  4. Check Queue UI shows "1 Document" instead of "0 Documents"
  5. Verify `metadata.document_count = 1` in job's metadata_json field

## Search Relevance Debugging

With enhanced logging, search relevance issues can now be diagnosed:
1. Check logs for "Google Places Text/Nearby Search completed - verify relevance"
2. Compare `search_query` with `sample_places` returned
3. If places are irrelevant, investigate Google Places API query construction
4. Consider adding filters (type, keyword) to narrow results

Updated: 2025-11-10T20:45:00Z
