# Step 3: Add Deprecation Notices to Old Files

**Skill:** @go-coder
**Files:** `internal/jobs/processor/crawler_executor.go`, `internal/jobs/processor/crawler_executor_auth.go`

---

## Iteration 1

### Agent 2 - Implementation

Added detailed deprecation comments to both old processor files to indicate they have been merged and migrated to worker package.

**Changes made:**
- `internal/jobs/processor/crawler_executor.go` - Added deprecation notice
- `internal/jobs/processor/crawler_executor_auth.go` - Added deprecation notice

**Deprecation Notice Added to crawler_executor.go:**
```go
// DEPRECATED: This file has been merged with crawler_executor_auth.go and migrated to
// internal/jobs/worker/crawler_worker.go (ARCH-005).
// This file is kept temporarily for backward compatibility and will be removed in ARCH-008.
// New code should import from internal/jobs/worker and use CrawlerWorker instead.
//
// Migration Details:
// - Struct renamed: CrawlerExecutor → CrawlerWorker
// - Constructor renamed: NewCrawlerExecutor() → NewCrawlerWorker()
// - Auth logic merged: injectAuthCookies() now in crawler_worker.go
// - Package changed: processor → worker
```

**Deprecation Notice Added to crawler_executor_auth.go:**
```go
// DEPRECATED: This file has been merged into internal/jobs/worker/crawler_worker.go (ARCH-005).
// The injectAuthCookies() method is now a private method in CrawlerWorker.
// This file is kept temporarily for backward compatibility and will be removed in ARCH-008.
// New code should import from internal/jobs/worker and use CrawlerWorker instead.
//
// Migration Details:
// - Method moved: injectAuthCookies() now in crawler_worker.go as private method
// - Receiver updated: func (e *CrawlerExecutor) → func (w *CrawlerWorker)
// - All auth logic preserved: conditional authStorage check, three-phase injection, extensive logging
// - Package changed: processor → worker
```

**Commands run:**
```bash
# Verify files still compile (for backward compatibility)
go build internal/jobs/processor/crawler_executor.go internal/jobs/processor/crawler_executor_auth.go
```

### Agent 3 - Validation

**Skill:** @go-coder

**Compilation:**
✅ Compiles cleanly - Files remain functional

**Tests:**
⚙️ No tests applicable (documentation only)

**Code Quality:**
✅ Clear deprecation notices - explains what changed and where to find new code
✅ Migration timeline documented - indicates removal in ARCH-008
✅ Migration details provided - lists all transformations applied
✅ Guidance for developers - points to new location (internal/jobs/worker)
✅ Backward compatibility maintained - files remain functional until ARCH-008
✅ Consistent format - both files use same deprecation notice structure

**Quality Score:** 10/10

**Issues Found:**
None - deprecation notices added successfully

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
Deprecation notices added to both old processor files. The notices clearly explain the merge and migration to worker package, provide migration timeline (removal in ARCH-008), document all transformations, and guide developers to the new location. Files remain functional for backward compatibility.

**→ Continuing to Step 4**
