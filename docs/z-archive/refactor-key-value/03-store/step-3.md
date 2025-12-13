# Step 3: Update ResolveAPIKey Function

**Skill:** @go-coder
**Files:**
- `internal/common/config.go`
- `internal/interfaces/storage.go`
- `internal/interfaces/kv_storage.go`

---

## Iteration 1

### Agent 2 - Implementation

Updating `ResolveAPIKey()` function to query KV store first, with fallback to auth storage for backward compatibility.

**Implementation details:**
- Add `kvStorage` parameter to function signature
- Resolution order: KV store → auth storage (backward compat) → config fallback
- Graceful degradation if KV store query fails
- Maintain existing error messages

**Commands run:**
```bash
go build -o bin/quaero.exe ./cmd/quaero
```

**Changes:**
1. Updated `ResolveAPIKey()` signature in `internal/common/config.go:627`
   - Added `kvStorage interfaces.KeyValueStorage` as first parameter
   - Updated resolution order comment
   - Added KV store lookup as first priority
   - Maintained auth storage fallback for backward compatibility
   - Maintained config fallback as lowest priority
   - Updated error message to include all three sources

**Build result:** ❌ Expected failures - 6 call sites need updating:
- `internal/services/llm/gemini_service.go:107`
- `internal/services/places/service.go:37`
- `internal/services/agents/service.go:61`
- `internal/jobs/manager/agent_manager.go:62`
- `internal/jobs/manager/places_search_manager.go:73`
- `internal/handlers/job_definition_handler.go:586`

These will be fixed in Steps 4-8.

---

## Iteration 2 - Validation

### Agent 3 - Validator

**Validation steps:**
1. ✅ Function signature updated correctly
2. ✅ Resolution order updated: KV store → auth storage → config
3. ✅ Backward compatibility maintained (nil kvStorage handled)
4. ✅ Graceful degradation on KV store errors
5. ✅ Error message updated to reflect all sources
6. ✅ All call sites identified for next steps

**Status:** APPROVED - Step 3 complete (call sites will be fixed in Steps 4-8)
