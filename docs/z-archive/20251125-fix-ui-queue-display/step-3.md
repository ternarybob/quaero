# Step 3: Fix status filter handling

**Skill:** @go-coder
**Files:** `internal/storage/badger/job_storage.go`

---

## Iteration 1

### Agent 2 - Implementation

**FIX IMPLEMENTED:**

Modified `ListJobs` method in `internal/storage/badger/job_storage.go` to properly parse comma-separated status values.

**Changes made:**
- `internal/storage/badger/job_storage.go:3-12`: Added `strings` import
- `internal/storage/badger/job_storage.go:171-185`: Replaced single status comparison with comma-separated status parsing

**Before (lines 171-176):**
```go
// Apply status filter
if opts != nil && opts.Status != "" {
    if string(jobState.Status) != opts.Status {
        continue
    }
}
```

**After (lines 171-185):**
```go
// Apply status filter (supports comma-separated values)
if opts != nil && opts.Status != "" {
    // Parse comma-separated status values
    statusList := strings.Split(opts.Status, ",")
    matchFound := false
    for _, status := range statusList {
        if string(jobState.Status) == strings.TrimSpace(status) {
            matchFound = true
            break
        }
    }
    if !matchFound {
        continue
    }
}
```

**How it works:**
1. Split the status string by comma (e.g., "pending,running,failed")
2. Trim whitespace from each status value
3. Check if the job's status matches ANY of the provided status values
4. Only include the job if a match is found

**Commands run:**
```bash
# Added strings import to package
# Compiled package successfully
cd internal/storage/badger && go build
```

### Agent 3 - Validation
**Skill:** @go-coder

**Compilation:**
✅ Compiles cleanly

**Tests:**
⚙️ Not run yet (will run in Step 7)

**Code Quality:**
✅ Follows Go patterns
✅ Properly handles comma-separated values with trimming
✅ Maintains backward compatibility (single status still works)
✅ Clear comments explaining the change

**Quality Score:** 10/10

**Issues Found:**
None

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
The fix properly implements comma-separated status filtering as documented in the handler. The UI can now query for `status=pending,running,completed,failed,cancelled` and jobs with any of those statuses will be returned.

**→ Continuing to Step 4**
