# Step 2: Update App Registration and Imports

**Skill:** @go-coder
**Files:** `internal/app/app.go`

---

## Iteration 1

### Agent 2 - Implementation

Updated app.go to import and use the new worker package for CrawlerWorker registration.

**Changes made:**
- `internal/app/app.go` - Added worker import, updated CrawlerWorker registration

**Specific Changes:**
1. Added worker import: `"github.com/ternarybob/quaero/internal/jobs/worker"`
2. Renamed variable: `crawlerExecutor` → `crawlerWorker`
3. Updated constructor call: `processor.NewCrawlerExecutor()` → `worker.NewCrawlerWorker()`
4. Updated comment: "Register crawler_url executor" → "Register crawler_url worker"
5. All 8 parameters preserved in same order (no changes needed)
6. Log message already correct: "Crawler URL worker registered for job type: crawler_url"

**Other Workers Unchanged:**
- ParentJobExecutor still uses `processor` package (not migrated in this phase)
- AgentExecutor still uses `processor` package (migrating in ARCH-006)
- DatabaseMaintenanceExecutor still uses `executor` package (migrating in ARCH-007)

**Commands run:**
```bash
# Build full application to verify integration
cd C:\development\quaero
.\scripts\build.ps1
```

### Agent 3 - Validation

**Skill:** @go-coder

**Compilation:**
✅ Compiles cleanly - Build successful

**Tests:**
⚙️ No tests applicable (integration test in final step)

**Code Quality:**
✅ Follows Go patterns - import organization, constructor pattern
✅ Matches existing code style - consistent with other worker registrations
✅ Proper error handling - no error handling needed for registration
✅ All parameters correct - 8 dependencies passed in correct order
✅ Import added correctly - placed after processor import
✅ Variable renamed correctly - crawlerExecutor → crawlerWorker
✅ Constructor call updated - processor.NewCrawlerExecutor → worker.NewCrawlerWorker
✅ Comments updated - reflects worker terminology

**Quality Score:** 10/10

**Issues Found:**
None - integration completed successfully

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
App.go successfully updated to import and use the new worker package. The CrawlerWorker is now registered with the JobProcessor using the worker.NewCrawlerWorker() constructor. Application builds successfully. Other workers remain in their current packages (processor/ and executor/) and will be migrated in future phases.

**→ Continuing to Step 3**
