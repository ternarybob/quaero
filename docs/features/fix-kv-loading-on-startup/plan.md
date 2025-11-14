# Plan: Fix Key/Value Loading on Startup

## Problem Statement

The service startup should load key/values from `bin\keys\example-keys.toml`, but the settings page shows no key/values upon restart. Analysis reveals:

1. **Log Evidence (line 48)**: `WRN > file=example-keys.toml section=google-places-key error=value is required Key/value validation failed`
2. **Root Cause**: The example keys file uses legacy format with `api_key` field, but the loader expects `value` field
3. **Example File Format (bin/keys/example-keys.toml)**:
   ```toml
   [google-places-key]
   api_key = "AIzaSyCwXVa0E5aCDmCg9FlhPeX8ct83E9EADFg"
   service_type = "google-places"
   description = "Google Places API key for location search functionality"
   ```
4. **Expected Format (per load_keys.go:34)**:
   ```toml
   [google-places-key]
   value = "AIzaSyCwXVa0E5aCDmCg9FlhPeX8ct83E9EADFg"
   description = "Google Places API key for location search functionality"
   ```

## Steps

### Step 1: Validate and clean up deployments/local/quaero.toml
- **Skill:** @none
- **Files:** `deployments/local/quaero.toml`
- **User decision:** no
- **Description:** Review the template TOML file and ensure it follows the documented default settings. The file already has proper comments for the `[keys]` section (lines 123-135) with `credentials_dir = "./auth"` (default). The `[keys]` section should use `dir = "./keys"` as per config.go:224. Verify there are no redundant or incorrect settings.

### Step 2: Fix bin/keys/example-keys.toml format
- **Skill:** @none
- **Files:** `bin/keys/example-keys.toml`
- **User decision:** no
- **Description:** Update the example keys file to use the correct format expected by the loader. Change `api_key` field to `value` field and remove `service_type` field (not used by the KV store - it's metadata for API key authentication, not generic KV pairs).

### Step 3: Verify startup process loads keys correctly
- **Skill:** @go-coder
- **Files:** `internal/storage/sqlite/load_keys.go`, `internal/app/app.go`
- **User decision:** no
- **Description:** Review the startup sequence in app.go to confirm LoadKeysFromFiles is called with the correct path. Check log lines 47-50 in the provided log file to verify the loading process. Ensure the path resolution uses config.Keys.Dir (default: "./keys").

### Step 4: Create UI test for settings API keys loading
- **Skill:** @test-writer
- **Files:** `test/ui/settings_apikeys_test.go` (new)
- **User decision:** no
- **Description:** Create a new test in `./test/ui` that:
  1. Uses `test/config/keys/test-keys.toml` as the keys directory (starts service with `-keys-dir` flag)
  2. Navigates to `/settings?a=auth-apikeys`
  3. Waits for the API keys list to load
  4. Verifies that `test-google-places-key` from `test/config/keys/test-keys.toml` is present
  5. Verifies the masked value is displayed
  6. Tests the "Show Full" toggle functionality

## Success Criteria

- `deployments/local/quaero.toml` has correct and minimal configuration with proper defaults documented
- `bin/keys/example-keys.toml` uses the correct `value` field format
- Service startup successfully loads key/value pairs (no warnings in logs)
- Settings page displays loaded API keys after restart
- UI test verifies API keys loading from `test/config/keys/test-keys.toml`

## Technical Notes

### Configuration Hierarchy
- **Config Structure:** `config.Keys.Dir` (default: "./keys") at common/config.go:224
- **Loader:** `Manager.LoadKeysFromFiles()` at internal/storage/sqlite/load_keys.go:46
- **Validation:** Requires `value` field (line 159), `description` is optional (line 162)

### File Format Requirements
```toml
[section-name]
value = "secret-value"      # Required
description = "Optional description"  # Optional
```

### Startup Sequence (from logs)
1. Line 47: `LoadKeysFromFiles path=./keys Loading key/value pairs from files`
2. Line 48: `WRN > file=example-keys.toml section=google-places-key error=value is required` ❌
3. Line 49: `Finished loading key/value pairs from files skipped=1 loaded=0` ❌
4. Line 50: `Key/value pairs loaded from files dir=./keys`
5. Line 53: `No key/value pairs found, skipping config replacement`

### Test Requirements
- Test file location: `test/config/keys/test-keys.toml` (already exists with correct format)
- Service start: Use custom keys directory via flag or config
- UI navigation: `/settings?a=auth-apikeys`
- Verification: Check for `test-google-places-key` in the displayed list
