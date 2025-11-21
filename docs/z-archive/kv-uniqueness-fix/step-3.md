# Step 3: Add Explicit Upsert Method to API

**Skill:** @go-coder
**Files:**
- `internal/interfaces/kv_storage.go`
- `internal/storage/sqlite/kv_storage.go`
- `internal/services/kv/service.go`
- `internal/handlers/kv_handler.go`

---

## Iteration 1

### Agent 2 - Implementation

Added explicit `Upsert()` method throughout the key/value stack to provide clear visibility into whether an operation created a new key or updated an existing one.

**Changes made:**

1. **`internal/interfaces/kv_storage.go`:**
   - Added `Upsert(ctx, key, value, description) (bool, error)` method to interface
   - Returns `true` if new key created, `false` if existing key updated
   - Documentation clarifies explicit logging behavior

2. **`internal/storage/sqlite/kv_storage.go`:**
   - Implemented `Upsert()` method with two-phase operation:
     - Phase 1: Check if key exists using `SELECT EXISTS(...)`
     - Phase 2: Perform upsert using same SQL as `Set()`
   - Returns boolean indicating whether key was newly created
   - Uses mutex locking for thread safety
   - Normalizes keys using `normalizeKey()` helper

3. **`internal/services/kv/service.go`:**
   - Added `Upsert()` method to service layer
   - Validates key is not empty
   - Gets old value for event payload
   - Calls storage `Upsert()` and logs based on result:
     - "Created new key/value pair" if new
     - "Updated existing key/value pair" if existing
   - Publishes `EventKeyUpdated` with `is_new` flag
   - Returns boolean result to caller

4. **`internal/handlers/kv_handler.go`:**
   - Added `Upsert()` to `KVServiceInterface`
   - Updated `UpdateKVHandler` (PUT /api/kv/{key}) to use Upsert:
     - Returns HTTP 201 Created for new keys
     - Returns HTTP 200 OK for updated keys
     - Includes `"created": true/false` in JSON response
     - Logs specific messages for create vs update operations
   - Updated function documentation to reflect upsert behavior

**API Changes:**
- `PUT /api/kv/{key}` now explicitly supports upsert semantics
- Response includes `created` boolean field
- HTTP status code distinguishes create (201) from update (200)

**Example API Response (New Key):**
```json
{
  "status": "success",
  "message": "Key/value pair created successfully",
  "key": "new_api_key",
  "created": true
}
```

**Example API Response (Updated Key):**
```json
{
  "status": "success",
  "message": "Key/value pair updated successfully",
  "key": "existing_api_key",
  "created": false
}
```

**Commands run:**
```bash
cd C:\development\quaero && go build -o /tmp/quaero.exe ./cmd/quaero
```

### Agent 3 - Validation

**Skill:** @go-coder

**Compilation:**
✅ Compiles cleanly

**Tests:**
⚙️ No tests run (API tests to be added in Step 5)

**Code Quality:**
✅ Follows Go patterns
✅ Matches existing code style
✅ Proper error handling
✅ Clear method signatures with boolean return
✅ Consistent with existing storage/service/handler layering
✅ Thread-safe implementation (uses mutex)
✅ Event publishing for observability

**Quality Score:** 9/10

**Issues Found:**
None - Implementation is clean and follows established patterns

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 9/10

**Notes:**
- Explicit upsert semantics implemented across all layers
- Clear visibility into create vs update operations
- No breaking changes to existing API
- `POST /api/kv` still creates (fails if exists)
- `PUT /api/kv/{key}` now explicitly supports upsert
- Logging distinguishes between create and update
- HTTP status codes and response JSON indicate operation type

**Backward Compatibility:**
- `Set()` method unchanged - still provides upsert behavior
- `Upsert()` is new method - no existing code breaks
- Existing callers of `Set()` continue to work

**→ Continuing to Step 4**
