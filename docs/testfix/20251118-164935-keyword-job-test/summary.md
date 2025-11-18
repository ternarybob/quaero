# Test Fix Summary: keyword_job_test.go

**Test File:** `test/ui/keyword_job_test.go`
**Started:** 2025-11-18T16:49:35+11:00
**Completed:** 2025-11-18T16:58:47+11:00
**Duration:** ~9 minutes
**Iterations:** 1

---

## Executive Summary

Successfully fixed the test infrastructure to load and pass Google API keys from `.env.test` to the Quaero service during initialization. The original problem (missing API key authentication) has been **completely resolved**. The test now fails due to an **external API configuration issue** (Google Cloud project doesn't have legacy Places API enabled), not a test infrastructure problem.

**Status:** ✅ Test infrastructure fix complete (API configuration issue remains)

---

## Original Problem

The test `TestKeywordJob` was designed to:
1. Execute a Google Places "nearby restaurants" job
2. Execute a Keyword Extraction agent job
3. Validate error handling and UI display

However, even though `.env.test` contained a valid Google API key, jobs were failing with:
```
REQUEST_DENIED - You must use an API key to authenticate
```

---

## Root Cause Analysis

### Issue 1: Service Initialization Timing
- `test/common/setup.go` loads `.env.test` into `env.EnvVars` map (lines 326-343) ✅
- However, services initialize during `SetupTestEnvironment()` ❌
- Services cache API keys at initialization in `NewService()` ❌
- **Problem:** No way to pass API keys to services before they initialize

### Issue 2: Environment Variable Naming
- Places service expects: `QUAERO_PLACES_API_KEY`
- Gemini service expects: `QUAERO_GEMINI_GOOGLE_API_KEY`
- `.env.test` provides: `GOOGLE_API_KEY`
- **Problem:** Environment variable name mismatch

### Initial Failed Approach
We initially tried to upsert API keys to KV storage after the service started, but this didn't work because:
1. Services initialize and cache API keys during `NewService()`
2. This happens in `SetupTestEnvironment()` before the test can make API calls
3. By the time we could upsert to KV storage, services already initialized with empty keys

---

## Solution Implemented

### Approach: Environment Variable Mapping

Pass `.env.test` variables as **process environment variables** to the service, with proper name mapping, BEFORE the service process starts.

### Changes Made

#### 1. `test/common/setup.go` (lines 805-824)

Added environment variable mapping and passing logic in `startService()`:

```go
// Pass .env.test variables to service process
// Map generic environment variable names to service-specific names
envMappings := map[string][]string{
	"GOOGLE_API_KEY": {"QUAERO_PLACES_API_KEY", "QUAERO_GEMINI_GOOGLE_API_KEY"},
}

for envKey, envValue := range env.EnvVars {
	// Check if this env var needs to be mapped to multiple service env vars
	if targetEnvVars, ok := envMappings[envKey]; ok {
		// Map to all target environment variables
		for _, targetEnvVar := range targetEnvVars {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", targetEnvVar, envValue))
			fmt.Fprintf(env.LogFile, "  Env: %s=***REDACTED*** (mapped from %s)\n", targetEnvVar, envKey)
		}
	} else {
		// No mapping - use as-is
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", envKey, envValue))
		fmt.Fprintf(env.LogFile, "  Env: %s=***REDACTED***\n", envKey)
	}
}
```

**Why this works:**
- Environment variables are set BEFORE the service process starts
- Services read these during `NewService()` initialization
- Solves the timing problem by making keys available early

#### 2. `test/ui/keyword_job_test.go`

Updated test expectations to require successful job completion:

```go
// Lines 296-311: Updated Phase 1 completion handling
if err != nil {
	// Job failed unexpectedly
	env.LogTest(t, "ERROR: Places job failed: %v", err)
	t.Fatalf("Places job failed: %v", err)
} else {
	// Job succeeded
	env.LogTest(t, "✓ Places job completed with %d documents", placesDocCount)

	// Verify documents were created
	if placesDocCount == 0 {
		env.LogTest(t, "ERROR: Places job created 0 documents")
		t.Fatalf("Places job should have created documents but got 0")
	} else {
		env.LogTest(t, "✅ PHASE 1 PASS: Places job created %d documents", placesDocCount)
	}
}
```

Removed unused imports: `io`, `net/http`, `net/url`

---

## Test Results

### Before Fix (Baseline)
```
⚠️  Places job failed (expected without Google Places API key):
    job failed: API error: REQUEST_DENIED - You must use an API key to authenticate
```

### After Fix (Iteration 1)
```
ERROR: Places job failed: job failed: failed to search places: API error:
REQUEST_DENIED - You're calling a legacy API, which is not enabled for your
project. To get newer features and more functionality, switch to the Places
API (New) or Routes API.
```

### Analysis of Results

**✅ What Worked:**
1. API key is now loaded from `.env.test` ✅
2. API key is passed to service as environment variables ✅
3. Services correctly read environment variables during initialization ✅
4. Google API receives the API key and authenticates the request ✅

**❌ New Issue (External API Configuration):**
- Google Cloud project doesn't have the **legacy Google Places API** enabled
- The error is coming FROM Google, not from Quaero
- The API key is valid and working - just the wrong API is enabled

**This is NOT a test infrastructure problem.** The test infrastructure is working correctly.

---

## Recommendations

### Option 1: Enable Legacy API (Quick Fix)
Enable the "Places API (Legacy)" in Google Cloud Console:
1. Go to Google Cloud Console
2. Navigate to "APIs & Services" > "Enabled APIs & services"
3. Search for "Places API" (the legacy one)
4. Enable it for this project

### Option 2: Update to New API (Long-term Solution)
Update `internal/services/places/service.go` to use the new "Places API (New)":
- Migration guide: https://developers.google.com/maps/legacy
- New API offers better features and functionality
- Recommended by Google for new projects

---

## Files Modified

1. **test/common/setup.go**
   - Lines 805-824: Added environment variable mapping logic
   - Maps `.env.test` variables to service-specific environment variables
   - Passes env vars to service process before initialization

2. **test/ui/keyword_job_test.go**
   - Lines 3-12: Removed unused imports (`io`, `net/http`, `net/url`)
   - Lines 296-311: Updated Phase 1 expectations to require success
   - Removed unused `upsertEnvVarsToKV()` function

---

## Documentation Generated

1. **baseline.md** - Initial test run showing the original problem
2. **iteration-1.md** - Complete documentation of the fix implementation
3. **summary.md** - This document

---

## Conclusion

**Problem Solved:** ✅ API key authentication now works correctly

**Test Infrastructure:** ✅ Complete and working

**External Dependency:** ❌ Google Cloud project needs API enabled

The test infrastructure changes are **production-ready** and should be kept. The remaining test failure is due to Google Cloud project configuration, which requires either:
1. Enabling the legacy Places API in Google Cloud Console, OR
2. Updating the Quaero Places service to use the new Places API

The environment variable mapping approach is **extensible** - additional API keys can be easily added to the `envMappings` map in `setup.go` for future tests.
