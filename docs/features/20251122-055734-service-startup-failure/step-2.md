# Step 2: Fix BadgerDB Type Assertion

**Skill:** @go-coder
**Files:** `internal/app/app.go`, `internal/queue/badger_manager.go`

---

## Iteration 1

### Agent 2 - Implementation

Fixing the type assertion to properly extract the underlying `*badger.DB` from `*badgerhold.Store`.

**Changes made:**

1. **`internal/app/app.go` lines 347-366**: Fixed BadgerDB type assertion
   - Changed from unsafe type assertion to `*badger.DB` directly
   - Now first casts to `*badgerhold.Store`, then extracts underlying `*badger.DB` using `.Badger()` method
   - Added `badgerhold/v4` import

2. **`internal/queue/badger_manager.go` line 10**: Updated badger import
   - Changed from `github.com/dgraph-io/badger/v3` to `github.com/dgraph-io/badger/v4`
   - Required to match the version used by badgerhold

3. **`internal/app/app.go`**: Removed unused badger import
   - Removed `github.com/dgraph-io/badger/v4` import since we only access badger through badgerhold

**Implementation:**

**Modified code in `app.go` (lines 347-357):**
```go
// 5.6. Initialize queue manager (Badger-backed)
// Obtain underlying Badger DB from storage manager
// StorageManager.DB() returns *badgerhold.Store, we need to extract the underlying *badger.DB
badgerStore, ok := a.StorageManager.DB().(*badgerhold.Store)
if !ok {
	return fmt.Errorf("storage manager is not backed by BadgerDB (got %T)", a.StorageManager.DB())
}

// Extract underlying *badger.DB from BadgerHold wrapper
badgerDB := badgerStore.Badger()
```

**Root cause of version mismatch:**
- Queue manager was using badger v3
- BadgerHold uses badger v4
- Type assertion from v4 DB to v3 DB failed

**Build verification:**
```bash
go build -o /tmp/quaero.exe ./cmd/quaero
# SUCCESS - No errors
```

**Runtime verification:**
```bash
./scripts/build.ps1 -run
# Application started successfully
# Server ready at http://localhost:8085
# Log file: bin/logs/quaero.2025-11-22T15-59-44.log
```

### Agent 3 - Validation
