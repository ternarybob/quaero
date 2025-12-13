# Step 1: Create settings_system_test.go with KV Store tests

**Skill:** @test-writer
**Files:** `test/api/settings_system_test.go` (NEW)

---

## Iteration 1

### Agent 2 - Implementation

Created comprehensive KV Store tests in `test/api/settings_system_test.go` following the `health_check_test.go` pattern.

**Implementation details:**
- Package: `api`
- Imports: `testing`, `net/http`, `fmt`, `github.com/stretchr/testify/assert`, `github.com/stretchr/testify/require`, `github.com/ternarybob/quaero/test/common`
- Test setup pattern: `SetupTestEnvironment()` with Badger config `../config/test-quaero-badger.toml`
- Helper functions: `createKVPair()`, `deleteKVPair()`, `createConnector()`, `deleteConnector()`

**Test functions implemented:**
1. **TestKVStore_CRUD** - Complete CRUD lifecycle:
   - POST /api/kv with TEST_KEY → 201 Created
   - GET /api/kv → Verify masking ("test...123")
   - GET /api/kv/test_key (lowercase) → Full value
   - PUT /api/kv/Test_Key (mixed case) → 200 OK, created=false
   - DELETE /api/kv/test_key → 200 OK
   - GET /api/kv/TEST_KEY → 404 Not Found

2. **TestKVStore_CaseInsensitive** - Case-insensitive key handling:
   - POST with GOOGLE_API_KEY
   - GET with lowercase, mixed case → Same value
   - PUT with mixed case → Updates, not creates
   - Verify only 1 key exists (not 3 duplicates)

3. **TestKVStore_Upsert** - PUT upsert behavior:
   - PUT NEW_KEY → 201 Created, created=true
   - PUT new_key → 200 OK, created=false
   - Verify updated value

4. **TestKVStore_DuplicateValidation** - Duplicate detection:
   - POST DUPLICATE_KEY → 201 Created
   - POST DUPLICATE_KEY → 409 Conflict
   - POST duplicate_key (lowercase) → 409 Conflict

5. **TestKVStore_ValueMasking** - Value masking:
   - POST SHORT="abc", LONG="sk-1234567890abcdef"
   - GET /api/kv → Verify SHORT="••••••••", LONG="sk-1...cdef"
   - GET /api/kv/SHORT → Full value "abc"
   - GET /api/kv/LONG → Full value "sk-1234567890abcdef"

6. **TestKVStore_ValidationErrors** - Validation errors:
   - POST with empty key → 400 Bad Request
   - POST with empty value → 400 Bad Request
   - POST with invalid JSON → 400 Bad Request
   - GET with empty key → 400 or 404
   - PUT description-only on missing key → 400 or 404

**Changes made:**
- `test/api/settings_system_test.go`: Created with 6 KV test functions + 4 helper functions

**Commands run:**
```bash
cd test/api && go test -c -o /tmp/settings_system_test.exe
```
Result: ✅ Compilation successful

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 9/10

**Notes:**
- All 6 KV Store test functions implemented with comprehensive coverage
- Tests follow established patterns from health_check_test.go
- Helper functions created for common operations
- Proper use of require/assert for error handling
- Tests verify exact response structures and status codes
- Error cases properly tested

**→ Continuing to Step 2**
