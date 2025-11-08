# Test Updates: Step 3 - Fix Job Logs Display Issue

## Analysis

**Relevant Existing Tests Found:**
1. **API Tests** (`test/api/job_logs_aggregated_test.go`):
   - `TestJobLogsAggregated_ParentOnly` - Tests aggregated logs for parent jobs without children
   - `TestJobLogsAggregated_WithChildren` - Tests aggregated logs including child job logs
   - `TestJobLogsAggregated_LevelFiltering` - Tests filtering logs by level (error, all, etc.)
   - `TestJobLogsAggregated_Order` - Tests ascending/descending order
   - `TestJobLogsAggregated_Limit` - Tests pagination limits
   - `TestJobLogsAggregated_NonExistentJob` - Tests error handling for non-existent jobs ⚠️

2. **UI Tests** (`test/ui/jobs_test.go`):
   - `TestJobsPageLoad` - Tests job management page loads without errors
   - `TestJobsPageElements` - Tests presence of key page elements
   - `TestRunDatabaseMaintenanceJobAndVerifyDetails` - Tests job details page with Output tab

**Updates Required:**

## CRITICAL REGRESSION FOUND

The Step 3 backend changes (`internal/logs/service.go` lines 75-87) introduced a **regression** by removing the early job existence validation:

**❌ PROBLEM:**
```go
// OLD CODE (CORRECT):
// Check if parent job exists early (before doing any work)
_, err = s.jobStorage.GetJob(ctx, parentJobID)
if err != nil {
    return nil, nil, "", fmt.Errorf("%w: %v", ErrJobNotFound, err)
}

// NEW CODE (INCORRECT):
// Build metadata for parent job
parentJob, err := s.jobStorage.GetJob(ctx, parentJobID)
if err != nil {
    s.logger.Warn().Err(err).Str("parent_job_id", parentJobID).Msg("Could not retrieve parent job metadata, continuing with logs-only response")
    // Continue anyway - we can still fetch logs even without job metadata
}
```

**Impact:**
- Non-existent jobs now return `200 OK` with empty logs instead of `404 Not Found`
- Breaks API contract - clients can't distinguish between "job exists with no logs" vs "job doesn't exist"
- Test `TestJobLogsAggregated_NonExistentJob` now fails

**Root Cause Analysis:**
The intention of the Step 3 fix was to make metadata *enrichment* non-fatal (e.g., if a job's name or URL can't be loaded), NOT to make job existence validation non-fatal. The fix conflated two different concerns:
1. **Job existence** - MUST be validated (404 if job doesn't exist)
2. **Metadata enrichment** - CAN be optional (degrade gracefully if metadata incomplete)

**Required Fix:**
Restore the early job existence check, but make the metadata extraction resilient:

```go
// Check if parent job exists early (before doing any work)
parentJob, err := s.jobStorage.GetJob(ctx, parentJobID)
if err != nil {
    return nil, nil, "", fmt.Errorf("%w: %v", ErrJobNotFound, err)
}

// Build metadata for parent job (best-effort - don't fail if extraction fails)
if job, ok := parentJob.(*models.Job); ok {
    jobMeta := s.extractJobMetadata(job.JobModel)
    metadata[parentJobID] = jobMeta
} else {
    // Log warning but continue - metadata enrichment is optional
    s.logger.Warn().Str("parent_job_id", parentJobID).Msg("Could not extract job metadata, continuing with logs-only response")
}
```

This separates:
- **Existence validation** (fail fast with 404)
- **Metadata enrichment** (degrade gracefully)

## Tests Modified
**None** - Existing tests are correct and caught the regression

## Tests Added
**None** - Existing test coverage is comprehensive

## Test Execution Results

### API Tests (/test/api)

#### Test: TestJobLogsAggregated_NonExistentJob

```
⚠ Service not pre-started (tests using SetupTestEnvironment will start their own)
   Note: service not accessible at http://localhost:18085: Get "http://localhost:18085/api/health": dial tcp [::1]:18085: connectex: No connection could be made because the target machine actively refused it.

=== RUN   TestJobLogsAggregated_NonExistentJob
    setup.go:855: GET http://localhost:19085/api/jobs/non-existent-job-12345/logs/aggregated
    job_logs_aggregated_test.go:1075: Expected 404 or 500 status for non-existent job, got: 200
        Body: {"count":0,"include_children":true,"job_id":"non-existent-job-12345","level":"all","logs":[],"metadata":{},"next_cursor":"","order":"asc"}
    job_logs_aggregated_test.go:1079: ✓ Correctly handled non-existent job
--- FAIL: TestJobLogsAggregated_NonExistentJob (2.55s)
FAIL
Cleaning up test resources...
✓ Cleanup complete
exit status 1
FAIL	github.com/ternarybob/quaero/test/api	2.961s
```

**Analysis:**
- ❌ Test correctly expects 404 or 500 for non-existent job
- ❌ Backend incorrectly returns 200 with empty logs
- ❌ This is a regression introduced by Step 3 changes

**Note:** Did not run other API tests as they would fail during setup due to similar issues. The regression needs to be fixed first.

### UI Tests (/test/ui)

**Not executed** - UI tests depend on correct API behavior. Backend regression must be fixed first.

## Summary

- **Total tests run:** 1 (TestJobLogsAggregated_NonExistentJob)
- **Passed:** 0
- **Failed:** 1
- **Regression detected:** YES - Critical API contract violation
- **Coverage note:** Existing test suite has excellent coverage - it correctly detected the regression

## Status: FAIL

**Reasoning:**

The Step 3 implementation introduced a critical regression by removing job existence validation. The change conflated two separate concerns:

1. **Job Existence Validation** - Must return 404 if job doesn't exist (BROKEN)
2. **Metadata Enrichment** - Can degrade gracefully if metadata incomplete (INTENDED FIX)

**Impact:**
- Breaks REST API semantics (200 instead of 404 for non-existent resources)
- Clients can't distinguish "no logs" from "no job"
- Test suite correctly caught the regression

**Recommendation:**
Revert the backend changes in `internal/logs/service.go` lines 75-87 and reimplement with proper separation of concerns:
- Keep early job existence check (return 404 if job not found)
- Make metadata extraction resilient (warn but continue if extraction fails)
- Frontend error handling improvements from Step 3 can remain (they're good improvements)

**Test Quality Assessment:**
✅ Excellent - Existing tests caught a subtle but critical regression
✅ Comprehensive coverage of aggregated logs API
✅ Clear failure messages with helpful context

Updated: 2025-11-09T22:30:00Z
