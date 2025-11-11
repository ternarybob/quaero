# Plan: Fix Places Job Document Saving

## Problem
The "Nearby Restaurants (Wheelers Hill)" job completes successfully but shows **0 Documents** in the Queue Management UI. The Google Places API returns JSON data, but this data is only stored in the job's progress field - it's never converted to documents in the `documents` table.

## Root Cause Analysis

**Current Flow:**
1. `PlacesSearchStepExecutor.ExecuteStep()` calls `placesService.SearchPlaces()`
2. `placesService.SearchPlaces()` fetches JSON from Google Places API
3. Result (JSON with places array) is returned to executor
4. Executor stores result in job's `progress_json` field
5. **MISSING:** No conversion from JSON to `documents` table

**Architecture Violation:**
- Requirements state: "The list job needs to save the output as a document"
- Google returns JSON → Phase 1: save as document (direct JSON → document conversion)
- No special places tables should exist
- Currently: JSON is stored in job progress, never becomes a searchable document

## Solution: Convert JSON Places to Documents

### Steps

1. **Add document conversion method to PlacesSearchStepExecutor**
   - Skill: @go-coder
   - Files: `internal/jobs/executor/places_search_step_executor.go`
   - User decision: no
   - Add method: `convertPlacesResultToDocument(*models.PlacesSearchResult, string) *models.Document`
   - Convert JSON places list to single markdown document
   - Metadata: store places array as JSON in document.Metadata field
   - SourceType: "places"
   - SourceID: generated from search query + timestamp

2. **Inject DocumentService into PlacesSearchStepExecutor**
   - Skill: @code-architect
   - Files:
     - `internal/jobs/executor/places_search_step_executor.go` (constructor)
     - `internal/app/app.go` (dependency injection)
   - User decision: no
   - Add DocumentService interface dependency to executor
   - Pass documentService from app initialization

3. **Save document after search completes**
   - Skill: @go-coder
   - Files: `internal/jobs/executor/places_search_step_executor.go`
   - User decision: no
   - After successful `SearchPlaces()` call, convert result to document
   - Call `documentService.SaveDocument()` to persist
   - Document will be searchable via FTS5 index

4. **Remove places-specific tables/code (cleanup)**
   - Skill: @code-architect
   - Files:
     - `internal/models/places.go` (review, keep models needed for API response)
     - `internal/interfaces/places_service.go` (keep interface)
     - Remove any places_lists/places_items table definitions if they exist
   - User decision: no
   - Keep: API response models (`PlaceItem`, `PlacesSearchResult`)
   - Remove: Any storage-specific places tables (already confirmed none exist in schema.go)
   - Document design: Single document per search, places array in metadata

5. **Write API test**
   - Skill: @test-writer
   - Files: `test/api/places_job_test.go` (new)
   - User decision: no
   - Test: Create places job, wait for completion, verify document exists
   - Assert: Document count > 0 for completed job
   - Assert: Document has correct source_type="places"
   - Assert: Document metadata contains places array

## Success Criteria
- Places job completes and creates 1 document in `documents` table
- Document has `source_type = "places"`
- Document `content_markdown` contains formatted places list
- Document `metadata` contains original JSON places array
- Queue UI shows "1 Document" (or N documents if we split per place)
- No places-specific tables exist (use generic documents table)
- Test verifies document creation

## Design Decision: Single Document vs Multiple Documents

**Phase 1 Approach (Recommended):**
- **Single document per search job**
- content_markdown: Formatted markdown list of all places
- metadata: Full JSON array of places for structured queries
- Simpler implementation
- Matches "save output as a document" requirement

**Future Enhancement:**
- Could split into N documents (1 per place) if needed
- Would require coordination with job result counting
- Not needed for Phase 1

**Decision:** Use single document approach for Phase 1 (implemented in Step 1)
