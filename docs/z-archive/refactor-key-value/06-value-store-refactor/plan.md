# Plan: Refactor Auth Loader to Be Cookie-Only

## Overview
Refactor the auth credentials loader to handle **only cookie-based authentication** by changing from API key format to cookie-based auth format, storing credentials in the `auth_credentials` table instead of the KV store. Any sections containing `api_key` field will be skipped with warnings. This completes the separation: auth for cookies, keys for API keys.

## Steps

1. **Rename load_auth_credentials.go to load_auth_only.go**
   - Skill: @go-coder
   - Files: `internal/storage/sqlite/load_auth_credentials.go` → `internal/storage/sqlite/load_auth_only.go`
   - User decision: no
   - Rename file to clarify cookie-only purpose

2. **Refactor loader to be cookie-only**
   - Skill: @go-coder
   - Files: `internal/storage/sqlite/load_auth_only.go`
   - User decision: no
   - Complete refactor: new struct fields, skip API keys, store in auth_credentials table

3. **Update app.go comments**
   - Skill: @go-coder
   - Files: `internal/app/app.go`
   - User decision: no
   - Clarify that auth loader is for cookies only, API keys loaded separately

4. **Rename test file and rewrite tests**
   - Skill: @test-writer
   - Files: `internal/storage/sqlite/load_auth_credentials_test.go` → `internal/storage/sqlite/load_auth_only_test.go`
   - User decision: no
   - Complete test rewrite for cookie-based auth, add API key skipping test

## Success Criteria
- Auth loader only processes cookie-based auth (site_domain, base_url, etc.)
- API key sections are detected and skipped with warnings
- Credentials stored in `auth_credentials` table (not KV store)
- Tests verify cookie-based auth loading and API key skipping
- All existing tests pass or are updated appropriately
- Clear documentation about cookie-only purpose
