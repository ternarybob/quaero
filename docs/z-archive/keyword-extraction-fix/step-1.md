# Step 1: Fix document query to return all documents when source_type is empty

**Skill:** @code-architect
**Files:** `internal/jobs/manager/agent_manager.go`

---

## Iteration 1

### Agent 2 - Implementation

Modified `queryDocuments()` function to handle empty/unspecified source_type by querying ALL documents.

**Changes made:**
- `agent_manager.go:154-179`: Changed document querying logic
  - Check if `jobDef.SourceType` is empty
  - If empty, don't add source type filter (query all documents)
  - If specified, keep existing filter behavior
  - Maintains backward compatibility for jobs that specify source_type

**Original code (lines 157-159):**
```go
opts := interfaces.SearchOptions{
    SourceTypes: []string{jobDef.SourceType},
    Limit:       1000,
}
```

**Updated code:**
```go
opts := interfaces.SearchOptions{
    Limit: 1000,
}

// Only filter by source type if specified in job definition
if jobDef.SourceType != "" {
    opts.SourceTypes = []string{jobDef.SourceType}
}
```

**Commands run:**
```bash
go build -o /tmp/agent_manager_test ./internal/jobs/manager
```

**Result:** ✅ Compiles cleanly

### Agent 3 - Validation
**Skill:** @code-architect

**Compilation:**
✅ Compiles cleanly - no errors

**Tests:**
⚙️ No existing tests for AgentManager.queryDocuments() - will test in Step 4

**Code Quality:**
✅ Follows Go patterns - simple conditional check
✅ Matches existing code style - consistent with codebase
✅ Proper error handling - maintains existing error handling
✅ Backward compatible - jobs with source_type still work
✅ Solves root cause - allows querying all documents when source_type empty
✅ Clean implementation - minimal changes, clear intent

**Quality Score:** 9/10

**Issues Found:**
None - implementation is clean and solves the problem directly.

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 9/10

**Notes:**
- Simple, focused fix that addresses root cause
- Maintains backward compatibility
- Will be tested end-to-end in Step 4

**→ Continuing to Step 2**

