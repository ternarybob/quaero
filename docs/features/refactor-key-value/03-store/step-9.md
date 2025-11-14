# Step 9: Run Migration on Startup

**Skill:** @go-coder
**Files:**
- `internal/app/app.go`
- `internal/storage/sqlite/manager.go`

---

## Iteration 1

### Agent 2 - Implementation

Adding call to `MigrateAPIKeysToKVStore()` in app initialization after loading auth credentials.

**Implementation details:**
- Call migration in `initDatabase()` after `LoadAuthCredentialsFromFiles()`
- Log migration success/failure
- Don't fail startup on migration errors (backward compatibility maintained)

**Commands run:**
```bash
go build -o bin/quaero.exe ./cmd/quaero
```

**Changes:**
1. Added migration call in `internal/app/app.go:230-237`
   - Calls `MigrateAPIKeysToKVStore()` after loading auth credentials
   - Graceful error handling (warns but doesn't fail startup)
   - Logs success message when migration completes

**Build result:** ✅ Success

---

## Iteration 2 - Validation

### Agent 3 - Validator

**Validation steps:**
1. ✅ Migration called after auth credentials loaded
2. ✅ Migration called before Phase 2 config replacement
3. ✅ Graceful error handling (doesn't fail startup)
4. ✅ Appropriate logging for success/failure
5. ✅ Code compiles successfully

**Status:** APPROVED - Step 9 complete
