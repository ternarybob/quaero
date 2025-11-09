# Testing the Job Stats and Logging Fixes

## Current Status

✅ **All code fixes are deployed and active:**
- Frontend: `pages/queue.html` (document count priority fix)
- Frontend: `pages/job.html` (job details display + log error handling)
- Backend: `internal/logs/service.go` (job validation + metadata extraction)

✅ **Application rebuilt:** Version 0.1.1968, Build 11-09-09-27-59

## Why You're Still Seeing Issues

**Your database was reset** when the application restarted.

**Config Setting:** `bin/quaero.toml` line 50:
```toml
[storage.sqlite]
reset_on_startup = true # Delete database on startup for clean test runs
```

This means:
- ❌ Your old "News Crawler" job (ID: 3bec25b5) **no longer exists**
- ❌ All job data and logs were deleted
- ❌ The queue is empty (API returns `"total": 0`)

The screenshots you showed were from the OLD job data **before** the restart.

## How to Test the Fixes

### Option 1: Run a New Job (Recommended)

1. **Navigate to** http://localhost:8085/jobs
2. **Click** "News Crawler" to start a new job
3. **Wait** for the job to complete (should take 1-2 minutes)
4. **Observe:**
   - ✅ Document count should be correct (NOT doubled)
   - ✅ Job logs should display properly
   - ✅ "Documents Created" field should show correct value

### Option 2: Disable Database Reset

1. **Stop the service** (Ctrl+C in the terminal)
2. **Edit** `bin/quaero.toml` line 50:
   ```toml
   reset_on_startup = false  # Keep database between restarts
   ```
3. **Rebuild and run:**
   ```powershell
   .\scripts\build.ps1 -Run
   ```
4. **Run a new job** and verify fixes

## What the Fixes Do

### Fix 1: Document Count Priority
**File:** `pages/queue.html` line 1920
**Before:** Checked `child_count > 0` before using `document_count` (race condition)
**After:** Directly checks `document_count` first (authoritative source)

**Expected Result:** Job shows correct count immediately (e.g., 17 not 34)

### Fix 2: Job Details Display
**File:** `pages/job.html` line 97
**Before:** Only used `result_count` (empty for parent jobs)
**After:** Prioritizes `document_count` from metadata

**Expected Result:** Job details page shows correct "Documents Created" value

### Fix 3: Job Logs Display
**Files:** `pages/job.html` (lines 466-525), `internal/logs/service.go` (lines 75-88)
**Before:**
- Backend treated metadata retrieval as fatal error
- Frontend showed generic "Failed to fetch" for all errors

**After:**
- Backend validates job existence (404 if not found) but makes metadata optional
- Frontend distinguishes between 404 (normal), 5xx (error), and other HTTP errors

**Expected Result:**
- Job logs display correctly
- Empty logs show friendly message
- Real errors show appropriate status codes

## Verification Steps

1. **Start a NEW crawler job**
2. **While running**, check the queue page:
   - Document count should increment correctly
   - NOT doubled

3. **When completed**, click "Job Details":
   - "Documents Created" should show correct value
   - Progress should show correct stats

4. **Click "Output" tab**:
   - Logs should display
   - If empty, shows "No logs available" (not an error)
   - If 404, silently handled (job might not have logs)

## Common Issues

### "No logs available for this job"
- **Cause:** Job has no logs in `job_logs` table
- **Fix:** This is normal for some jobs - not an error
- **Verification:** Frontend should NOT show error notification for 404

### Document count still doubled
- **Cause:** OLD job data from before fixes
- **Fix:** Run a NEW job to test the fixes

### Logs endpoint returns 404
- **Cause:** Job ID doesn't exist (database was reset)
- **Fix:** Use a valid job ID from a NEW job

## Expected Test Results

After running a NEW job with the fixes:

✅ **Queue Page:**
```
News Crawler
📄 17 Documents  (not 34)
Progress: 17 completed
```

✅ **Job Details Page:**
```
Documents Created: 17  (not 0)
```

✅ **Output Tab:**
```
Job Logs
[timestamp] [INFO] Starting crawl...
[timestamp] [INFO] Processing URL...
[timestamp] [INFO] Document saved...
```

## Timestamp

Fixes deployed: 2025-11-09T09:28:03+11:00
Documentation created: 2025-11-09T09:35:00+11:00
