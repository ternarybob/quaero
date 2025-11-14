I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current State:**
- API keys stored in `auth_credentials` table with `auth_type='api_key'`
- `ResolveAPIKey()` queries `AuthStorage.GetAPIKeyByName()` with config fallback
- Services (LLM, Agent, Places) and managers (Agent, Places) use `ResolveAPIKey()`
- KV store infrastructure exists (Phase 1), replacement engine ready (Phase 2)

**Migration Strategy:**
- One-time migration copies API keys from `auth_credentials` to `key_value_store`
- Update `ResolveAPIKey()` to query KV store first, then auth_credentials (backward compat)
- Update `load_auth_credentials.go` to load into KV store instead
- Migration runs automatically on startup (idempotent)
- Phase 4 will remove API key support from auth_credentials entirely

**Key Design Decisions:**
- Use KV store key naming: `{name}` (e.g., `gemini-llm`, `google-places`)
- Migration is idempotent (safe to run multiple times)
- Backward compatibility maintained until Phase 4 cleanup
- No changes to service/manager call sites (transparent migration)

### Approach

Migrate API keys from `auth_credentials` table to `key_value_store` table, update resolution logic to query KV store first with backward compatibility fallback, and run migration automatically on startup. This separates concerns: auth_credentials for cookie-based authentication, key_value_store for API keys and generic key/value pairs. The migration is idempotent and maintains backward compatibility during transition.

### Reasoning

Reviewed all relevant files: `manager.go`, `load_auth_credentials.go`, `config.go`, service files (LLM, Agent, Places), manager files (Agent, Places), `app.go`, `auth_storage.go`, `storage.go` interface, and `auth.go` model. Understood current API key storage in `auth_credentials` table, resolution logic via `ResolveAPIKey()`, and usage patterns across services/managers. Confirmed KV store infrastructure exists and replacement engine is operational.

## Mermaid Diagram

sequenceDiagram
    participant App as app.go
    participant Mgr as Manager
    participant KV as KV Store
    participant Auth as Auth Storage
    participant Svc as Services/Managers

    Note over App,Svc: Startup Sequence

    App->>Mgr: LoadAuthCredentialsFromFiles()
    Mgr->>KV: Set(name, api_key) for API keys
    Mgr->>Auth: StoreCredentials() for cookies
    
    App->>Mgr: MigrateAPIKeysToKVStore()
    Mgr->>Auth: Query auth_type='api_key'
    Auth-->>Mgr: Return API key list
    loop For each API key
        Mgr->>KV: Get(name) - check exists
        alt Not exists
            Mgr->>KV: Set(name, api_key, description)
        else Already exists
            Note over Mgr: Skip (idempotent)
        end
    end
    Mgr-->>App: Migration complete

    Note over App,Svc: Runtime Resolution

    Svc->>App: ResolveAPIKey(kv, auth, name, config)
    App->>KV: Get(name)
    alt Found in KV
        KV-->>App: Return api_key
        App-->>Svc: Return api_key
    else Not in KV
        App->>Auth: GetAPIKeyByName(name)
        alt Found in Auth
            Auth-->>App: Return api_key
            App-->>Svc: Return api_key (legacy)
        else Not in Auth
            alt Config fallback exists
                App-->>Svc: Return config fallback
            else No fallback
                App-->>Svc: Error: not found
            end
        end
    end

## Proposed File Changes

### internal\storage\sqlite\manager.go(MODIFY)

References: 

- internal\interfaces\storage.go
- internal\storage\sqlite\kv_storage.go
- internal\storage\sqlite\auth_storage.go

Add `MigrateAPIKeysToKVStore()` method to Manager struct:

**Purpose:** One-time migration to copy API keys from `auth_credentials` to `key_value_store`

**Implementation:**
- Query all credentials with `auth_type='api_key'` from `auth_credentials` table
- For each API key credential:
  - Extract `name` and `api_key` fields
  - Check if key already exists in KV store via `KeyValueStorage().Get()`
  - If not exists, insert into KV store via `KeyValueStorage().Set()` with description from `Data` field
  - Log migration progress (count migrated, skipped, errors)
- Return error only if critical failure (e.g., database connection lost)
- Idempotent: safe to run multiple times (checks existence before insert)

**Error Handling:**
- Log warnings for individual key migration failures, continue with remaining keys
- Return error only for catastrophic failures
- Track counts: migrated, skipped (already exists), failed

**Logging:**
- Info: "Starting API key migration from auth_credentials to key_value_store"
- Debug: "Migrated API key: {name}" (mask key value)
- Debug: "Skipped API key (already exists): {name}"
- Warn: "Failed to migrate API key: {name}" (with error)
- Info: "API key migration complete: {migrated} migrated, {skipped} skipped, {failed} failed"

### internal\storage\sqlite\load_auth_credentials.go(MODIFY)

References: 

- internal\storage\sqlite\manager.go(MODIFY)
- internal\interfaces\storage.go
- internal\models\auth.go

Update `LoadAuthCredentialsFromFiles()` to load API keys into KV store instead of auth_credentials:

**Changes:**
- After parsing TOML file and validating fields (lines 86-100)
- Check if `auth_type` is `'api_key'` (from `ToAuthCredentials()` line 40)
- If API key:
  - Extract `name`, `api_key`, `description` from `AuthCredentialFile`
  - Call `m.kv.Set(ctx, name, api_key, description)` to store in KV store
  - Log: "Loaded API key into KV store: {name}" (mask key)
  - Skip `m.auth.StoreCredentials()` call for API keys
- If cookie-based auth (not API key):
  - Keep existing logic: call `m.auth.StoreCredentials()` (line 106)
  - Log: "Loaded cookie credentials: {name}"

**Backward Compatibility:**
- Existing TOML files work unchanged (same format)
- API keys go to KV store, cookies go to auth_credentials
- No breaking changes to file format

**Error Handling:**
- Log warnings for individual key load failures
- Continue processing remaining files
- Update `loadedCount` and `skippedCount` appropriately

**Note:** Manager struct already has `kv` field (line 17) from Phase 1

### internal\common\config.go(MODIFY)

References: 

- internal\interfaces\storage.go
- internal\interfaces\kv_storage.go

Update `ResolveAPIKey()` function to query KV store first, then fall back to auth_credentials:

**Current Signature (line 627):**
```
func ResolveAPIKey(ctx context.Context, authStorage interfaces.AuthStorage, name string, configFallback string) (string, error)
```

**New Signature:**
```
func ResolveAPIKey(ctx context.Context, kvStorage interfaces.KeyValueStorage, authStorage interfaces.AuthStorage, name string, configFallback string) (string, error)
```

**Resolution Order:**
1. **KV Store (Primary):** Query `kvStorage.Get(ctx, name)`
   - If found and non-empty, return immediately
   - Log debug: "Resolved API key from KV store: {name}"
2. **Auth Storage (Backward Compat):** Query `authStorage.GetAPIKeyByName(ctx, name)`
   - If found and non-empty, return
   - Log debug: "Resolved API key from auth storage (legacy): {name}"
3. **Config Fallback:** Return `configFallback` if non-empty
   - Log debug: "Using config fallback for API key: {name}"
4. **Error:** Return error if all sources fail
   - Error message: "API key '{name}' not found in KV store, auth storage, or config"

**Error Handling:**
- Graceful degradation: if KV store query fails, log warning and try auth storage
- Only return error if all three sources fail
- Preserve existing error messages for backward compatibility

**Note:** This maintains backward compatibility during migration period. Phase 4 will remove auth storage fallback.

### internal\services\llm\gemini_service.go(MODIFY)

References: 

- internal\common\config.go(MODIFY)
- internal\interfaces\storage.go
- internal\app\app.go(MODIFY)

Update `NewGeminiService()` to pass KV storage to `ResolveAPIKey()`:

**Current Call (line 107):**
```
apiKey, err := common.ResolveAPIKey(ctx, authStorage, "gemini-llm", config.LLM.GoogleAPIKey)
```

**New Call:**
```
apiKey, err := common.ResolveAPIKey(ctx, config.Storage.KeyValueStorage(), authStorage, "gemini-llm", config.LLM.GoogleAPIKey)
```

**Changes:**
- Add `config.Storage.KeyValueStorage()` as first parameter
- Keep all other parameters unchanged
- No changes to error handling or logging

**Note:** Requires passing full `*common.Config` instead of just `*common.LLMConfig` to access `Storage.KeyValueStorage()`. Update function signature:
- Current: `NewGeminiService(config *common.Config, authStorage interfaces.AuthStorage, logger arbor.ILogger)`
- Already correct (line 104) - no signature change needed

**Access Pattern:**
- `config` parameter is `*common.Config` (full config)
- Access KV storage via `config.Storage` (not available directly)
- **Alternative:** Pass `StorageManager` instead of `AuthStorage`, then access both via manager

**Recommended Approach:**
- Change signature to accept `storageManager interfaces.StorageManager` instead of `authStorage interfaces.AuthStorage`
- Access both: `storageManager.KeyValueStorage()` and `storageManager.AuthStorage()`
- Update call in `app.go` (line 288) to pass `a.StorageManager` instead of `a.StorageManager.AuthStorage()`

### internal\services\agents\service.go(MODIFY)

References: 

- internal\common\config.go(MODIFY)
- internal\interfaces\storage.go
- internal\app\app.go(MODIFY)

Update `NewService()` to pass KV storage to `ResolveAPIKey()`:

**Current Call (line 61):**
```
apiKey, err := common.ResolveAPIKey(ctx, authStorage, "gemini-agent", config.GoogleAPIKey)
```

**New Call:**
```
apiKey, err := common.ResolveAPIKey(ctx, storageManager.KeyValueStorage(), storageManager.AuthStorage(), "gemini-agent", config.GoogleAPIKey)
```

**Signature Change:**
- Current: `NewService(config *common.AgentConfig, authStorage interfaces.AuthStorage, logger arbor.ILogger)`
- New: `NewService(config *common.AgentConfig, storageManager interfaces.StorageManager, logger arbor.ILogger)`
- Replace `authStorage` parameter with `storageManager`
- Access both storages via manager methods

**Update Call Site in `app.go` (line 435):**
- Current: `agents.NewService(&a.Config.Agent, a.StorageManager.AuthStorage(), a.Logger)`
- New: `agents.NewService(&a.Config.Agent, a.StorageManager, a.Logger)`
- Pass full `StorageManager` instead of just `AuthStorage()`

### internal\services\places\service.go(MODIFY)

References: 

- internal\common\config.go(MODIFY)
- internal\interfaces\storage.go
- internal\app\app.go(MODIFY)

Update `NewService()` to pass KV storage to `ResolveAPIKey()`:

**Current Call (line 37):**
```
apiKey, err := common.ResolveAPIKey(ctx, authStorage, "google-places", config.APIKey)
```

**New Call:**
```
apiKey, err := common.ResolveAPIKey(ctx, storageManager.KeyValueStorage(), storageManager.AuthStorage(), "google-places", config.APIKey)
```

**Signature Change:**
- Current: `NewService(config *common.PlacesAPIConfig, authStorage interfaces.AuthStorage, eventService interfaces.EventService, logger arbor.ILogger)`
- New: `NewService(config *common.PlacesAPIConfig, storageManager interfaces.StorageManager, eventService interfaces.EventService, logger arbor.ILogger)`
- Replace `authStorage` parameter with `storageManager`
- Access both storages via manager methods

**Update Call Site in `app.go` (line 407):**
- Current: `places.NewService(&a.Config.PlacesAPI, a.StorageManager.AuthStorage(), a.EventService, a.Logger)`
- New: `places.NewService(&a.Config.PlacesAPI, a.StorageManager, a.EventService, a.Logger)`
- Pass full `StorageManager` instead of just `AuthStorage()`

### internal\jobs\manager\agent_manager.go(MODIFY)

References: 

- internal\common\config.go(MODIFY)
- internal\interfaces\storage.go
- internal\app\app.go(MODIFY)

Update `CreateParentJob()` to pass KV storage to `ResolveAPIKey()`:

**Current Call (line 62):**
```
resolvedAPIKey, err := common.ResolveAPIKey(ctx, m.authStorage, apiKeyName, "")
```

**New Call:**
```
resolvedAPIKey, err := common.ResolveAPIKey(ctx, m.kvStorage, m.authStorage, apiKeyName, "")
```

**Struct Changes:**
- Add `kvStorage interfaces.KeyValueStorage` field to `AgentManager` struct (after `authStorage` field, line 22)
- Update `NewAgentManager()` constructor (line 29):
  - Add `kvStorage interfaces.KeyValueStorage` parameter
  - Assign to struct field: `kvStorage: kvStorage`

**Update Call Site in `app.go` (line 480):**
- Current: `manager.NewAgentManager(jobMgr, queueMgr, a.SearchService, a.StorageManager.AuthStorage(), a.Logger)`
- New: `manager.NewAgentManager(jobMgr, queueMgr, a.SearchService, a.StorageManager.KeyValueStorage(), a.StorageManager.AuthStorage(), a.Logger)`
- Add `a.StorageManager.KeyValueStorage()` parameter before `a.StorageManager.AuthStorage()`

### internal\jobs\manager\places_search_manager.go(MODIFY)

References: 

- internal\common\config.go(MODIFY)
- internal\interfaces\storage.go
- internal\app\app.go(MODIFY)

Update `CreateParentJob()` to pass KV storage to `ResolveAPIKey()`:

**Current Call (line 73):**
```
resolvedAPIKey, err := common.ResolveAPIKey(ctx, m.authStorage, apiKeyName, "")
```

**New Call:**
```
resolvedAPIKey, err := common.ResolveAPIKey(ctx, m.kvStorage, m.authStorage, apiKeyName, "")
```

**Struct Changes:**
- Add `kvStorage interfaces.KeyValueStorage` field to `PlacesSearchManager` struct (after `authStorage` field, line 22)
- Update `NewPlacesSearchManager()` constructor (line 29):
  - Add `kvStorage interfaces.KeyValueStorage` parameter
  - Assign to struct field: `kvStorage: kvStorage`

**Update Call Site in `app.go` (line 474):**
- Current: `manager.NewPlacesSearchManager(a.PlacesService, a.DocumentService, a.EventService, a.StorageManager.AuthStorage(), a.Logger)`
- New: `manager.NewPlacesSearchManager(a.PlacesService, a.DocumentService, a.EventService, a.StorageManager.KeyValueStorage(), a.StorageManager.AuthStorage(), a.Logger)`
- Add `a.StorageManager.KeyValueStorage()` parameter before `a.StorageManager.AuthStorage()`

### internal\app\app.go(MODIFY)

References: 

- internal\storage\sqlite\manager.go(MODIFY)
- internal\services\llm\gemini_service.go(MODIFY)
- internal\services\agents\service.go(MODIFY)
- internal\services\places\service.go(MODIFY)
- internal\jobs\manager\agent_manager.go(MODIFY)
- internal\jobs\manager\places_search_manager.go(MODIFY)

Run API key migration automatically on startup:

**Location:** In `initDatabase()` method, after loading auth credentials from files (line 229)

**Implementation:**
- After `LoadAuthCredentialsFromFiles()` completes successfully
- Call migration: `sqliteMgr.MigrateAPIKeysToKVStore(ctx)`
- Log migration result:
  - Success: "API key migration completed successfully"
  - Failure: "API key migration failed" (warning, not error - don't fail startup)
- Migration is idempotent, safe to run on every startup

**Error Handling:**
- Log warning if migration fails, but don't fail startup
- Migration errors are non-critical (backward compatibility maintained)
- Services will fall back to auth_credentials if KV store migration incomplete

**Placement:**
```
// After line 229 (after LoadAuthCredentialsFromFiles)
if err := sqliteMgr.MigrateAPIKeysToKVStore(ctx); err != nil {
    a.Logger.Warn().Err(err).Msg("API key migration failed - services will use legacy auth storage")
} else {
    a.Logger.Info().Msg("API key migration completed successfully")
}
```

**Update Service Initialization Calls:**
- Line 288: `llm.NewGeminiService()` - Change to pass `a.StorageManager` instead of `a.StorageManager.AuthStorage()`
- Line 407: `places.NewService()` - Change to pass `a.StorageManager` instead of `a.StorageManager.AuthStorage()`
- Line 435: `agents.NewService()` - Change to pass `a.StorageManager` instead of `a.StorageManager.AuthStorage()`
- Line 474: `manager.NewPlacesSearchManager()` - Add `a.StorageManager.KeyValueStorage()` parameter
- Line 480: `manager.NewAgentManager()` - Add `a.StorageManager.KeyValueStorage()` parameter

**Note:** All service/manager signature changes documented in their respective file change entries above.

### internal\storage\sqlite\migration_test.go(NEW)

References: 

- internal\storage\sqlite\manager.go(MODIFY)
- internal\storage\sqlite\kv_storage_test.go
- internal\storage\sqlite\auth_storage_test.go

Create comprehensive test suite for API key migration:

**Test Cases:**

1. **TestMigrateAPIKeysToKVStore_Success:**
   - Setup: Create in-memory SQLite DB with test schema
   - Insert 3 API keys into `auth_credentials` with `auth_type='api_key'`
   - Insert 2 cookie credentials with `auth_type='cookie'`
   - Run migration: `MigrateAPIKeysToKVStore()`
   - Verify: All 3 API keys exist in `key_value_store` with correct values
   - Verify: Cookie credentials remain in `auth_credentials` (not migrated)
   - Verify: Migration logs show "3 migrated, 0 skipped, 0 failed"

2. **TestMigrateAPIKeysToKVStore_Idempotent:**
   - Setup: Insert API keys into both `auth_credentials` and `key_value_store`
   - Run migration twice
   - Verify: No duplicates created
   - Verify: Second run logs show "0 migrated, 3 skipped, 0 failed"
   - Verify: Values unchanged after second migration

3. **TestMigrateAPIKeysToKVStore_EmptyDatabase:**
   - Setup: Empty database (no credentials)
   - Run migration
   - Verify: No errors
   - Verify: Migration logs show "0 migrated, 0 skipped, 0 failed"

4. **TestMigrateAPIKeysToKVStore_MixedAuthTypes:**
   - Setup: Insert mix of `api_key` and `cookie` credentials
   - Run migration
   - Verify: Only `api_key` credentials migrated to KV store
   - Verify: Cookie credentials remain in `auth_credentials` only

5. **TestMigrateAPIKeysToKVStore_PartialFailure:**
   - Setup: Insert API keys, simulate KV store failure for one key
   - Run migration
   - Verify: Other keys still migrated successfully
   - Verify: Migration continues despite individual failures
   - Verify: Failed count incremented appropriately

**Test Utilities:**
- `setupTestDB()` - Create in-memory SQLite with schema
- `insertTestAPIKey()` - Helper to insert test API key
- `insertTestCookieCredential()` - Helper to insert test cookie credential
- `verifyKVStoreContains()` - Assert key exists in KV store with expected value
- `verifyAuthStorageContains()` - Assert credential exists in auth_credentials

**Assertions:**
- Use `testify/assert` for readable assertions
- Verify counts, values, and error handling
- Check log output for expected messages

**Note:** Follow existing test patterns in `c:/development/quaero/internal/storage/sqlite/kv_storage_test.go` for consistency.