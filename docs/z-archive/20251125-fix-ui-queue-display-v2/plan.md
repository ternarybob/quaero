# Plan: Fix .env.test API Key Loading for Tests

## Problem Analysis

The test configuration loading has a critical timing issue:

**Current Flow:**
1. `test/common/setup.go` lines 347-354: Loads `.env.test` into `env.EnvVars` map
2. `test/common/setup.go` lines 715-768: Injects env vars into `variables.toml` **BUT**:
   - Looks for `env.EnvVars["GOOGLE_API_KEY"]`
   - But `.env.test` contains `google_places_api_key` and `google_gemini_api_key`
   - **MISMATCH**: Wrong key names are being checked
3. Service starts and loads config
4. `internal/app/app.go` line 247: Loads `variables.toml` from `test/config/variables/` into KV storage
5. `internal/app/app.go` lines 263-274: Performs `{key}` replacement using KV storage
6. **Problem**: The KV storage has fake values from `variables.toml`, not real values from `.env.test`

**Root Cause:**
The injection logic in `setup.go` lines 715-768 uses wrong environment variable names and doesn't match the actual keys in `.env.test`.

## Dependency Analysis

- **Step 1 (Fix)**: Must update `test/common/setup.go` to use correct `.env.test` key names
- **Step 2 (Verify)**: Depends on Step 1 completing successfully
- Both are sequential - can't run tests until fix is applied

## Execution Groups

### Group 1 (Sequential - Implementation)

**1. Fix test/common/setup.go variable injection**
   - Skill: @go-coder
   - Files: test/common/setup.go
   - Complexity: low
   - Critical: no
   - Depends on: none
   - User decision: no

**Changes needed:**
   - Lines 718-738: Update to read `env.EnvVars["google_places_api_key"]` and `env.EnvVars["google_gemini_api_key"]`
   - Ensure all variables from `.env.test` are properly mapped to `variables.toml` entries
   - Variables to inject:
     - `google_gemini_api_key` → `google_gemini_api_key`
     - `google_places_api_key` → both `google_places_api_key` AND `test-google-places-key`

**2. Verify fix with test run**
   - Skill: @test-writer
   - Files: test/ui/queue_test.go
   - Complexity: low
   - Critical: no
   - Depends on: Step 1
   - User decision: no

**Verification steps:**
   - Run `cd test/ui && go test -v -run TestQueue`
   - Check that `{google_places_api_key}` placeholder is replaced
   - Verify Places API calls succeed
   - Check test logs for successful API key injection

### Group 2 (Sequential - Documentation)

**3. Document the fix**
   - Skill: @none
   - Files: docs/features/20251125-fix-ui-queue-display-v2/summary.md
   - Complexity: low
   - Critical: no
   - Depends on: Step 2
   - User decision: no

## Success Criteria

- ✅ `.env.test` variables are correctly injected into `variables.toml` during test setup
- ✅ `{google_places_api_key}` placeholder in `test-quaero.toml` is replaced with real API key
- ✅ `test/ui/queue_test.go` passes successfully
- ✅ Places API calls complete without authentication errors
- ✅ Test logs show "Injected API keys into variables.toml"

## Technical Details

### Files to Modify

**test/common/setup.go (lines 715-768)**
Current code checks for wrong keys:
```go
if key := env.EnvVars["GOOGLE_API_KEY"]; key != "" { // WRONG
```

Should check for:
```go
if key := env.EnvVars["google_places_api_key"]; key != "" { // CORRECT
if key := env.EnvVars["google_gemini_api_key"]; key != "" { // CORRECT
```

### Configuration Flow (Corrected)

1. Test loads `.env.test` → `env.EnvVars["google_places_api_key"]`
2. Test injects into `variables.toml` → `google_places_api_key.value = "AIza..."`
3. Service starts → Loads `variables.toml` into KV storage
4. Service loads config → Replaces `{google_places_api_key}` with value from KV storage
5. Places API service → Uses replaced value from config
6. **Result**: Real API key is used, test passes

## Parallel Execution Map

```
[Step 1: Fix setup.go] ──> [Step 2: Verify test] ──> [Step 3: Document]

All sequential (no parallelization possible)
```

## Implementation Notes

- This is a straightforward bug fix - no architectural changes
- No breaking changes - only affects test setup
- Can be completed in single pass with minimal changes
- Low risk - isolated to test infrastructure
