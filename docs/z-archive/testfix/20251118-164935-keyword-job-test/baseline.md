# Baseline Test Results

**Test File:** test/ui/keyword_job_test.go
**Test Command:** `cd test/ui && go test -v -run TestKeywordJob`
**Timestamp:** 2025-11-18T16:49:35+11:00

## Test Output
```
=== RUN   TestKeywordJob
    ✓ Test environment ready, service running at: http://localhost:18085
    === PHASE 1: Places Job - Document Creation ===
    ✓ Places job definition created/exists
    ✓ Places job execution button clicked and dialog accepted
    ✓ Found Places parent job: 4c658edd-d336-4b7c-a1b0-88ce76618c63
      Job 4c658edd status: failed (document_count: 0)
    ⚠️  Places job failed (expected without Google Places API key):
        job failed: API error: REQUEST_DENIED - You must use an API key to authenticate
    ✅ PHASE 1 PASS: Job executed via UI and failure properly tracked

    === PHASE 2: Keyword Extraction Agent Job - Error Handling ===
    ✓ Keyword Extraction job definition created/exists
    ✓ Keyword Extraction job execution button clicked and dialog accepted
    ✓ Found Keyword Extraction parent job: efb14dbb-5a39-4c63-afb3-ae04bc10cb24
      Job efb14dbb status: completed
    ✅ PHASE 2 PASS: Job executed and status properly displayed in UI
--- PASS: TestKeywordJob (29.78s)
```

## Failures Identified

### Issue 1: Test doesn't actually use Google API key
- **Test:** TestKeywordJob
- **Expected:** Google API key from `.env.test` should be loaded and used for Google Places API calls
- **Actual:** API key is loaded by `setup.go` into `env.EnvVars` but never upserted into KV storage
- **Result:** Places job fails with "REQUEST_DENIED - You must use an API key"
- **Source:** `test/ui/keyword_job_test.go` - missing API key upsert before executing jobs

### Issue 2: Test doesn't validate successful job execution
- **Test:** TestKeywordJob
- **Expected:** With API key configured, Places job should complete successfully and create documents
- **Actual:** Test passes even when API key is missing (treats failure as expected)
- **Source:** `test/ui/keyword_job_test.go:293-306` - accepts both success and failure

## Source Files to Fix
- `test/ui/keyword_job_test.go` - Add API key upsert logic after environment setup

## Dependencies
- ✅ `test/config/.env.test` exists with `GOOGLE_API_KEY`
- ✅ `setup.go` loads `.env.test` into `env.EnvVars` map
- ❌ Missing: API call to upsert API key into KV storage before job execution

## Test Statistics
- **Total Tests:** 1
- **Passing:** 1 (but not testing the intended functionality)
- **Failing:** 0
- **Skipped:** 0

## Required Changes

### 1. Upsert Google API Key to KV Storage
After test environment setup, before creating job definitions:
- Use HTTP helper to PUT `/api/kv/GOOGLE_API_KEY` with value from `env.EnvVars["GOOGLE_API_KEY"]`
- Verify the key was stored successfully
- This will make the API key available to the Places API integration

### 2. Update Test Expectations
Phase 1 should expect:
- Places job to complete successfully (not fail)
- Document count > 0 (actual restaurants found)
- Remove the "expected without API key" failure handling

**→ Starting Iteration 1**
