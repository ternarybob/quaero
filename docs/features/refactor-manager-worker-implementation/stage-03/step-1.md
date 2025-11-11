# Step 1: Create crawler_manager.go in internal/jobs/manager/

**Skill:** @code-architect
**Files:** internal/jobs/manager/crawler_manager.go (NEW), internal/jobs/executor/crawler_step_executor.go (READ)

---

## Iteration 1

### Agent 2 - Implementation

Created new `crawler_manager.go` file by copying from `crawler_step_executor.go` and applying the following transformations:

**Changes made:**
- `internal/jobs/manager/crawler_manager.go`: Created new file with following changes:
  - Package declaration: `executor` → `manager`
  - Struct name: `CrawlerStepExecutor` → `CrawlerManager`
  - Constructor name: `NewCrawlerStepExecutor()` → `NewCrawlerManager()`
  - Constructor return type: `*CrawlerStepExecutor` → `*CrawlerManager`
  - Method receiver variable: `e` → `m` (throughout all methods)
  - Updated all `e.` references to `m.` in method bodies
  - Kept all method signatures unchanged (CreateParentJob, GetManagerType)
  - Kept all imports unchanged
  - Kept all helper methods (buildCrawlConfig, buildSeedURLs) with updated receivers
  - Total lines: 255 (same as original)

**Commands run:**
```bash
go build -o /tmp/test_crawler_manager internal/jobs/manager/crawler_manager.go internal/jobs/manager/interfaces.go
```

### Agent 3 - Validation

**Skill:** @code-architect

**Compilation:**
✅ Compiles cleanly - No errors or warnings

**Tests:**
⚙️ No tests applicable - File migration, tests run in Step 8

**Code Quality:**
✅ Follows Go patterns - Clean struct/constructor/method pattern
✅ Matches existing code style - Consistent with manager package conventions
✅ Proper error handling - All error paths properly handled with context
✅ Interface compliance - Implements JobManager interface correctly

**Quality Score:** 10/10

**Issues Found:**
None

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
Mechanical transformation completed successfully. All naming conventions updated from "StepExecutor" to "Manager" terminology. File compiles independently and implements JobManager interface correctly. Ready for integration in app.go.

**→ Continuing to Step 2**
