# Plan: Settings API Key Warning Tests

## Overview
Create comprehensive UI tests for the settings page API key management, covering scenarios where the `google_api_key` is both set and not set, and verifying that the "Configuration Required" warning displays correctly on the jobs page.

## Context
- **Current State:** All tests currently show the "Configuration Required" warning because `google_api_key` is not set in the test TOML files
- **Target:** Test both scenarios: (1) warning displayed when key is NOT set, (2) warning NOT displayed when key IS set
- **Config:** Use `test/config/test-quaero-no-variables.toml` as the base config for no-variables scenario
- **Jobs Page:** Warning appears at `jobs.html` line 115-122 with text "Configuration Required: {runtime_error}"

## Steps

### 1. **Analyze existing test patterns and config setup**
   - Skill: @none
   - Files:
     - `test/ui/settings_apikeys_test.go`
     - `test/ui/jobs_agent_disabled_test.go`
     - `test/config/test-quaero-no-variables.toml`
     - `test/config/test-quaero-apikeys.toml`
   - User decision: no
   - **Action:** Understand how existing tests navigate to settings, add API keys, and check for warnings. Understand the config file structure and variables directory setup.

### 2. **Create test for google_api_key NOT set scenario**
   - Skill: @test-writer
   - Files:
     - `test/ui/settings_apikey_warning_test.go` (new)
     - `test/config/variables-no-variables/` (directory structure)
   - User decision: no
   - **Action:** Create `TestSettingsAPIKeyWarning_NotSet` that:
     1. Uses `test-quaero-no-variables.toml` config (no google_api_key in TOML)
     2. Navigates to `/jobs` page
     3. Verifies "Configuration Required" warning IS displayed
     4. Extracts the runtime_error text
     5. Verifies it mentions Google API key
     6. Takes screenshots for verification

### 3. **Create test for adding google_api_key via settings page**
   - Skill: @test-writer
   - Files:
     - `test/ui/settings_apikey_warning_test.go` (add test function)
   - User decision: no
   - **Action:** Create `TestSettingsAPIKeyWarning_AddKey` that:
     1. Uses `test-quaero-no-variables.toml` config (start with no key)
     2. Navigates to `/settings?a=auth-apikeys` page
     3. Clicks "Add New Variable" or equivalent button
     4. Fills in key name as "google_api_key"
     5. Fills in a test value
     6. Saves the new API key
     7. Verifies key appears in the list
     8. Navigates to `/jobs` page
     9. Verifies "Configuration Required" warning IS STILL displayed (because runtime status is checked at startup/LIST call, not dynamically)
     10. Takes screenshots at each step

### 4. **Create test for google_api_key set in TOML scenario**
   - Skill: @test-writer
   - Files:
     - `test/ui/settings_apikey_warning_test.go` (add test function)
     - `test/config/test-quaero-with-apikey.toml` (new config file)
   - User decision: no
   - **Action:**
     1. Create new config file `test-quaero-with-apikey.toml` that sets `agent.google_api_key = "test-key-value"`
     2. Create `TestSettingsAPIKeyWarning_KeySet` that:
        - Uses the new config with API key set
        - Navigates to `/jobs` page
        - Verifies "Configuration Required" warning is NOT displayed
        - Verifies agent jobs do NOT have runtime_status="disabled"
        - Takes screenshots for verification

### 5. **Run all tests and validate functionality**
   - Skill: @go-coder
   - Files:
     - `test/ui/settings_apikey_warning_test.go`
   - User decision: no
   - **Action:**
     1. Run all three test functions
     2. Verify screenshots show expected behavior
     3. Ensure tests pass with proper assertions
     4. Document any findings about runtime validation behavior

## Success Criteria
- ✅ Test verifies warning IS displayed when `google_api_key` is NOT set in TOML
- ✅ Test can add `google_api_key` via settings page (UI interaction works)
- ✅ Test verifies warning is NOT displayed when `google_api_key` IS set in TOML
- ✅ All tests compile and run without errors
- ✅ Screenshots captured at key steps for visual verification
- ✅ Tests follow existing patterns from `settings_apikeys_test.go` and `jobs_agent_disabled_test.go`

## Notes
- The "Configuration Required" warning is driven by `runtime_status === 'disabled'` and `runtime_error` field
- Runtime validation happens server-side when listing job definitions
- Adding a key via settings/KV storage may not immediately update runtime status (requires service restart or re-validation)
- Tests should document this behavior clearly
