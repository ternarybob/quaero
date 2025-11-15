I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current Implementation Analysis:**

The `load_auth_credentials.go` file currently loads API keys from TOML files with sections containing `api_key`, `service_type`, and `description` fields, storing them in the `key_value_store` table via `m.kv.Set()`. This was implemented in Phase 3 of the Auth/KV separation project as a migration step.

**Key Files:**
- `internal/storage/sqlite/load_auth_credentials.go` - Current API key loader (106 lines)
- `internal/storage/sqlite/load_auth_credentials_test.go` - Tests validating API key loading to KV store
- `internal/app/app.go` - Calls `LoadAuthCredentialsFromFiles(ctx, a.Config.Auth.CredentialsDir)` at line 223
- `internal/models/auth.go` - Defines `AuthCredentials` struct for cookie-based auth
- `internal/storage/sqlite/auth_storage.go` - Implements `StoreCredentials()` for auth_credentials table

**Schema Context:**
- `auth_credentials` table: id, name, site_domain, service_type, data, cookies, tokens, base_url, user_agent, created_at, updated_at
- `key_value_store` table: key, value, description, created_at, updated_at

**User Requirements:**
1. Load only cookie-based auth from `./auth` directory
2. Skip sections with `api_key` field (log warning)
3. Store in `auth_credentials` table (not KV store)
4. Update documentation to clarify cookie-only purpose
5. Optionally rename file to `load_auth_only.go`
6. Add test to verify API key sections are skipped
7. Keep `./auth` path in `app.go`

**Design Decision:**
Cookie-based auth files are rare since the Chrome extension typically captures credentials, but file-based loading is useful for testing, CI/CD, or manual setup scenarios where the extension isn't available.

### Approach

Refactor the auth credentials loader to be **cookie-only** by changing the TOML structure from API key format (`api_key`, `service_type`, `description`) to cookie-based auth format (`name`, `site_domain`, `service_type`, `base_url`, etc.), storing credentials in the `auth_credentials` table instead of the KV store. Any sections containing `api_key` field will be skipped with a warning log, directing users to use the `./keys` directory instead. The file will be optionally renamed to `load_auth_only.go` for clarity, with updated documentation emphasizing cookie-only functionality. Tests will be updated to verify API key sections are properly skipped and cookie-based auth is correctly loaded.

### Reasoning

Listed the repository structure to understand the codebase layout, then read the key files mentioned by the user (`load_auth_credentials.go` and `app.go`) to understand the current implementation. Examined the `AuthCredentials` model and `AuthStorage` interface to understand the expected cookie-based auth structure. Reviewed the existing test file to understand current behavior and test patterns. Searched for the schema definition to understand table structures for both `auth_credentials` and `key_value_store`. This exploration revealed that the current loader is API-key focused and needs to be completely refactored to support cookie-based auth instead.

## Mermaid Diagram

sequenceDiagram
    participant App as app.go
    participant Loader as load_auth_only.go
    participant AuthStorage as auth_storage.go
    participant DB as SQLite (auth_credentials)
    
    App->>Loader: LoadAuthCredentialsFromFiles(ctx, "./auth")
    Loader->>Loader: Read TOML files from ./auth directory
    
    loop For each TOML section
        Loader->>Loader: Parse section into AuthCredentialFile
        
        alt Section has api_key field
            Loader->>Loader: Log warning: "Skipping API key section"
            Loader->>Loader: Increment skippedCount
        else Section is cookie-based auth
            Loader->>Loader: Validate required fields (name, site_domain/base_url)
            Loader->>Loader: Build models.AuthCredentials struct
            Loader->>AuthStorage: StoreCredentials(ctx, credentials)
            AuthStorage->>DB: INSERT/UPDATE auth_credentials table
            DB-->>AuthStorage: Success
            AuthStorage-->>Loader: Success
            Loader->>Loader: Log info: "Loaded cookie-based auth"
            Loader->>Loader: Increment loadedCount
        end
    end
    
    Loader-->>App: Return (loaded: N, skipped: M)
    
    Note over Loader,DB: API keys now handled by<br/>load_keys.go → key_value_store table

## Proposed File Changes

### internal\storage\sqlite\load_auth_credentials.go → internal\storage\sqlite\load_auth_only.go

Rename file to `load_auth_only.go` to clearly indicate this loader is dedicated to cookie-based authentication only, not API keys. This naming makes the separation of concerns explicit and prevents confusion with the new `load_keys.go` file that handles API keys.

### internal\storage\sqlite\load_auth_only.go(NEW)

References: 

- internal\models\auth.go
- internal\storage\sqlite\auth_storage.go
- internal\storage\sqlite\load_keys.go

**Complete refactor of the auth loader to be cookie-only:**

1. **Update file header comments** (lines 1-14):
   - Change description from "Load API Keys from Files" to "Load Cookie-Based Authentication from Files"
   - Document that this is for cookie-based auth only (captured via Chrome extension or manual TOML files)
   - Clarify that API keys should use `./keys` directory and `load_keys.go` instead
   - Add TOML format example showing cookie-based auth structure

2. **Replace `AuthCredentialFile` struct** (lines 26-36):
   - Remove fields: `APIKey`, `ServiceType`, `Description`
   - Add fields matching `models.AuthCredentials`: `Name`, `SiteDomain`, `ServiceType`, `BaseURL`, `UserAgent`, `Tokens` (as map[string]string), `Data` (as map[string]interface{})
   - Update TOML tags accordingly
   - Add comment explaining this matches the `auth_credentials` table schema

3. **Refactor `LoadAuthCredentialsFromFiles()` method** (lines 38-132):
   - Update function comment to clarify cookie-only loading
   - Change log message from "Loading auth credentials" to "Loading cookie-based auth credentials"
   - In the section processing loop (lines 90-122):
     - **Add API key detection**: Check if section has `api_key` field (via type assertion or reflection)
     - If `api_key` present: Log warning with message "Skipping API key section '{sectionName}' - API keys should be in ./keys directory, not ./auth", increment `skippedCount`, continue to next section
     - If no `api_key`: Proceed with cookie-based auth loading
   - Replace `m.kv.Set()` call (line 106) with `m.auth.StoreCredentials()` call
   - Build `models.AuthCredentials` struct from parsed TOML data
   - Handle optional fields (tokens, data, user_agent) with defaults if missing
   - Update log messages to reflect auth storage (not KV store)
   - Remove API key masking logic (lines 112-119) since we're not handling API keys

4. **Update `loadAuthCredsFromTOML()` helper** (lines 134-156):
   - Change return type from `map[string]*AuthCredentialFile` to `map[string]*AuthCredentialFile` (keep same but struct is different)
   - Update function comment to clarify cookie-based auth parsing
   - Keep section-based parsing logic (works for both formats)

5. **Refactor `validateAuthCredentialFile()` method** (lines 158-172):
   - Remove validation for `api_key` field
   - Remove validation for `service_type` field (optional for cookie auth)
   - Add validation for required cookie-based auth fields: `site_domain` or `base_url` (at least one required)
   - Add validation for `name` field (required)
   - Update error messages to reflect cookie-based auth requirements

6. **Remove `maskAPIKeyForLogging()` helper** (lines 174-180):
   - Delete this method entirely since we're not handling API keys

**Key Implementation Notes:**
- Use `m.auth.StoreCredentials(ctx, &credentials)` instead of `m.kv.Set()`
- The `AuthStorage.StoreCredentials()` method handles ID generation, timestamp setting, and upsert logic (see `internal/storage/sqlite/auth_storage.go` lines 35-142)
- Cookies field will typically be empty when loading from files (captured via extension), but can be manually added for testing
- Tokens field can be specified as TOML inline table: `tokens = { "token1" = "value1", "token2" = "value2" }`
- Data field can be specified as TOML inline table for service-specific metadata

### internal\app\app.go(MODIFY)

References: 

- internal\storage\sqlite\load_auth_only.go(NEW)

**Update function call to match renamed file:**

No functional changes needed, but update the comment at line 220 to clarify the purpose:
- Change comment from "Load auth credentials from files (after job definitions)" to "Load cookie-based auth credentials from files (after job definitions)"
- Add comment: "Note: API keys are loaded separately via LoadKeysFromFiles() below"
- The method call `sqliteMgr.LoadAuthCredentialsFromFiles(ctx, a.Config.Auth.CredentialsDir)` remains unchanged (method name stays the same despite file rename)
- Keep the path as `a.Config.Auth.CredentialsDir` which defaults to `./auth`

**Rationale:** The method name `LoadAuthCredentialsFromFiles()` is still accurate since it loads authentication credentials (just cookie-based, not API keys). Renaming the method would require updating all callers and isn't necessary for clarity.

### internal\storage\sqlite\load_auth_credentials_test.go → internal\storage\sqlite\load_auth_only_test.go

Rename test file to match the renamed implementation file (`load_auth_only.go`). This maintains the Go convention of colocating tests with implementation files.

### internal\storage\sqlite\load_auth_only_test.go(NEW)

References: 

- internal\storage\sqlite\load_auth_only.go(NEW)
- internal\storage\sqlite\auth_storage.go
- internal\models\auth.go

**Complete rewrite of tests to validate cookie-based auth loading:**

1. **Replace `TestLoadAuthCredsFromTOML_WithSections`** (lines 14-64):
   - Rename to `TestLoadAuthCredsFromTOML_CookieBasedAuth`
   - Create test TOML with cookie-based auth sections:
     ```toml
     [atlassian-site]
     name = "Bob's Atlassian"
     site_domain = "bobmcallan.atlassian.net"
     service_type = "atlassian"
     base_url = "https://bobmcallan.atlassian.net"
     user_agent = "Mozilla/5.0"
     
     [github-site]
     name = "GitHub Enterprise"
     site_domain = "github.example.com"
     service_type = "github"
     base_url = "https://github.example.com"
     ```
   - Verify sections are parsed correctly with expected fields
   - Assert `Name`, `SiteDomain`, `ServiceType`, `BaseURL` fields match expected values

2. **Keep `TestLoadAuthCredsFromTOML_EmptyFile`** (lines 66-91):
   - No changes needed - still validates empty file handling

3. **Replace `TestLoadAuthCredentialsFromFiles_StoresInKV`** (lines 93-157):
   - Rename to `TestLoadAuthCredentialsFromFiles_StoresInAuthTable`
   - Create test TOML with cookie-based auth (not API keys)
   - Call `LoadAuthCredentialsFromFiles()` with test directory
   - Verify credentials stored in `auth_credentials` table using `m.auth.GetCredentialsBySiteDomain()`
   - Assert fields match expected values (name, site_domain, service_type, base_url)
   - Remove KV store assertions (lines 130-156)

4. **Add new test: `TestLoadAuthCredentialsFromFiles_SkipsAPIKeySections`**:
   - Create test TOML with mixed sections:
     ```toml
     [valid-cookie-auth]
     name = "Valid Site"
     site_domain = "example.com"
     service_type = "generic"
     base_url = "https://example.com"
     
     [invalid-api-key]
     api_key = "sk-test-key-12345"
     service_type = "openai"
     description = "Should be skipped"
     ```
   - Call `LoadAuthCredentialsFromFiles()`
   - Verify only `valid-cookie-auth` is stored in auth_credentials table
   - Verify `invalid-api-key` is NOT in auth_credentials table
   - Check log output contains warning about skipped API key section
   - Assert loaded count is 1, skipped count is 1

5. **Add new test: `TestLoadAuthCredentialsFromFiles_WithTokensAndData`**:
   - Create test TOML with inline tables for tokens and data:
     ```toml
     [site-with-tokens]
     name = "Site with Tokens"
     site_domain = "api.example.com"
     service_type = "api"
     base_url = "https://api.example.com"
     tokens = { "access_token" = "token123", "refresh_token" = "refresh456" }
     data = { "api_version" = "v2", "region" = "us-east-1" }
     ```
   - Verify tokens and data are correctly parsed and stored
   - Use `m.auth.GetCredentialsBySiteDomain()` to retrieve and assert JSON fields

6. **Update `TestValidateAuthCredentialFile`** (lines 159-229):
   - Replace test cases to validate cookie-based auth fields
   - Test cases:
     - Valid credentials with all fields
     - Missing `name` (should error)
     - Missing both `site_domain` and `base_url` (should error)
     - Valid with only `base_url` (site_domain derived from base_url)
     - Valid with only `site_domain` (base_url constructed from site_domain)
   - Remove API key validation test cases

**Test Coverage Goals:**
- Cookie-based auth loading and storage
- API key section detection and skipping
- Mixed TOML files (valid + invalid sections)
- Optional fields (tokens, data, user_agent)
- Validation logic for required fields
- Error handling for malformed TOML