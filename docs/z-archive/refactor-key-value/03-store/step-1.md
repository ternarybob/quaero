# Step 1: Add Migration Method to Manager

**Skill:** @go-coder
**Files:**
- `internal/storage/sqlite/manager.go`
- `internal/interfaces/storage.go`
- `internal/storage/sqlite/kv_storage.go`
- `internal/storage/sqlite/auth_storage.go`

---

## Iteration 1

### Agent 2 - Implementation

Adding `MigrateAPIKeysToKVStore()` method to Manager struct to enable one-time migration of API keys from auth_credentials to key_value_store.

**Implementation details:**
- Query all credentials with `auth_type='api_key'` from auth_credentials table
- For each API key, check if already exists in KV store
- Insert into KV store if not exists (idempotent)
- Track counts: migrated, skipped (already exists), failed
- Log progress and final summary

**Changes made:**
- `internal/storage/sqlite/manager.go`: Added `MigrateAPIKeysToKVStore()` method after existing methods

**Commands run:**
```bash
go build -o bin/quaero.exe ./cmd/quaero
```

**Changes:**
1. Added `MigrateAPIKeysToKVStore()` method to `internal/storage/sqlite/manager.go:97-158`
   - Queries all credentials with `auth_type='api_key'`
   - For each API key, checks if already exists in KV store
   - Inserts into KV store if not exists (idempotent)
   - Tracks counts: migrated, skipped, failed
   - Logs progress and final summary
2. Added required imports (context, fmt) to manager.go

**Build result:** ✅ Success

---

## Iteration 2 - Validation

### Agent 3 - Validator

**Validation steps:**
1. ✅ Code compiles successfully
2. ✅ Method signature matches plan specifications
3. ✅ Idempotent implementation (checks existence before insert)
4. ✅ Proper error handling with graceful degradation
5. ✅ Comprehensive logging (debug, warn, info levels)
6. ✅ Migration statistics tracking

**Status:** APPROVED - Step 1 complete
