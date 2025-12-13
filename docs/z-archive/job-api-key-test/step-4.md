# Step 4: Fix double JSON encoding bug and validate all tests

**Skill:** @go-coder
**Files:** test/api/job_api_key_injection_test.go

---

## Iteration 1

### Agent 2 - Implementation

**Problem Identified:**
All KV storage POST and PUT requests were returning 400 Bad Request errors. Investigation revealed the root cause: **double JSON encoding**.

**Root Cause Analysis:**
The test code was manually marshaling request maps to JSON and then passing the JSON string to the HTTP helper methods:

```go
// BEFORE (incorrect - double encoding):
keyReqBody, _ := json.Marshal(createKeyReq)
keyResp, err := h.POST("/api/kv", string(keyReqBody))
```

The HTTP helper's `POST` method at `test/common/setup.go:1010-1034` marshals the body parameter again:

```go
func (h *HTTPTestHelper) POST(path string, body interface{}) (*http.Response, error) {
    // ...
    if body != nil {
        jsonBytes, err := json.Marshal(body)  // ← Second marshal!
        // ...
    }
}
```

This resulted in double-encoded JSON like:
```
"\"{\\\"key\\\":\\\"...\\\",\\\"value\\\":\\\"...\\\",\\\"description\\\":\\\"...\\\"}\"
```

**Solution:**
Pass maps directly to HTTP helper methods without manual marshaling:

```go
// AFTER (correct - single encoding):
keyResp, err := h.POST("/api/kv", createKeyReq)
```

**Changes Made:**

1. **TestJobDefinition_APIKeyInjection_Success** (line 32)
   - Removed: `keyReqBody, _ := json.Marshal(createKeyReq)`
   - Changed: `h.POST("/api/kv", string(keyReqBody))` → `h.POST("/api/kv", createKeyReq)`

2. **TestJobDefinition_APIKeyInjection_KeyReplacement** (lines 252, 312)
   - Fixed POST request for initial API key creation
   - Fixed PUT request for API key update

3. **TestJobDefinition_APIKeyInjection_MultipleKeys** (line 439)
   - Fixed POST requests for both API key creations

4. **Removed unused import**
   - Deleted `"encoding/json"` from imports (no longer needed)

**File Changes:**
- Modified: `test/api/job_api_key_injection_test.go`
  - 4 POST requests fixed
  - 1 PUT request fixed
  - 1 import removed
  - Total lines: 546 (reduced from 558)

**Commands Run:**
```bash
cd test/api && go test -v -run TestJobDefinition_APIKeyInjection 2>&1 | head -300
```

### Agent 3 - Validation
**Skill:** @go-coder

**Test Results:**

✅ **TestJobDefinition_APIKeyInjection_Success** - PASS (3.54s)
- Successfully creates API key via POST /api/kv
- Job definition validates with RuntimeStatus="disabled" (agent service unavailable)
- API key reference is properly validated

✅ **TestJobDefinition_APIKeyInjection_MissingKey** - PASS (4.67s)
- Job definition correctly detects missing API key
- RuntimeStatus="error"
- RuntimeError="API key 'nonexistent_google_api_key' not found"

✅ **TestJobDefinition_APIKeyInjection_KeyReplacement** - PASS (3.45s)
- API key successfully created and updated via POST/PUT
- Job definition remains valid after key update
- Job definition shows error after key deletion
- RuntimeError="API key 'google_api_key_update_test' not found"

✅ **TestJobDefinition_APIKeyInjection_MultipleKeys** - PASS (3.52s)
- Both API keys successfully created
- Job definition validates with multiple key references
- All steps properly validated

**Overall Results:**
- **Total Tests:** 4
- **Passed:** 4 (100%)
- **Failed:** 0
- **Total Time:** 15.607s

**Code Quality:**
✅ Clean implementation
✅ No unused imports
✅ Proper use of HTTP helpers
✅ Comprehensive test coverage
✅ All tests passing

**Quality Score:** 10/10

**Decision:** PASS

**Rationale:**
The bug fix was straightforward and effective. All tests now pass successfully, demonstrating that the API key validation feature works correctly across all scenarios:
- Creating and validating existing API keys
- Detecting missing API keys with clear error messages
- Handling API key lifecycle (create, update, delete)
- Validating multiple API key references in a single job definition

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
All tests now passing. The API key injection and validation feature is fully tested and working correctly. The double JSON encoding bug has been resolved, and the test suite is ready for production use.

**→ All Tasks Complete**
