# Plan: Rename 'Keys'/'API Keys' to 'Variables' in User-Facing Contexts

## Problem Statement

The KV service (`internal/services/kv/service.go`) is currently referred to as "Keys" or "API Keys" in user-facing contexts (settings page, UI tests, documentation). This terminology is ambiguous and confusing because:
1. "Keys" could mean API keys, cryptographic keys, or key-value pair keys
2. The service actually stores generic variables, not just API keys
3. The directory is named "keys" which doesn't reflect its broader purpose

## Goals

1. Update all user-facing references from "Keys"/"API Keys" to "Variables"
2. Update configuration references to use "Variables" terminology
3. Include directory and filename references in documentation
4. Maintain backward compatibility in code (keep internal "kv" naming)

## Steps

1. **Update TOML configuration files**
   - Skill: @none
   - Files: `bin/quaero.toml`, `deployments/local/quaero.toml`, `test/config/test-quaero-apikeys.toml`
   - User decision: no
   - Update comments to reference "Variables" instead of "Keys"
   - Document the default directory (`./keys`) and filename (`*.toml`) in comments

2. **Update UI test files**
   - Skill: @none
   - Files: `test/ui/settings_apikeys_test.go`, `test/ui/settings_test.go`
   - User decision: no
   - Update test names, comments, and log messages to use "Variables" terminology
   - Keep test file names as-is for backward compatibility

3. **Update handler documentation**
   - Skill: @none
   - Files: `internal/handlers/kv_handler.go`
   - User decision: no
   - Update comments to reference "Variables" instead of "API Keys"

4. **Update common config documentation**
   - Skill: @none
   - Files: `internal/common/keys_config.go`, `internal/common/config.go`
   - User decision: no
   - Update struct comments to clarify "Variables" purpose
   - Document default directory path

5. **Update storage layer documentation**
   - Skill: @none
   - Files: `internal/storage/sqlite/load_keys.go`, `internal/storage/sqlite/kv_storage.go`
   - User decision: no
   - Update function comments to reference "Variables" terminology

6. **Update API test documentation**
   - Skill: @none
   - Files: `test/api/auth_config_test.go`, `test/api/config_dynamic_injection_test.go`
   - User decision: no
   - Update test comments to use "Variables" terminology

7. **Update application initialization logs**
   - Skill: @go-coder
   - Files: `internal/app/app.go`
   - User decision: no
   - Update log messages to reference "Variables" instead of "Keys"

8. **Compile and verify**
   - Skill: @go-coder
   - Files: All modified files
   - User decision: no
   - Ensure all code compiles
   - Run quick smoke test

## Success Criteria

- All user-facing references updated to "Variables"
- Configuration comments document directory (`./keys`) and file patterns (`*.toml`)
- Log messages use "Variables" terminology
- Code still compiles and runs
- Internal code references remain as "kv" (backward compatible)
- No breaking changes to APIs or file paths

## Technical Notes

- Keep all file/directory names unchanged (`keys/`, `test-keys.toml`, etc.)
- Keep internal code references as "kv" (KeyValueStorage, kvService, etc.)
- Only update user-facing strings (comments, logs, documentation)
- This is a documentation/UX improvement, not a refactoring
