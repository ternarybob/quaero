# Plan: Key/Value Uniqueness and Case-Insensitivity Fix

## Problem Statement
The current key/value implementation has two main issues:
1. Keys are case-sensitive (e.g., "GOOGLE_API_KEY" != "google_api_key")
2. Duplicate keys can exist in different TOML files or within the same file with different cases

This causes confusion and potential conflicts when loading variables from multiple TOML files at startup.

## Current Implementation Analysis

**Database Schema:**
- Table: `key_value_store`
- Primary key: `key TEXT PRIMARY KEY` (case-sensitive in SQLite)
- Constraint: Uses `ON CONFLICT(key) DO UPDATE SET...` for upsert behavior

**Loading Process:**
- Location: `internal/storage/sqlite/load_keys.go`
- Loads from `./variables/` directory (configurable)
- Uses `Set()` method which has built-in upsert via `ON CONFLICT`
- No duplicate detection or warning for case-variant keys

**API Endpoints:**
- `POST /api/kv` - Create (uses Set with upsert)
- `PUT /api/kv/{key}` - Update (uses Set with upsert)
- `GET /api/kv/{key}` - Retrieve
- `DELETE /api/kv/{key}` - Delete
- `GET /api/kv` - List all

## Steps

### 1. Add Case-Insensitive Key Normalization
**Skill:** @code-architect
**Files:**
- `internal/storage/sqlite/kv_storage.go`
- `internal/interfaces/kv_storage.go`
- `internal/storage/sqlite/schema.go`

**Changes:**
- Add `normalizeKey()` helper function to convert keys to lowercase
- Update schema to store normalized keys (lowercase) while preserving original casing in metadata
- Add migration logic to normalize existing keys in database
- Update all storage methods (Get, GetPair, Set, Delete) to use normalized keys

**User decision:** no

### 2. Add Duplicate Key Detection to File Loading
**Skill:** @go-coder
**Files:**
- `internal/storage/sqlite/load_keys.go`

**Changes:**
- Track loaded keys during file loading process
- Detect case-insensitive duplicates across files
- Detect case-insensitive duplicates within the same file
- Log warnings for duplicate keys (showing file names and which key is being used)
- Use upsert behavior (last one wins) but warn user

**User decision:** no

### 3. Add Explicit Upsert Method to API
**Skill:** @go-coder
**Files:**
- `internal/interfaces/kv_storage.go`
- `internal/storage/sqlite/kv_storage.go`
- `internal/services/kv/service.go`
- `internal/handlers/kv_handler.go`

**Changes:**
- Add `Upsert()` method to `KeyValueStorage` interface (separate from `Set`)
- Implement `Upsert()` in SQLite storage with explicit logging of insert vs update
- Add `Upsert()` to KV service with event publishing
- Add handler method for `PUT /api/kv/{key}` to explicitly support upsert
- Document the difference: `Set()` for internal use, `Upsert()` for API

**User decision:** no

### 4. Update Startup Loading to Use Upsert with Warnings
**Skill:** @go-coder
**Files:**
- `internal/storage/sqlite/load_keys.go`
- `internal/app/app.go`

**Changes:**
- Modify `LoadKeysFromFiles()` to use new `Upsert()` method
- Add pre-load check: query existing keys and compare with file keys
- Log warnings when file key will update existing database key
- Log info when file key is creating new database key
- Ensure startup doesn't fail on duplicate keys (graceful degradation)

**User decision:** no

### 5. Add Tests for Case-Insensitive Behavior
**Skill:** @test-writer
**Files:**
- `test/api/kv_case_insensitive_test.go` (new)
- `internal/storage/sqlite/kv_storage_test.go`

**Changes:**
- Add unit tests for normalized key storage
- Add API integration tests for case-insensitive key operations
- Test duplicate detection during file loading
- Test upsert warnings at startup
- Test API upsert endpoint behavior

**User decision:** no

### 6. Update Documentation
**Skill:** @none
**Files:**
- `docs/features/kv-uniqueness-fix/implementation-notes.md` (new)

**Changes:**
- Document the case-insensitive key design
- Document the upsert behavior
- Provide examples of TOML file loading with warnings
- Document migration strategy for existing deployments

**User decision:** no

## Success Criteria

- ✅ Keys are case-insensitive (e.g., "GOOGLE_API_KEY" and "google_api_key" resolve to same entry)
- ✅ Duplicate keys are prevented (unique constraint on normalized key)
- ✅ API supports explicit upsert operation via PUT endpoint
- ✅ Service startup uses upsert and warns about duplicates/updates
- ✅ All code compiles cleanly
- ✅ Tests pass (or documented if issues found)
- ✅ No breaking changes to existing API contracts
- ✅ Backward compatible with existing TOML files

## Migration Strategy

For existing deployments:
1. On first startup with new code, migration runs automatically
2. Existing keys are normalized (converted to lowercase)
3. If case-variant duplicates exist (e.g., "API_KEY" and "api_key"), last one wins
4. Warning logged for any merged keys
5. TOML files don't need changes - normalization happens at runtime

## Technical Notes

- SQLite's `TEXT` type is case-sensitive by default for PRIMARY KEY
- We'll normalize keys in application layer before storage
- Original key casing can be preserved in a separate column if needed for display
- The `ON CONFLICT` clause already provides upsert semantics
- We're making the upsert behavior more explicit and adding observability
