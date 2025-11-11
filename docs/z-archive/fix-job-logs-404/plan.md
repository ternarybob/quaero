# Fix Job Logs 404 Issue - Implementation Plan

## Executive Summary

**Issue:** Test `TestNewsCrawlerJobExecution` intermittently gets 404 error when requesting `/api/jobs/{id}/logs/aggregated`.

**Root Cause:** Database schema mismatch - the `error` column in the `jobs` table is nullable (NULL by default for successful jobs), but the `scanJob()` function in `internal/storage/sqlite/job_storage.go` incorrectly scans it into a non-nullable `string` variable instead of `sql.NullString`.

**Error Message:**
```
ERR > Failed to get aggregated logs
error=job not found: failed to scan job: sql: Scan error on column index 14, name "error": converting NULL to string is unsupported
```

**Impact:**
- Aggregated logs endpoint returns 404 "Job not found" for jobs with NULL error column
- UI displays "No logs available" message
- Test correctly fails as designed (validates logs are displayed)

## Complexity Assessment

**Complexity: LOW (2/10)**

**Reasoning:**
- Simple type fix (string → sql.NullString)
- Single function affected (`scanJob()`)
- Similar pattern already correctly implemented in `scanJobs()` (line 376)
- No database migration required (schema is correct, code is wrong)
- No API contract changes
- No business logic changes

## Root Cause Analysis

### Discovery Timeline

1. **13:36:31** - Job `3da9db81-70eb-487e-9d55-73de8b1bd6f3` created and executed successfully
2. **13:36:31** - Job reached "Completed" status with no errors (error column = NULL)
3. **13:36:41** - Test navigates to job details page `/job?id=3da9db81-70eb-487e-9d55-73de8b1bd6f3`
4. **13:36:41** - Page loads job details successfully via `GET /api/jobs/{id}` (200 OK)
5. **13:36:41** - Output tab tries to load aggregated logs via `GET /api/jobs/{id}/logs/aggregated`
6. **13:36:41** - `LogService.GetAggregatedLogs()` calls `jobStorage.GetJob(ctx, parentJobID)`
7. **13:36:41** - `GetJob()` calls `scanJob()` which fails with SQL scan error on column 14 (error field)
8. **13:36:41** - Error bubbles up as `ErrJobNotFound`, handler returns 404
9. **13:36:41** - UI receives 404, but **still displays logs** from earlier successful `/api/jobs/{id}/logs` call

### Key Insight

**The test actually passed!** The UI successfully fell back to the non-aggregated logs endpoint and displayed logs correctly. The 404 error was logged but did not break functionality. However, this is a **latent bug** that should be fixed.

### Why This Happens

Compare the two scanning functions:

**❌ BROKEN - scanJob() (line 175-195):**
```go
func (s *JobStorage) scanJob(row *sql.Row) (*models.Job, error) {
    var (
        id, jobType, name, description, configJSON, metadataJSON, status, progressJSON, errorMsg string  // ← BUG: errorMsg as string
        parentID sql.NullString
        // ...
    )

    err := row.Scan(
        &id, &parentID, &jobType, &name, &description, &configJSON, &metadataJSON,
        &status, &progressJSON, &createdAt, &startedAt, &completedAt, &finishedAt,
        &lastHeartbeat, &errorMsg, &resultCount, &failedCount, &depth,  // ← errorMsg scanned as string
    )
    // ...
}
```

**✅ CORRECT - scanJobs() (line 369-436):**
```go
func (s *JobStorage) scanJobs(rows *sql.Rows) ([]*models.JobModel, error) {
    for rows.Next() {
        var (
            id, jobType, name, description, configJSON, metadataJSON, status, progressJSON string
            parentID, errorMsg sql.NullString  // ← CORRECT: errorMsg as sql.NullString
            // ...
        )

        err := rows.Scan(
            &id, &parentID, &jobType, &name, &description, &configJSON, &metadataJSON,
            &status, &progressJSON, &createdAt, &startedAt, &completedAt, &finishedAt,
            &lastHeartbeat, &errorMsg, &resultCount, &failedCount, &depth,  // ← errorMsg scanned as sql.NullString
        )
        // ...
    }
}
```

### Why the Mismatch Exists

1. **Database Schema:** The `error` column in the `jobs` table is TEXT and **nullable** (allows NULL)
2. **Successful Jobs:** When a job completes successfully, the `error` field is NULL
3. **Failed Jobs:** When a job fails, the `error` field contains an error message string
4. **Go SQL Driver:** Cannot scan NULL values into non-nullable Go types (string, int, etc.)
5. **Correct Approach:** Use `sql.NullString` which has `.Valid` and `.String` fields

## Fix Strategy

### Step 1: Update scanJob() Function Signature ✅ LOW RISK

**File:** `internal/storage/sqlite/job_storage.go`
**Lines:** 175-195
**Change:** Declare `errorMsg` as `sql.NullString` instead of `string`

**Before:**
```go
var (
    id, jobType, name, description, configJSON, metadataJSON, status, progressJSON, errorMsg string
    parentID sql.NullString
    // ...
)
```

**After:**
```go
var (
    id, jobType, name, description, configJSON, metadataJSON, status, progressJSON string
    parentID, errorMsg sql.NullString  // ← errorMsg moved to NullString group
    // ...
)
```

**Risk Assessment:** ✅ MINIMAL
- Type-safe change
- Follows existing pattern in `scanJobs()`
- No API changes
- No database changes

### Step 2: Update Error Handling Logic ✅ LOW RISK

**File:** `internal/storage/sqlite/job_storage.go`
**Lines:** 241-248 (in scanJob function)

**Current Logic:**
```go
job := &models.Job{
    JobModel:    jobModel,
    Status:      models.JobStatus(status),
    Progress:    progress,
    Error:       errorMsg,  // ← Direct assignment of string
    ResultCount: resultCount,
    FailedCount: failedCount,
}
```

**New Logic:**
```go
// Extract error message (NULL-safe)
var errorMessage string
if errorMsg.Valid {
    errorMessage = errorMsg.String
}

job := &models.Job{
    JobModel:    jobModel,
    Status:      models.JobStatus(status),
    Progress:    progress,
    Error:       errorMessage,  // ← NULL-safe assignment
    ResultCount: resultCount,
    FailedCount: failedCount,
}
```

**Risk Assessment:** ✅ MINIMAL
- Preserves existing behavior (empty string for NULL)
- Type-safe null handling
- No downstream code changes required

### Step 3: Verify Consistency with scanJobs() ✅ NO RISK

**Action:** Review `scanJobs()` function (lines 369-436) to ensure error handling is identical

**Observation:** `scanJobs()` returns `[]*models.JobModel` (not `*models.Job`), so it doesn't include the `Error` field. No changes needed there.

**Why No Changes Needed:**
- `JobModel` struct does not have an `Error` field (it's a persistent model field)
- `Job` struct extends `JobModel` with runtime state including `Error` field
- `scanJobs()` is used for listings where error details are not needed
- `scanJob()` is used for single job retrieval where error details are needed

## Files to Modify

### Primary Changes

1. **C:/development/quaero/internal/storage/sqlite/job_storage.go**
   - Function: `scanJob()` (lines 175-268)
   - Changes:
     - Line 177: Change `errorMsg` from `string` to `sql.NullString`
     - Lines 241-248: Add NULL-safe error extraction before job struct creation

### No Changes Required

- ❌ Database schema (already correct)
- ❌ API handlers (no contract changes)
- ❌ Models (Error field already string)
- ❌ Frontend (no API changes)
- ❌ Tests (will pass after fix)

## Testing Strategy

### Unit Tests

**Location:** `internal/storage/sqlite/job_storage_test.go` (create if not exists)

**Test Cases:**
1. **TestScanJob_WithNullError** - Scan job with NULL error field
2. **TestScanJob_WithErrorMessage** - Scan job with error message
3. **TestGetJob_Successful** - Get job that completed successfully (NULL error)
4. **TestGetJob_Failed** - Get job that failed (error message)

### Integration Tests

**Location:** `test/ui/crawler_test.go`

**Existing Test:** `TestNewsCrawlerJobExecution`
- **Current Status:** Test passes because UI falls back to non-aggregated logs
- **After Fix:** Test should pass without 404 errors in logs
- **Validation:** Check service logs for absence of "Failed to get aggregated logs" error

### Manual Testing

**Steps:**
1. Run `.\scripts\build.ps1 -Run`
2. Execute a crawler job (e.g., news-crawler)
3. Wait for job to complete successfully
4. Navigate to job details page
5. Click "Output" tab
6. **Expected:** Logs load without errors
7. **Verify:** No 404 errors in server logs

**Negative Test:**
1. Create a job that will fail (e.g., invalid URL)
2. Wait for job to fail with error message
3. Navigate to job details page
4. Click "Output" tab
5. **Expected:** Logs load with error message visible
6. **Verify:** Error message displayed correctly in UI

## Success Criteria

### Functional Requirements

✅ **GET /api/jobs/{id}/logs/aggregated** returns 200 OK for jobs with NULL error
✅ **GET /api/jobs/{id}/logs/aggregated** returns 200 OK for jobs with error messages
✅ Job details page Output tab displays logs without errors
✅ Test `TestNewsCrawlerJobExecution` passes without 404 errors in service logs

### Non-Functional Requirements

✅ No database migration required
✅ No API contract changes
✅ No breaking changes to existing functionality
✅ Performance unchanged (same query, same logic)

### Validation Checklist

- [ ] `scanJob()` declares `errorMsg` as `sql.NullString`
- [ ] Error extraction uses `.Valid` and `.String` fields
- [ ] `TestNewsCrawlerJobExecution` passes without 404 logs
- [ ] Service logs show no "Failed to get aggregated logs" errors
- [ ] Jobs with NULL error load aggregated logs successfully
- [ ] Jobs with error messages load aggregated logs successfully
- [ ] UI displays logs correctly in both cases

## Risk Assessment

### Per-Step Risk Analysis

**Step 1: Update Variable Declaration** - ✅ MINIMAL RISK
- **What Could Go Wrong:** None - type-safe change, compiler will catch issues
- **Mitigation:** Follow exact pattern from `scanJobs()` function
- **Rollback Plan:** Revert single line change

**Step 2: Update Error Handling** - ✅ MINIMAL RISK
- **What Could Go Wrong:** Incorrect null check logic could cause panics
- **Mitigation:** Use standard `sql.NullString.Valid` check pattern
- **Rollback Plan:** Revert error extraction logic

**Step 3: Testing** - ✅ LOW RISK
- **What Could Go Wrong:** Incomplete test coverage
- **Mitigation:** Test both NULL and non-NULL error cases
- **Rollback Plan:** N/A (testing step)

### Overall Risk: ✅ MINIMAL

**Confidence Level:** 95%

**Why Low Risk:**
1. **Localized Change:** Single function in single file
2. **Proven Pattern:** Same logic already works in `scanJobs()`
3. **Type Safety:** Compiler enforces correctness
4. **No DB Changes:** Schema is already correct
5. **No API Changes:** Same response format
6. **Backward Compatible:** Existing behavior preserved

## Constraints

### Technical Constraints

1. **Must use sql.NullString** - Required for NULL-safe scanning
2. **Must preserve empty string for NULL** - Existing downstream code expects `""` not `nil`
3. **Must not change database schema** - Schema is correct, code is wrong
4. **Must not change API contract** - Handler responses must remain unchanged

### Timeline Constraints

**Estimated Implementation Time:** 15 minutes
- Code changes: 5 minutes
- Testing: 5 minutes
- Validation: 5 minutes

**Total Effort:** < 30 minutes including documentation

### Dependencies

**None** - This fix is completely independent:
- No database migration
- No API changes
- No model changes
- No UI changes
- No dependency updates

## Implementation Notes

### Code Pattern Reference

Use the exact pattern from `scanJobs()` line 376:

```go
parentID, errorMsg sql.NullString
```

Then extract the value:

```go
var errorMessage string
if errorMsg.Valid {
    errorMessage = errorMsg.String
}
```

### Testing Notes

**Key Test Case:** Job with `error = NULL`
- This is the most common case (successful jobs)
- This is the failing case currently
- This must work after the fix

**Secondary Test Case:** Job with `error = "some error message"`
- Less common (failed jobs)
- Should already work (string values scan fine)
- Must continue to work after fix

### Logging Notes

**Before Fix:**
```
ERR > Failed to get aggregated logs
error=job not found: failed to scan job: sql: Scan error on column index 14, name "error": converting NULL to string is unsupported
```

**After Fix:**
```
No error logs - successful aggregated log retrieval
```

## Appendix: Related Code

### Database Schema (Reference)

```sql
CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    parent_id TEXT,
    job_type TEXT NOT NULL,
    name TEXT,
    description TEXT,
    config_json TEXT NOT NULL,
    metadata_json TEXT,
    status TEXT NOT NULL,
    progress_json TEXT,
    created_at INTEGER NOT NULL,
    started_at INTEGER,
    completed_at INTEGER,
    finished_at INTEGER,
    last_heartbeat INTEGER,
    error TEXT,  -- ← NULLABLE (this is correct)
    result_count INTEGER DEFAULT 0,
    failed_count INTEGER DEFAULT 0,
    depth INTEGER DEFAULT 0,
    FOREIGN KEY (parent_id) REFERENCES jobs(id) ON DELETE CASCADE
);
```

### Call Chain (For Reference)

```
UI: GET /api/jobs/{id}/logs/aggregated
  ↓
JobHandler.GetAggregatedJobLogsHandler() [job_handler.go:524]
  ↓
LogService.GetAggregatedLogs() [logs/service.go:65]
  ↓
JobStorage.GetJob() [sqlite/job_storage.go:161]
  ↓
scanJob() [sqlite/job_storage.go:175] ← BUG HERE
  ↓
row.Scan(&errorMsg) ← FAILS when errorMsg is string and DB value is NULL
```

### Error Flow

```
scanJob() returns error: "failed to scan job: sql: Scan error..."
  ↓
GetJob() returns error
  ↓
GetAggregatedLogs() wraps error with ErrJobNotFound
  ↓
GetAggregatedJobLogsHandler() checks errors.Is(err, ErrJobNotFound)
  ↓
Handler returns 404 "Job not found"
```

## Conclusion

This is a **simple, low-risk fix** with **high confidence** of success. The bug is a straightforward type mismatch (string vs sql.NullString) with a proven fix pattern already in use elsewhere in the same file (`scanJobs()`).

**Recommendation:** Proceed with implementation immediately.

---

**Plan Created:** 2025-11-09
**Estimated Completion:** 2025-11-09
**Risk Level:** Minimal
**Confidence:** 95%
