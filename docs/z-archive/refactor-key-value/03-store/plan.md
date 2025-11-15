# Plan: Migrate API Keys from auth_credentials to key_value_store

## Overview
Migrate API keys from `auth_credentials` table to `key_value_store` table, update resolution logic to query KV store first with backward compatibility fallback, and run migration automatically on startup. This separates concerns: auth_credentials for cookie-based authentication, key_value_store for API keys and generic key/value pairs.

## Steps

### 1. Add Migration Method to Manager
**Skill:** @go-coder
**Files:**
- `internal/storage/sqlite/manager.go`
- `internal/interfaces/storage.go`
- `internal/storage/sqlite/kv_storage.go`
- `internal/storage/sqlite/auth_storage.go`

**User decision:** no

**Implementation:**
- Add `MigrateAPIKeysToKVStore()` method to Manager struct
- Query all credentials with `auth_type='api_key'` from auth_credentials
- For each API key, check if exists in KV store, insert if not
- Idempotent: safe to run multiple times
- Log migration progress (migrated, skipped, failed counts)

### 2. Update Auth Credentials Loader
**Skill:** @go-coder
**Files:**
- `internal/storage/sqlite/load_auth_credentials.go`
- `internal/storage/sqlite/manager.go`
- `internal/models/auth.go`

**User decision:** no

**Implementation:**
- Update `LoadAuthCredentialsFromFiles()` to detect API key type
- Load API keys into KV store instead of auth_credentials
- Keep cookie-based auth loading to auth_credentials unchanged
- Maintain backward compatibility with existing TOML format

### 3. Update ResolveAPIKey Function
**Skill:** @go-coder
**Files:**
- `internal/common/config.go`
- `internal/interfaces/storage.go`
- `internal/interfaces/kv_storage.go`

**User decision:** no

**Implementation:**
- Add `kvStorage` parameter to `ResolveAPIKey()` signature
- Resolution order: KV store → auth storage (backward compat) → config fallback
- Graceful degradation if KV store query fails
- Maintain existing error messages

### 4. Update LLM Service
**Skill:** @go-coder
**Files:**
- `internal/services/llm/gemini_service.go`
- `internal/app/app.go`

**User decision:** no

**Implementation:**
- Change `NewGeminiService()` signature to accept `StorageManager` instead of `AuthStorage`
- Update `ResolveAPIKey()` call to pass both KV and auth storages
- Update call site in `app.go` to pass full StorageManager

### 5. Update Agent Service
**Skill:** @go-coder
**Files:**
- `internal/services/agents/service.go`
- `internal/app/app.go`

**User decision:** no

**Implementation:**
- Change `NewService()` signature to accept `StorageManager` instead of `AuthStorage`
- Update `ResolveAPIKey()` call to pass both storages
- Update call site in `app.go`

### 6. Update Places Service
**Skill:** @go-coder
**Files:**
- `internal/services/places/service.go`
- `internal/app/app.go`

**User decision:** no

**Implementation:**
- Change `NewService()` signature to accept `StorageManager` instead of `AuthStorage`
- Update `ResolveAPIKey()` call to pass both storages
- Update call site in `app.go`

### 7. Update Agent Manager
**Skill:** @go-coder
**Files:**
- `internal/jobs/manager/agent_manager.go`
- `internal/app/app.go`

**User decision:** no

**Implementation:**
- Add `kvStorage` field to `AgentManager` struct
- Update constructor to accept KV storage parameter
- Update `ResolveAPIKey()` call in `CreateParentJob()`
- Update call site in `app.go`

### 8. Update Places Search Manager
**Skill:** @go-coder
**Files:**
- `internal/jobs/manager/places_search_manager.go`
- `internal/app/app.go`

**User decision:** no

**Implementation:**
- Add `kvStorage` field to `PlacesSearchManager` struct
- Update constructor to accept KV storage parameter
- Update `ResolveAPIKey()` call in `CreateParentJob()`
- Update call site in `app.go`

### 9. Run Migration on Startup
**Skill:** @go-coder
**Files:**
- `internal/app/app.go`
- `internal/storage/sqlite/manager.go`

**User decision:** no

**Implementation:**
- Call `MigrateAPIKeysToKVStore()` in `initDatabase()` after loading auth credentials
- Log migration success/failure
- Don't fail startup on migration errors (backward compatibility maintained)

### 10. Create Migration Tests
**Skill:** @test-writer
**Files:**
- `internal/storage/sqlite/migration_test.go` (NEW)
- `internal/storage/sqlite/kv_storage_test.go` (reference)
- `internal/storage/sqlite/auth_storage_test.go` (reference)

**User decision:** no

**Implementation:**
- Test successful migration of API keys
- Test idempotency (running migration twice)
- Test empty database scenario
- Test mixed auth types (API keys + cookies)
- Test partial failure handling
- Follow existing test patterns for consistency

## Success Criteria
- ✅ API keys migrate from auth_credentials to key_value_store on startup
- ✅ Migration is idempotent (safe to run multiple times)
- ✅ `ResolveAPIKey()` checks KV store first, falls back to auth_credentials
- ✅ All services and managers updated to use new resolution logic
- ✅ Backward compatibility maintained (existing deployments work unchanged)
- ✅ Cookie-based auth remains in auth_credentials (separation of concerns)
- ✅ All tests pass
- ✅ Application compiles and runs successfully

## Notes
- This is Phase 3 of the KV store refactor
- Phase 1 (infrastructure) and Phase 2 (replacement engine) are already complete
- Phase 4 will remove API key support from auth_credentials entirely
- Migration maintains backward compatibility until Phase 4 cleanup
