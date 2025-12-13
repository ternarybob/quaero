# Plan: Create Individual Documents for Each Place in Search Results

## Problem Statement
Currently, Places API search jobs create a single document containing all search results. This creates issues:
- All places are grouped together in one document
- Cannot search/filter individual places
- Cannot tag/categorize individual places
- Poor user experience for browsing results

## Solution
Modify `PlacesSearchManager` to create **one document per place** instead of one document per search.

## Steps

1. **Refactor document creation logic**
   - Skill: @code-architect
   - Files: `internal/jobs/manager/places_search_manager.go`
   - User decision: no
   - Change `convertPlacesResultToDocument()` to `createPlaceDocuments()` that returns `[]*models.Document`
   - Loop through each place in `result.Places` and create individual documents
   - Generate unique document IDs for each place (use place_id)

2. **Update document saving logic**
   - Skill: @go-coder
   - Files: `internal/jobs/manager/places_search_manager.go`
   - User decision: no
   - Replace single `SaveDocument()` call with loop to save each document
   - Publish `EventDocumentSaved` for each document (not just one)
   - Update logging to report number of documents created

3. **Update tests to expect multiple documents**
   - Skill: @test-writer
   - Files: `test/api/places_job_document_test.go`
   - User decision: no
   - Modify `TestPlacesJobDocumentCount` to expect N documents (where N = max_results or actual results)
   - Modify `TestPlacesJobDocumentTags` to verify all documents have tags
   - Add test to verify each document is searchable individually

4. **Test and validate**
   - Skill: @test-writer
   - Files: `test/api/places_job_document_test.go`
   - User decision: no
   - Run existing tests to ensure they pass
   - Verify document count matches number of places returned
   - Verify each document is independently searchable

## Success Criteria
- Each place in search results creates its own document
- Document IDs are unique per place (using place_id)
- Each document inherits tags from job definition
- `EventDocumentSaved` is published for each document
- Job `result_count` and `document_count` reflect total places (not just 1)
- All existing tests updated and passing
- Documents are individually searchable and filterable
