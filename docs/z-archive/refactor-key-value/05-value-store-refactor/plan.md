# Plan: Create Dedicated Key/Value Loader

## Overview
Create a dedicated key/value loader infrastructure that is separate from auth credentials. The new loader will read TOML files from a `./keys` directory with a simpler format (`value` + optional `description`) and store them in the KV store. This separation prepares for Phase 6 where auth will become cookie-only.

## Steps

1. **Create KeysDirConfig struct**
   - Skill: @go-coder
   - Files: `internal/common/keys_config.go` (new)
   - User decision: no
   - Create new config struct with `Dir` field for keys directory path

2. **Update main Config to support keys directory**
   - Skill: @go-coder
   - Files: `internal/common/config.go`
   - User decision: no
   - Add `Keys KeysDirConfig` field, default to `./keys`, add env var override

3. **Create key/value file loader**
   - Skill: @go-coder
   - Files: `internal/storage/sqlite/load_keys.go` (new)
   - User decision: no
   - Implement `LoadKeysFromFiles()` method with simpler TOML format (value + description)

4. **Integrate loader into app initialization**
   - Skill: @go-coder
   - Files: `internal/app/app.go`
   - User decision: no
   - Call new loader after auth credentials loading, before API key migration

5. **Create comprehensive unit tests**
   - Skill: @test-writer
   - Files: `internal/storage/sqlite/load_keys_test.go` (new)
   - User decision: no
   - Test all loader scenarios: valid files, empty files, missing directory, validation

## Success Criteria
- New `LoadKeysFromFiles()` method loads keys from `./keys` directory
- Config supports `Keys.Dir` configuration and `QUAERO_KEYS_DIR` env var
- TOML format is simpler: `[key-name]` with `value` (required) and `description` (optional)
- Loader is called during app initialization after auth credentials
- All unit tests pass (6 test cases)
- Code follows existing patterns from `load_auth_credentials.go`
- Non-fatal error handling: missing directory or invalid files log warnings but don't fail startup
