# Plan: Fix Places Job Tags Not Appearing

## Overview
Documents created by Places job executor are not receiving tags from the job definition. The root cause is that the `convertPlacesResultToDocument()` method in `places_search_step_executor.go` does not populate the `Tags` field when creating documents.

## Root Cause Analysis

**Problem Location:** `internal/jobs/executor/places_search_step_executor.go:263`

The `convertPlacesResultToDocument()` method creates a Document struct but does not set the `Tags` field:

```go
doc := &models.Document{
    ID:              docID,
    SourceType:      "places",
    SourceID:        jobID,
    Title:           fmt.Sprintf("Places Search: %s", result.SearchQuery),
    ContentMarkdown: contentBuilder.String(),
    DetailLevel:     models.DetailLevelFull,
    Metadata:        metadata,
    URL:             "",
    CreatedAt:       now,
    UpdatedAt:       now,
    // MISSING: Tags field!
}
```

**Expected Flow:**
1. Job definition file (TOML) defines tags: `tags = ["Restaurants", "Wheelers Hill", "google-search"]`
2. Job definition loaded into `JobDefinition` model with tags field
3. **BROKEN:** Places search step executor needs to pass tags from job definition to created document
4. Document saved with tags to database
5. UI retrieves and displays tags

## Steps

### 1. **Pass JobDefinition Tags to PlacesSearchStepExecutor**
   - Skill: @go-coder
   - Files: `internal/jobs/executor/places_search_step_executor.go`
   - User decision: no
   - Modify `ExecuteStep()` method to pass `jobDef.Tags` to `convertPlacesResultToDocument()`
   - Update `convertPlacesResultToDocument()` signature to accept tags parameter
   - Assign tags to document struct

### 2. **Verify Tags are Stored in Database**
   - Skill: @go-coder
   - Files: `test/api/places_job_document_test.go`
   - User decision: no
   - Add API test that verifies Places job documents have tags
   - Test should create a job definition with tags, execute job, retrieve document, verify tags present

### 3. **Build and Manual Test**
   - Skill: @none
   - Files: N/A
   - User decision: no
   - Build application using `./scripts/build.ps1 -Run`
   - Navigate to Jobs page
   - Execute "Nearby Restaurants (Wheelers Hill)" job
   - Navigate to Documents page
   - Verify tags appear in TAGS column

## Success Criteria
- Places job documents created with tags from job definition
- Tags stored in database tags column as JSON array
- Tags displayed in UI TAGS column
- Tags available for filtering via tag multi-select
- API test verifies tag flow for Places jobs

## Architecture Notes
- Places jobs inherit tags from job definition (same pattern as crawler jobs)
- Tags passed from JobDefinition → PlacesSearchStepExecutor → Document
- No changes needed to database schema or UI (already implemented)
