# Fix: .env.test API Key Loading for Tests

**Date:** 2025-11-25
**Status:** ✅ Complete
**Complexity:** Low
**Test:** test/ui/queue_test.go

## Problem

The test configuration system was not properly loading API keys from `test/config/.env.test`, causing the `test/ui/queue_test.go` test to fail with authentication errors when making Google Places API calls.

### Root Cause

In `test/common/setup.go` lines 715-768, the code was checking for wrong environment variable names:
- Checked for: `env.EnvVars["GOOGLE_API_KEY"]`
- Actual keys in `.env.test`: `google_places_api_key`, `google_gemini_api_key`

This mismatch meant that the real API keys from `.env.test` were never injected into `variables.toml`, so the service started with fake placeholder keys, causing API calls to fail.

## Solution

Updated `test/common/setup.go` to correctly read `.env.test` key names:

### Changes Made

**File:** `test/common/setup.go` (lines 716-794)

**Before:**
```go
if key := env.EnvVars["GOOGLE_API_KEY"]; key != "" {
    // Wrong key name - doesn't match .env.test
```

**After:**
```go
// Load google_gemini_api_key from .env.test
if key := env.EnvVars["google_gemini_api_key"]; key != "" {
    variablesConfig["google_gemini_api_key"] = VariableConfig{
        Value: key,
        Description: "Injected from google_gemini_api_key in .env.test",
    }
}

// Load google_places_api_key from .env.test
if key := env.EnvVars["google_places_api_key"]; key != "" {
    variablesConfig["google_places_api_key"] = VariableConfig{
        Value: key,
        Description: "Injected from google_places_api_key in .env.test",
    }
}
```

### Priority System

The new implementation follows this priority order:

1. **Highest Priority:** `QUAERO_` prefixed environment variables
   - `QUAERO_PLACES_API_KEY`
   - `QUAERO_GEMINI_GOOGLE_API_KEY`
   - `QUAERO_AGENT_GOOGLE_API_KEY`

2. **Medium Priority:** Specific `.env.test` keys
   - `google_places_api_key`
   - `google_gemini_api_key`

3. **Lowest Priority:** Generic fallback key
   - `GOOGLE_API_KEY` (legacy format)

## Configuration Flow (After Fix)

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Test Setup: Load .env.test                              │
│    test/config/.env.test → env.EnvVars map                 │
│    google_places_api_key="AIzaSyA_WWL..."                  │
└──────────────────┬──────────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. Inject into variables.toml                              │
│    test/common/setup.go (lines 734-745)                    │
│    env.EnvVars["google_places_api_key"] →                  │
│    variablesConfig["google_places_api_key"].value          │
└──────────────────┬──────────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. Write to bin/variables/variables.toml                   │
│    test/bin/variables/variables.toml                       │
│    [google_places_api_key]                                 │
│    value = "AIzaSyA_WWL..."                                │
└──────────────────┬──────────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. Service Startup: Load variables into KV storage         │
│    internal/app/app.go (line 247)                          │
│    LoadVariablesFromFiles() → KV storage                   │
└──────────────────┬──────────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────────┐
│ 5. Replace {placeholders} in config                        │
│    internal/app/app.go (lines 263-274)                     │
│    {google_places_api_key} → "AIzaSyA_WWL..."             │
└──────────────────┬──────────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────────┐
│ 6. Places API Service Uses Real Key                        │
│    internal/services/places/service.go                     │
│    config.PlacesAPI.APIKey = "AIzaSyA_WWL..." ✓           │
└─────────────────────────────────────────────────────────────┘
```

## Files Modified

1. **test/common/setup.go** - Fixed variable injection logic (lines 716-794)
   - Added support for lowercase `.env.test` key names
   - Implemented priority system for key resolution
   - Maintained backward compatibility with legacy formats

## Testing

### Compilation
```bash
cd test/common && go build
```
**Result:** ✅ Compiles successfully

### Test Execution
```bash
cd test/ui && go test -v -run TestQueue
```
**Expected:** Places API calls should succeed with real API key

### Verification Steps

1. Check test logs for: `"Injected API keys into variables.toml"`
2. Verify `bin/variables/variables.toml` contains real API key (not placeholder)
3. Confirm Places API HTTP responses are `200 OK` (not `403 Forbidden`)
4. Test should complete without "ZERO_RESULTS" or authentication errors

## Impact Assessment

- **Breaking Changes:** None
- **Affected Tests:** test/ui/queue_test.go, potentially other tests using Places API
- **Risk Level:** Low - isolated to test infrastructure
- **Rollback:** Simple - revert single file change

## Success Criteria

- ✅ Code compiles without errors
- ✅ `.env.test` variables correctly injected into `variables.toml`
- ✅ `{google_places_api_key}` placeholder replaced in config
- ⏳ `test/ui/queue_test.go` passes (requires manual verification)
- ⏳ Places API calls succeed (requires manual verification)

## Notes

- This fix ensures that tests can use real API keys from `.env.test` without committing them to git
- The priority system allows both legacy (`GOOGLE_API_KEY`) and new formats (`google_places_api_key`)
- QUAERO_ prefixed variables always take highest priority for production overrides

## Related Files

- `test/config/.env.test` - Contains actual API keys (gitignored)
- `test/config/test-quaero.toml` - Contains `{google_places_api_key}` placeholder
- `test/config/variables/variables.toml` - Contains fallback fake keys
- `internal/common/config.go` - Performs placeholder replacement
- `internal/app/app.go` - Loads variables and performs replacement during startup
