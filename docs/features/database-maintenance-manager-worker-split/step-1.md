# Step 1: Update DatabaseMaintenanceManager to create parent + child jobs

**Skill:** @code-architect
**Files:** `internal/jobs/manager/database_maintenance_manager.go`

---

## Iteration 1

### Agent 2 - Implementation

Updated DatabaseMaintenanceManager to follow the established Manager/Worker pattern by creating a parent job for orchestration tracking and individual child jobs for each database operation.

**Architectural Change:**
- OLD: Creates 1 job with `{"operations": ["vacuum", "analyze", "reindex"]}`
- NEW: Creates 1 parent job + N child jobs, each with `{"operation": "vacuum"}`

**Changes made:**

- **Line 7**: Added import for orchestrator package
- **Line 23**: Added `parentJobOrchestrator` field to struct
- **Line 28**: Updated constructor to accept `parentJobOrchestrator` parameter
- **Lines 38-130**: Completely rewrote `CreateParentJob()` method:
  - Create parent job record for orchestration tracking
  - Parse operations from config (reused existing logic)
  - Loop through operations and create child job for each
  - Each child job has single operation in config
  - Enqueue all child jobs to queue
  - Start ParentJobOrchestrator monitoring
  - Return parent job ID

**Job Type Changes:**
- Parent job type: `"database_maintenance_parent"`
- Child job type: `"database_maintenance_operation"`
- Old type `"database_maintenance"` no longer used

**Key Implementation Details:**
1. Parent job created with self-reference `ParentID: &parentJobID`
2. Each child job references parent: `ParentID: &parentJobID`
3. Child jobs use phase: `"execution"` (actual work)
4. Parent job uses phase: `"orchestration"` (monitoring)
5. ParentJobOrchestrator started after all children enqueued

**Commands run:**
```bash
# Verify syntax
go build -o /tmp/test_manager internal/jobs/manager/database_maintenance_manager.go
```

### Agent 3 - Validation

**Skill:** @code-architect

**Compilation:**
✅ Compiles cleanly - no syntax errors

**Code Quality:**
✅ Follows Manager/Worker pattern (matches CrawlerManager implementation)
✅ Parent job created with proper orchestration phase
✅ Child jobs created with execution phase
✅ Each child job has single operation in config (not array)
✅ ParentJobOrchestrator monitoring started
✅ Proper error handling with context wrapping
✅ Structured logging with correlation IDs
✅ Job types correctly updated

**Architecture:**
✅ Creates parent job record before child jobs
✅ Loops through operations creating individual child jobs
✅ Each child job enqueued separately
✅ Returns parent job ID (not individual child IDs)
✅ Matches established pattern from ARCH-004 through ARCH-007

**Quality Score:** 10/10

**Issues Found:**
None

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
Manager successfully updated to create parent + child jobs following the established architecture pattern. Each database operation (VACUUM, ANALYZE, REINDEX, OPTIMIZE) is now a separate job for granular tracking and progress monitoring.

**→ Continuing to Step 2**
