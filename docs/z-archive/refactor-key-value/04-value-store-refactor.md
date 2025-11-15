I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current State Analysis:**

The codebase has successfully completed Phases 1-3 of the Auth/KV separation:
- Phase 1: KV store infrastructure created (`internal/services/kv/`, `internal/storage/sqlite/kv_storage.go`)
- Phase 2: Runtime `{key-name}` replacement implemented (`internal/common/replacement.go`)
- Phase 3: API key migration completed (`MigrateAPIKeysToKVStore()`, `ResolveAPIKey()` with dual-storage fallback)

**Current API Key Footprint in Auth System:**
1. **Schema**: `auth_credentials` table has `api_key TEXT` and `auth_type TEXT` columns with CHECK constraint
2. **Storage Layer**: `GetAPIKeyByName()` method in `AuthStorage` interface and implementation
3. **Model**: `AuthCredentials` struct has `APIKey` and `AuthType` fields
4. **Handlers**: Four API key-specific HTTP handlers (`CreateAPIKeyHandler`, `GetAPIKeyHandler`, `UpdateAPIKeyHandler`, `DeleteAPIKeyHandler`)
5. **Routes**: `/api/auth/api-key` endpoints in `routes.go`
6. **File Loading**: `load_auth_credentials.go` now loads API keys into KV store (Phase 3 change)
7. **Tests**: API key test cases in `auth_storage_test.go` and `auth_config_test.go`
8. **Backward Compatibility**: `ResolveAPIKey()` still checks auth storage as fallback

**Breaking Changes Impact:**
- API key CRUD endpoints will be removed (users must use KV store endpoints instead)
- Auth file loading already migrated to KV store (no breaking change here)
- `ResolveAPIKey()` backward compatibility can be safely removed (migration completed in Phase 3)
- Tests referencing API keys in auth storage need updates

**Key Decision Points:**
1. **Schema Migration**: SQLite schema is rebuilt on startup (no ALTER TABLE needed), but existing databases will have orphaned API key data
2. **Test Updates**: `auth_config_test.go` expects API keys in auth list - needs refactoring to check KV store instead
3. **Route Cleanup**: Remove `/api/auth/api-key/*` routes entirely (no replacement needed - KV store has its own endpoints)
4. **Model Cleanup**: Remove `APIKey` and `AuthType` fields from `AuthCredentials` struct (breaking change for any external consumers)

### Approach

**Phased Removal Strategy:**

This cleanup follows a **bottom-up approach** - removing dependencies from lowest to highest layers:
1. **Schema & Storage** - Remove database columns and storage methods
2. **Model** - Remove struct fields
3. **Handlers & Routes** - Remove HTTP endpoints
4. **Config Resolution** - Remove backward compatibility fallback
5. **Tests** - Update to reflect new architecture

**No Data Migration Needed:** Since Phase 3 already migrated API keys to KV store and `ResolveAPIKey()` prioritizes KV store, removing auth storage support is safe. Any remaining API keys in `auth_credentials` are orphaned data that will be ignored.

**Validation Strategy:** Existing tests will fail if API key removal breaks cookie-based auth (our safety net). New tests should verify KV store is the sole source for API keys.

### Reasoning

Analyzed the codebase structure by reading relevant files (`schema.go`, `auth_storage.go`, `auth_handler.go`, `routes.go`, `config.go`, test files) and searched for all references to `api_key`, `auth_type`, and API key-related methods using grep. Identified 7 files requiring changes and confirmed Phase 3 migration is complete (API keys already loading into KV store, `ResolveAPIKey()` already prioritizes KV store).

## Mermaid Diagram

sequenceDiagram
    participant User
    participant Config as config.go
    participant KVStore as KeyValueStorage
    participant AuthStore as AuthStorage (Cookie Only)
    participant Schema as SQLite Schema

    Note over Schema: Phase 4: Remove API Key Support
    
    rect rgb(255, 200, 200)
        Note over Schema,AuthStore: 1. Schema Cleanup
        Schema->>Schema: Remove api_key column
        Schema->>Schema: Remove auth_type column
        Schema->>Schema: Remove CHECK constraint
        Schema->>Schema: Remove idx_auth_name_type
    end

    rect rgb(200, 255, 200)
        Note over AuthStore: 2. Storage Layer Cleanup
        AuthStore->>AuthStore: Remove GetAPIKeyByName()
        AuthStore->>AuthStore: Remove GetCredentialsByName()
        AuthStore->>AuthStore: Simplify StoreCredentials()
        AuthStore->>AuthStore: Remove api_key from queries
    end

    rect rgb(200, 200, 255)
        Note over Config: 3. Remove Backward Compatibility
        Config->>Config: Remove authStorage param
        Config->>KVStore: Query API key (primary)
        Config->>Config: Fallback to config value
        Note over Config,AuthStore: Auth storage fallback removed
    end

    rect rgb(255, 255, 200)
        Note over User: 4. API Endpoint Cleanup
        User->>User: Remove /api/auth/api-key/* routes
        User->>KVStore: Use /api/kv/* for API keys
        User->>AuthStore: Use /api/auth/* for cookies only
    end

    Note over User,Schema: Result: Clean Separation
    Note over KVStore: API Keys → KV Store
    Note over AuthStore: Cookies → Auth Storage

## Proposed File Changes

### internal\storage\sqlite\schema.go(MODIFY)

**Remove API key columns from `auth_credentials` table:**

1. Delete `api_key TEXT,` column (line 23)
2. Delete `auth_type TEXT NOT NULL DEFAULT 'cookie',` column (line 24)
3. Delete CHECK constraint `CHECK (auth_type IN ('cookie', 'api_key'))` (line 30)
4. Delete unique index `idx_auth_name_type` on `(name, auth_type)` (line 35) - no longer needed since name uniqueness is per site_domain for cookies
5. Update comment on line 14 to remove mention of API key storage: "Site-based authentication for cookie-based web services"

**Rationale:** The `auth_credentials` table should only store cookie-based authentication. API keys are now exclusively in `key_value_store` table. The CHECK constraint and unique index were only needed to support dual auth types.

### internal\storage\sqlite\auth_storage.go(MODIFY)

**Remove API key logic from `StoreCredentials()` method:**

1. Delete lines 84-127 (entire `else` block handling API keys with empty `site_domain`)
2. Simplify line 83 condition - remove `else` branch since all credentials now have `site_domain`
3. Delete lines 134-137 (auth_type defaulting logic in update path)
4. Delete lines 170-173 (auth_type defaulting logic in insert path)
5. Remove `api_key` and `auth_type` from UPDATE query (lines 140-158)
6. Remove `api_key` and `auth_type` from INSERT query (lines 175-182)
7. Update all SELECT queries to remove `api_key` and `auth_type` columns:
   - `GetCredentialsByID()` (lines 203-209)
   - `GetCredentialsBySiteDomain()` (lines 241-247)
   - `ListCredentials()` (lines 296-313)
   - `GetCredentials()` (lines 342-353)

**Delete entire methods:**

8. Delete `GetCredentialsByName()` method (lines 403-459) - was primarily for API key lookups
9. Delete `isLikelyAPIKeyName()` helper (lines 462-480)
10. Delete `handleGetCredentialsError()` helper (lines 482-488)
11. Delete `GetAPIKeyByName()` method (lines 490-515)

**Rationale:** All API key-related logic is obsolete. Cookie credentials always have `site_domain`, so the dual-path logic (lines 84-127) is unnecessary. The `GetCredentialsByName()` method was added specifically for API key lookups and is no longer needed.

### internal\interfaces\storage.go(MODIFY)

**Remove API key methods from `AuthStorage` interface:**

1. Delete `GetCredentialsByName(ctx context.Context, name string) (*models.AuthCredentials, error)` (line 20)
2. Delete `GetAPIKeyByName(ctx context.Context, name string) (string, error)` (line 21)

**Rationale:** These methods were added to support API key storage in auth_credentials table. With API keys now in KV store, they are no longer part of the auth storage contract. Cookie-based credentials are retrieved by ID or site domain, not by name.

### internal\models\auth.go(MODIFY)

**Remove API key fields from `AuthCredentials` struct:**

1. Delete `APIKey      string                 \`json:"api_key"\`` field (line 13)
2. Delete `AuthType    string                 \`json:"auth_type"\`` field (line 14)
3. Update struct comment (lines 1-4) to remove mention of API key storage: "AuthCredentials represents stored cookie-based authentication data for web services"

**Rationale:** The `AuthCredentials` model should only represent cookie-based authentication. API keys are now stored as simple key-value pairs in the KV store, not as structured credentials.

### internal\handlers\auth_handler.go(MODIFY)

**Remove API key-specific handlers:**

1. Delete `CreateAPIKeyHandler()` method (lines 203-275)
2. Delete `GetAPIKeyHandler()` method (lines 277-343)
3. Delete `UpdateAPIKeyHandler()` method (lines 345-435)
4. Delete `DeleteAPIKeyHandler()` method (lines 437-489)

**Update `ListAuthHandler()` to remove API key masking:**

5. Delete lines 137-150 (API key masking logic in list response)
6. Delete lines 148-150 specifically (the `if cred.AuthType == "api_key"` block)

**Update `GetAuthHandler()` to remove API key masking:**

7. Delete lines 195-198 (API key masking logic in detail response)

**Delete helper function:**

8. Delete `maskAPIKey()` function (lines 24-30) - no longer needed

**Rationale:** All API key CRUD operations should now use KV store endpoints (not auth endpoints). The masking logic is obsolete since auth credentials no longer contain API keys. Users managing API keys should use `/api/kv/*` endpoints instead.

### internal\server\routes.go(MODIFY)

**Remove API key routes from `handleAuthRoutes()` method:**

1. Delete entire block handling `/api/auth/api-key` endpoints (lines 191-219)
2. This includes:
   - POST `/api/auth/api-key` (create)
   - GET `/api/auth/api-key/{id}` (get by ID)
   - PUT `/api/auth/api-key/{id}` (update)
   - DELETE `/api/auth/api-key/{id}` (delete)

**Simplify route logic:**

3. After deletion, `handleAuthRoutes()` should only handle `/api/auth/{id}` for cookie credentials (lines 221-232 remain unchanged)

**Rationale:** API key management endpoints are removed. Users should use KV store endpoints for API key CRUD operations. Cookie-based auth credentials continue to use `/api/auth/{id}` endpoints.

### internal\common\config.go(MODIFY)

References: 

- internal\services\llm\gemini_service.go
- internal\services\agents\service.go
- internal\services\places\service.go
- internal\jobs\manager\agent_manager.go
- internal\jobs\manager\places_search_manager.go
- internal\handlers\job_definition_handler.go

**Remove backward compatibility fallback from `ResolveAPIKey()` function:**

1. Delete lines 638-647 (auth storage fallback block)
2. Update function signature to remove `authStorage` parameter: `func ResolveAPIKey(ctx context.Context, kvStorage interfaces.KeyValueStorage, name string, configFallback string) (string, error)`
3. Update function comment (lines 624-626) to remove mention of auth storage: "Resolution order: KV store → config fallback → error"
4. Update error message on line 655 to remove mention of auth storage: `"API key '%s' not found in KV store or config"`

**Update all callers of `ResolveAPIKey()`:**

5. In `internal/services/llm/gemini_service.go` - remove `authStorage` argument from `ResolveAPIKey()` call
6. In `internal/services/agents/service.go` - remove `authStorage` argument from `ResolveAPIKey()` call
7. In `internal/services/places/service.go` - remove `authStorage` argument from `ResolveAPIKey()` call
8. In `internal/jobs/manager/agent_manager.go` - remove `authStorage` argument from `ResolveAPIKey()` call (line 65)
9. In `internal/jobs/manager/places_search_manager.go` - remove `authStorage` argument from `ResolveAPIKey()` call (line 76)
10. In `internal/handlers/job_definition_handler.go` - remove `authStorage` argument from `ResolveAPIKey()` call (line 586)

**Rationale:** Phase 3 migration is complete - all API keys are in KV store. The auth storage fallback was temporary for backward compatibility during migration. Removing it simplifies the API and enforces KV store as the single source of truth for API keys.

### internal\storage\sqlite\auth_storage_test.go(MODIFY)

**Remove API key test cases:**

1. Delete `TestStoreCredentials_WithAPIKey()` test function (lines 13-61) - tests API key storage in auth_credentials
2. Delete `TestGetCredentialsByName_WithAuthType()` test function (lines 63-108) - tests name-based lookup for API keys
3. Delete `TestGetAPIKeyByName()` test function (lines 110-158) - tests `GetAPIKeyByName()` method
4. Delete `TestResolveAPIKey()` test function (lines 160-186) - tests API key resolution from auth storage
5. Delete `TestListCredentials_IncludesAuthType()` test function (lines 188-236) - tests auth_type field in list response

**Rationale:** All deleted tests verify API key functionality in auth storage, which is being removed. Cookie-based auth tests remain unchanged. API key functionality should be tested via KV store tests instead.

### test\api\auth_config_test.go(MODIFY)

**Update `TestAuthConfigLoading()` to verify KV store instead of auth storage:**

1. Change test to call `/api/kv/list` endpoint instead of `/api/auth/list` (line 23)
2. Update response parsing to expect KV store format: `[]map[string]interface{}` with `key`, `value`, `description` fields
3. Update verification logic (lines 40-68) to check for `test-google-places-key` in KV store list
4. Remove `auth_type` and `service_type` checks (lines 42-43) - KV store doesn't have these fields
5. Update assertion to verify key exists in KV store with correct description
6. Update log messages to reflect KV store instead of auth storage

**Delete or update `TestAuthConfigAPIKeyEndpoint()`:**

7. Option A: Delete entire test (lines 76-151) since `/api/auth/api-key/*` endpoints are removed
8. Option B: Refactor to test KV store endpoints (`/api/kv/{key}`) instead

**Rationale:** Auth file loading now stores API keys in KV store (Phase 3 change in `load_auth_credentials.go`). Tests must verify KV store instead of auth storage. The API key CRUD endpoint test is obsolete since those endpoints are being removed.

### internal\storage\sqlite\load_auth_credentials.go(MODIFY)

**Update comments to reflect KV store usage (no code changes needed):**

1. Update file header comment (lines 1-3) to clarify this file loads API keys into KV store, not auth_credentials
2. Update `AuthCredentialFile` struct comment (lines 19-21) to remove mention of `models.AuthCredentials` - this struct is now only for parsing TOML files
3. Delete `ToAuthCredentials()` method (lines 28-46) - no longer used since we load directly to KV store
4. Update `LoadAuthCredentialsFromFiles()` function comment (lines 48-50) to clarify it loads API keys to KV store

**Rationale:** This file was already updated in Phase 3 to load API keys into KV store (lines 102-114). The `ToAuthCredentials()` method is dead code since we no longer create `AuthCredentials` objects for API keys. Comments should reflect current behavior.

### internal\storage\sqlite\load_auth_credentials_test.go(MODIFY)

**Update test to verify KV store instead of auth storage:**

1. Update test expectations to verify API keys are loaded into KV store, not auth_credentials table
2. Replace calls to `storage.GetCredentialsByName()` with calls to `kvStorage.Get()` to retrieve API keys
3. Update assertions to check KV store format (key, value, description) instead of `AuthCredentials` struct fields
4. Remove checks for `AuthType`, `SiteDomain`, `BaseURL` fields (lines 29-31, 104-107) - KV store doesn't have these
5. Verify `APIKey` value is stored correctly in KV store as the value field

**Rationale:** The file loading logic now stores API keys in KV store (Phase 3 change). Tests must verify the correct storage location. Auth storage should only contain cookie-based credentials.

### internal\storage\sqlite\migration_test.go(MODIFY)

**Update migration test to remove auth storage references:**

1. Update test setup to create API keys in auth_credentials table for migration testing (lines 25-40 remain as-is for backward compat testing)
2. After migration, verify API keys are in KV store using `kvStorage.Get()` instead of `authStorage.GetAPIKeyByName()`
3. Remove assertions checking auth storage after migration (lines 71-74) - auth storage should no longer contain API keys
4. Update test to verify `ResolveAPIKey()` works with new signature (without `authStorage` parameter)

**Rationale:** Migration test should verify that old API keys in auth_credentials are successfully migrated to KV store. After migration, auth storage should not be checked for API keys. The test validates the migration path for existing deployments.

### internal\services\crawler\logging_test.go(MODIFY)

**Update `MockAuthStorage` to remove API key method:**

1. Delete `GetAPIKeyByName()` method from `MockAuthStorage` struct (lines 329-335)
2. This mock is used for testing crawler logging, not API key functionality

**Rationale:** The mock implements the `AuthStorage` interface, which no longer includes `GetAPIKeyByName()`. Removing this method keeps the mock in sync with the updated interface.