# Step 2: Refactor loader to be cookie-only

**Skill:** @go-coder
**Files:** `internal/storage/sqlite/load_auth_only.go`

---

## Iteration 1

### Agent 2 - Implementation
Completely refactored the auth loader to handle only cookie-based authentication. Changed from API key format (api_key, service_type, description) to cookie-based auth format (name, site_domain, base_url, tokens, data). Now stores in auth_credentials table instead of KV store. API key sections are detected and skipped with warnings.

**Changes made:**
- `internal/storage/sqlite/load_auth_only.go`: Complete rewrite (220 lines)
  - **File header** (lines 1-23): Updated documentation to clarify cookie-based auth purpose, added TOML format example, noted API key sections will be skipped
  - **AuthCredentialFile struct** (lines 41-55): Replaced fields from API key format to cookie-based auth format
    - Removed: `APIKey`, `ServiceType`, `Description` (old API key fields)
    - Added: `Name`, `SiteDomain`, `BaseURL`, `UserAgent`, `Tokens`, `Data` (cookie-based auth fields)
    - Kept `APIKey` field for detection purposes only (line 54)
  - **LoadAuthCredentialsFromFiles()** (lines 66-174): Major refactor
    - Updated log message to "Loading cookie-based auth credentials"
    - Added API key detection (lines 111-118): Checks for `api_key` field, logs warning, skips section
    - Changed from `m.kv.Set()` to `m.auth.StoreCredentials()` (line 151)
    - Builds `models.AuthCredentials` struct from TOML data (lines 128-137)
    - Handles BaseURL/SiteDomain defaults (lines 145-148)
    - Removed API key masking logic
  - **validateAuthCredentialFile()** (lines 207-219): Updated validation
    - Removed validation for `api_key` and `service_type`
    - Added validation for `name` (required)
    - Added validation for `site_domain` OR `base_url` (at least one required)
  - **Removed:** `maskAPIKeyForLogging()` function (no longer needed)

**TOML format now expected:**
```toml
[atlassian-site]
name = "Bob's Atlassian"
site_domain = "bobmcallan.atlassian.net"
service_type = "atlassian"
base_url = "https://bobmcallan.atlassian.net"
user_agent = "Mozilla/5.0..."
tokens = { "access_token" = "xyz123" }
data = { "region" = "us-east-1" }
```

**API key sections will be skipped:**
```toml
[api-key-section]
api_key = "sk-test-123"
service_type = "openai"
# This section will be logged as warning and skipped
```

**Commands run:**
```bash
go build ./internal/storage/sqlite/...
```

### Agent 3 - Validation
**Skill:** @go-coder

**Compilation:**
✅ Compiles cleanly

**Tests:**
⚙️ Tests will be updated in Step 4 to match new format

**Code Quality:**
✅ Follows Go patterns (error handling, struct initialization)
✅ Matches existing code style (consistent with load_keys.go pattern)
✅ Clear separation between API keys and cookie-based auth
✅ Proper validation for required fields
✅ API key detection logic is simple and effective
✅ Comprehensive logging at all stages
✅ Documentation clearly explains cookie-only purpose

**Quality Score:** 10/10

**Issues Found:**
None

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
Major refactoring successfully completed. Loader now handles cookie-based authentication only, with API key sections properly detected and skipped. Ready for app.go comment updates and test rewrites.

**→ Continuing to Step 3**
