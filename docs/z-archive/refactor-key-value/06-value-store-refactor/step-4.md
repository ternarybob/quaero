# Step 4: Rename test file and rewrite tests

**Skill:** @test-writer
**Files:** `internal/storage/sqlite/load_auth_credentials_test.go` → `internal/storage/sqlite/load_auth_only_test.go`

---

## Iteration 1

### Agent 2 - Implementation
Renamed test file to `load_auth_only_test.go` and completely rewrote all tests to verify cookie-based authentication loading. Added comprehensive test coverage for API key skipping, tokens/data fields, and validation rules.

**Changes made:**
- `internal/storage/sqlite/load_auth_credentials_test.go` → `internal/storage/sqlite/load_auth_only_test.go`: File renamed
- Complete test rewrite (457 lines) with 8 test functions:
  1. **TestLoadAuthCredsFromTOML_WithCookieBasedAuth** (lines 14-74): Tests TOML parsing for cookie-based auth sections
     - Verifies Name, SiteDomain, ServiceType, BaseURL, UserAgent fields
     - Tests Tokens and Data inline table parsing
  2. **TestLoadAuthCredsFromTOML_EmptyFile** (lines 76-101): Tests empty file error handling
  3. **TestLoadAuthCredentialsFromFiles_StoresInAuthTable** (lines 103-166): Verifies storage in auth_credentials table
     - Tests that credentials are stored (not in KV store)
     - Verifies all fields are preserved correctly
  4. **TestLoadAuthCredentialsFromFiles_SkipsAPIKeySections** (lines 168-236): **NEW** - Tests API key detection and skipping
     - Mixed TOML with valid cookie-based auth and API key sections
     - Verifies only cookie-based auth is stored
     - Verifies API keys are NOT stored in auth_credentials or KV store
  5. **TestLoadAuthCredentialsFromFiles_WithTokensAndData** (lines 238-291): **NEW** - Tests optional tokens/data fields
     - Verifies Tokens map (access_token, refresh_token)
     - Verifies Data map (region, environment, user_id)
  6. **TestLoadAuthCredentialsFromFiles_DirectoryNotFound** (lines 293-309): Tests graceful handling of missing directory
  7. **TestLoadAuthCredentialsFromFiles_SkipsNonTOML** (lines 311-351): Tests file type filtering
  8. **TestValidateAuthCredentialFile** (lines 353-456): Complete validation test rewrite
     - Tests required fields: name, site_domain OR base_url
     - Tests that service_type is optional
     - Removed old API key validation tests

**Key differences from old tests:**
- Old tests used API key format (api_key, service_type, description)
- New tests use cookie-based auth format (name, site_domain, base_url, tokens, data)
- Old tests verified storage in KV store
- New tests verify storage in auth_credentials table
- Added API key skipping test (critical new behavior)
- Added tokens/data field tests (new optional fields)

**Commands run:**
```bash
mv internal/storage/sqlite/load_auth_credentials_test.go internal/storage/sqlite/load_auth_only_test.go
go test ./internal/storage/sqlite -run TestLoadAuth -v
go test ./internal/storage/sqlite -run TestValidateAuthCredentialFile -v
go build ./...
```

### Agent 3 - Validation
**Skill:** @test-writer

**Tests:**
✅ All 8 test functions pass:
- TestLoadAuthCredsFromTOML_WithCookieBasedAuth (0.22s)
- TestLoadAuthCredsFromTOML_EmptyFile (0.21s)
- TestLoadAuthCredentialsFromFiles_StoresInAuthTable (0.21s)
- TestLoadAuthCredentialsFromFiles_SkipsAPIKeySections (0.23s)
- TestLoadAuthCredentialsFromFiles_WithTokensAndData (0.21s)
- TestLoadAuthCredentialsFromFiles_DirectoryNotFound (0.21s)
- TestLoadAuthCredentialsFromFiles_SkipsNonTOML (0.24s)
- TestValidateAuthCredentialFile (0.35s with 7 sub-tests)

**Compilation:**
✅ Entire codebase compiles cleanly

**Code Quality:**
✅ Comprehensive test coverage for cookie-based auth
✅ Tests verify storage in correct table (auth_credentials, not KV store)
✅ API key skipping behavior thoroughly tested
✅ Tokens and Data fields properly tested
✅ Validation tests match new requirements (name + site_domain/base_url)
✅ Clear test names that describe expected behavior
✅ Follows existing test patterns (setupTestDB, t.TempDir, etc.)

**Quality Score:** 10/10

**Issues Found:**
None (3 pre-existing test failures in job_deletion_test.go, unrelated to this change)

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
Test file successfully renamed and completely rewritten for cookie-based authentication. All 8 test functions pass with comprehensive coverage of:
- Cookie-based auth TOML parsing
- Storage in auth_credentials table (not KV store)
- API key section detection and skipping
- Optional tokens/data fields
- Validation rules for cookie-based auth

**→ Step 4 COMPLETE - All steps finished**
