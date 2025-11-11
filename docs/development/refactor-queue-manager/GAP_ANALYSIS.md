# Queue Refactor Gap Analysis

**Status:** Active Analysis
**Last Updated:** 2025-11-03
**Version:** 1.0

---

## 1. Executive Summary

The queue refactor is **approximately 65% complete** and currently in a transition state. The core infrastructure (goqite, WorkerPool, JobManager) is fully implemented and functional, but critical gaps block production use.

**Current State:**
- ✅ Core infrastructure complete (Queue Manager, Worker Pool, Message types)
- ✅ Architecture documented (`docs/architecture/MANAGER_WORKER_ARCHITECTURE.md` and `docs/architecture/README.md`)
- ⚠️ Database schema mismatch (code vs schema.go)
- ⚠️ CrawlerExecutor is placeholder only
- ❌ Queue management UI missing
- ❌ WebSocket real-time updates disabled

**Critical Blocker:** The most severe issue is the database schema mismatch. Code queries `jobs` table, but schema.go defines only `crawl_jobs` table. This will cause runtime failures.

**Estimated Completion:** 31-54 hours of focused work remaining across 3 phases.

---

## 2. Critical Gaps (P0 - Blocks Functionality)

### Gap 2.1: Database Schema Mismatch

**Severity:** P0 - Critical
**Impact:** Runtime failures when creating/querying jobs
**Status:** 🔴 Blocks all queue operations

**Evidence:**
- `internal/jobs/manager.go` queries `jobs` table (lines 65, 96, 126, 163, 209, 257, 278, 287, 301)
- `internal/storage/sqlite/schema.go` defines only `crawl_jobs` table (line 114)
- Migration file `docs/migrations/008_redesign_job_queue.sql` defines `jobs` table but was never applied
- No database migration exists to create `jobs` table in the schema

**Root Cause:**
The refactor plan proposed a new simplified `jobs` table, and a migration file was created. However:
1. The migration was never integrated into `schema.go`'s `schemaSQL` constant
2. The migration was never added to `runMigrations()` method
3. Schema continues to define the old `crawl_jobs` table
4. Code was updated to use new `jobs` table before schema was migrated

**Recommendation:**
**Option A: Migrate Schema to `jobs` Table** (Breaking Change)
- Pros: Cleaner separation, matches refactor plan, future-proof
- Cons: Requires data migration, breaking change, 8-12 hours work
- Steps:
  1. Integrate migration 008 into `schema.go`
  2. Add migration logic to `runMigrations()` to create `jobs` table
  3. Migrate existing `crawl_jobs` data to `jobs` table
  4. Update foreign keys (job_logs, job_seen_urls) to reference `jobs` table
  5. Test all CRUD operations

**Option B: Update Code to Use `crawl_jobs` Table** (Pragmatic)
- Pros: Preserves existing data, simpler, faster (4-6 hours)
- Cons: Violates separation of concerns (domain-specific fields in job table), deviates from refactor plan
- Steps:
  1. Update `internal/jobs/manager.go` to query `crawl_jobs` table
  2. Map simplified Job struct fields to crawl_jobs columns (see mapping below)
  3. Handle nullable/extra fields appropriately
  4. Test all CRUD operations

**Field Mapping (Job struct → crawl_jobs columns):**
| Job Field | crawl_jobs Column | Transformation |
|-----------|-------------------|----------------|
| `ID` | `id` | Direct mapping |
| `ParentID` | `parent_id` | Direct mapping (nullable) |
| `Type` | `job_type` | Direct mapping |
| `Phase` | `metadata` | Store as JSON: `{"phase": "core"}` |
| `Status` | `status` | Direct mapping |
| `CreatedAt` | `created_at` | Convert `time.Time` → Unix timestamp (INTEGER) |
| `StartedAt` | `started_at` | Convert `*time.Time` → Unix timestamp (nullable) |
| `CompletedAt` | `completed_at` | Convert `*time.Time` → Unix timestamp (nullable) |
| `Payload` | `config_json` | Direct mapping (store as JSON string) |
| `Result` | `metadata` | Store as JSON: `{"result": "..."}` (no direct column) |
| `Error` | `error` | Direct mapping (nullable) |
| `ProgressCurrent` | `progress_json` | Store/parse as JSON: `{"current": 50, "total": 100}` |
| `ProgressTotal` | `progress_json` | Store/parse as JSON: `{"current": 50, "total": 100}` |

**Unused crawl_jobs columns** (set to defaults or NULL):
- `name`, `description` → Empty strings or NULL
- `source_type`, `entity_type` → Set to generic values ("queue", "job") or empty
- `source_config_snapshot`, `auth_snapshot`, `seed_urls` → NULL
- `refresh_source` → 0 (default)
- `last_heartbeat` → Current timestamp or NULL
- `result_count`, `failed_count` → 0 (defaults)

**Recommended:** Option B for immediate functionality, plan Option A for future cleanup.

**Effort:** 4-6 hours (Option B), 8-12 hours (Option A)
**Dependencies:** None
**Priority:** Fix immediately - system is non-functional without this

---

### Gap 2.2: CrawlerExecutor Not Implemented

**Severity:** P0 - Critical
**Impact:** Jobs cannot execute - worker pool processes messages but fails
**Status:** 🔴 Placeholder returns error

**Evidence:**
- `internal/worker/crawler_executor.go` line 48: `return fmt.Errorf("crawler executor not yet fully implemented")`
- Worker pool starts, receives messages, routes to CrawlerExecutor, but execution fails
- No integration with `crawler.Service` for actual URL fetching

**Note:** The original refactor plan (`docs/refactor-queue-manager/01-refactor.md`, Example Executor section) shows the intended crawler execution behavior and integration patterns.

**Root Cause:**
During refactor, CrawlerExecutor skeleton was created to satisfy Executor interface, but business logic integration was deferred and never completed.

**Recommendation:**
Implement `Execute()` method with full crawler integration:

```go
func (e *CrawlerExecutor) Execute(ctx context.Context, jobID string, payload []byte) error {
    // 1. Parse payload
    var params CrawlerPayload
    if err := json.Unmarshal(payload, &params); err != nil {
        return fmt.Errorf("invalid payload: %w", err)
    }

    // 2. Initialize crawler for this job
    // 3. Fetch URL using crawler.Service
    // 4. Extract links from response
    // 5. Create child jobs for each discovered link via JobManager.CreateChildJob()
    // 6. Update progress via JobManager.UpdateJobProgress()
    // 7. Log events via JobManager.AddJobLog()
    // 8. Return success/error
}
```

**Integration Points:**
- `crawler.Service` - URL fetching, content extraction
- `JobManager.CreateChildJob()` - Create jobs for discovered URLs
- `JobManager.UpdateJobProgress()` - Track crawl progress
- `JobManager.AddJobLog()` - Structured logging
- `job_seen_urls` table - URL deduplication

**Effort:** 8-16 hours
**Dependencies:** Gap 2.1 must be resolved first
**Priority:** High - core functionality

---

## 3. High-Priority Gaps (P1 - Degrades UX)

### Gap 3.1: JobExecutor System Disabled

**Severity:** P1 - High
**Impact:** Multi-step workflows (JobDefinitions) cannot execute
**Status:** ⚠️ Commented out in app.go

**Evidence:**
- `internal/app/app.go` lines 374-416: JobExecutor initialization commented out with TODO
- Comment states: "OLD JOB EXECUTOR SYSTEM DISABLED - Uses deleted internal/services/jobs package"
- JobDefinition-based workflows are non-functional

**Root Cause:**
JobExecutor depends on the deleted `internal/services/jobs` package. During refactor, this system was disabled rather than updated to work with new queue architecture.

**Recommendation:**
**Option A: Re-enable with Queue Integration** (Recommended)
- Integrate JobExecutor with new queue-based job system
- Update JobRegistry to use new JobManager
- Ensure JobExecutor and queue-based jobs coexist properly
- Pros: Supports multi-step workflows, valuable feature
- Cons: 8-12 hours work, added complexity

**Option B: Remove Entirely**
- Delete JobExecutor code, remove from architecture
- Use only queue-based jobs for all execution
- Pros: Simpler architecture, one system
- Cons: Loses workflow orchestration capability

**Recommended:** Option A - workflows are valuable feature.

**Effort:** 8-12 hours
**Dependencies:** Gap 2.1 and 2.2 must be resolved first
**Priority:** High for users depending on job definitions

---

### Gap 3.2: Queue Management UI Missing

**Severity:** P1 - High
**Impact:** No visibility into queue state, job progress, or logs
**Status:** ❌ Not implemented

**Evidence:**
- `pages/jobs.html` focuses on authentication/sources/job definitions
- No queue monitoring page found
- No real-time job status visualization
- No job logs viewer

**Root Cause:**
Queue UI was planned in original refactor (`01-refactor.md` lines 1080-1403) but never implemented.

**Recommendation:**
Create comprehensive queue management UI:

**Page Structure** (`pages/queue.html`):
1. **Parent Jobs Table**
   - Columns: ID, Type, Status, Progress (current/total), Created, Started, Completed, Actions
   - Filter by status: All, Pending, Running, Completed, Failed
   - Sort by creation time (desc)
   - Auto-refresh every 5 seconds

2. **Child Jobs Expandable View**
   - Click parent row to expand child jobs
   - Show child job hierarchy with indentation
   - Phase indicators (pre, core, post)

3. **Job Logs Modal**
   - Click "Logs" button to open modal
   - Real-time log streaming via WebSocket
   - Filter by level (debug, info, warn, error)
   - Timestamps, color-coded levels

4. **Queue Statistics**
   - Pending jobs count
   - In-flight jobs count
   - Completed/Failed counts (24h)
   - Average execution time

**Technology:**
- Alpine.js for reactivity
- Bulma CSS for styling
- WebSocket for real-time updates

**Effort:** 6-10 hours
**Dependencies:** Gap 2.1 and 2.2 must be resolved first
**Priority:** High for operational visibility

---

### Gap 3.3: WebSocket Real-Time Updates Missing

**Severity:** P1 - High
**Impact:** UI requires manual refresh, no live progress updates
**Status:** ⚠️ Partially implemented, broadcaster disabled

**Evidence:**
- `internal/app/app.go` lines 684-699: WebSocketHandler initialized
- EventService subscription exists but queue stats broadcaster commented out
- No real-time job status broadcasts to UI

**Root Cause:**
WebSocket infrastructure exists but was partially disabled during refactor. Event publishing for job lifecycle events not connected.

**Recommendation:**
Re-enable and complete WebSocket broadcasting:

1. **Job Lifecycle Events** (Priority 1)
   - Broadcast on status changes: pending → running → completed/failed
   - Include job ID, status, timestamps in payload
   - Subscribe WebSocketHandler to job events

2. **Progress Updates** (Priority 2)
   - Broadcast when `UpdateJobProgress()` is called
   - Include current/total progress
   - Real-time progress bar updates in UI

3. **Log Streaming** (Priority 3)
   - Broadcast new log entries as they're added
   - Include level, message, timestamp
   - Append to logs modal in real-time

4. **Queue Statistics** (Priority 4)
   - Re-enable queue stats broadcaster (currently commented out)
   - Broadcast pending/in-flight message counts
   - Update dashboard stats live

**Effort:** 4-6 hours
**Dependencies:** Gap 3.2 (UI must exist to receive updates)
**Priority:** High for user experience

---

## 4. Technical Debt (P2 - Maintenance)

### Gap 4.1: Documentation Consistency

**Severity:** P2 - Medium
**Impact:** Developer confusion, incorrect assumptions
**Status:** ⚠️ Partially complete

**Current State:**
- ✅ `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md` - Comprehensive and accurate (updated 2025-11-03)
- ✅ `docs/architecture/README.md` - Architecture overview documentation
- ⚠️ `IMPLEMENTATION_TODO.md` - Marked as "COMPLETED" but system incomplete
- ⚠️ `AGENTS.md` - May reference old architecture
- ⚠️ `README.md` - Likely references old queue system

**Recommendation:**
1. Update `IMPLEMENTATION_TODO.md` to reflect actual state (65% complete, transition state)
2. Review `AGENTS.md` for outdated queue/job references
3. Update `README.md` queue section to reference new architecture
4. Cross-reference architecture docs in all relevant places

**Effort:** 2-3 hours
**Dependencies:** Should be done after P0/P1 gaps resolved
**Priority:** Medium - important for maintainability

---

### Gap 4.2: Test Coverage Incomplete

**Severity:** P2 - Medium
**Impact:** Regressions may go undetected
**Status:** ⚠️ Partial

**Evidence:**
- `IMPLEMENTATION_TODO.md` Phase 13-14: Testing marked "⚠️ Partially Complete"
- `test/api/job_error_tolerance_test.go.disabled` - References deleted package
- Unit tests pass but integration tests incomplete

**Recommendation:**
Write comprehensive tests for new queue system:

1. **JobManager Tests** (Priority 1)
   - CRUD operations against actual `crawl_jobs` table
   - Parent-child job relationships
   - Progress tracking
   - Log storage and retrieval

2. **CrawlerExecutor Tests** (Priority 1)
   - Integration with crawler.Service
   - Child job creation
   - URL deduplication via job_seen_urls
   - Error handling

3. **Worker Pool Tests** (Priority 2)
   - End-to-end job processing
   - Graceful shutdown
   - Multiple concurrent workers
   - Executor routing

4. **API Endpoint Tests** (Priority 3)
   - Job creation via API
   - Job status retrieval
   - Job logs retrieval
   - Error responses

**Effort:** 4-6 hours
**Dependencies:** Gap 2.1 and 2.2 must be resolved first
**Priority:** Medium - important for confidence

---

### Gap 4.3: Migration File Not Applied

**Severity:** P2 - Medium
**Impact:** Confusion about intended schema
**Status:** ⚠️ Migration exists but never applied

**Evidence:**
- `docs/migrations/008_redesign_job_queue.sql` exists and defines `jobs` table
- Migration was never integrated into `schema.go`'s `runMigrations()` method
- Schema continues to use `crawl_jobs` table

**Recommendation:**
- **If using Option A (migrate to jobs table):** Apply migration 008
- **If using Option B (keep crawl_jobs table):** Archive or delete migration file to avoid confusion

**Effort:** 1 hour
**Dependencies:** Resolution of Gap 2.1 determines action
**Priority:** Low - cleanup task

---

## 5. Architectural Divergences

### 5.1: Table Structure Complexity

**Plan:** Simple `jobs` table with 13 fields
- id, parent_id, job_type, phase, status, created_at, started_at, completed_at, payload, result, error, progress_current, progress_total

**Reality:** Complex `crawl_jobs` table with 21 fields
- Adds: name, description, source_type, entity_type, config_json, source_config_snapshot, auth_snapshot, refresh_source, seed_urls, metadata, progress_json, last_heartbeat, result_count, failed_count

**Analysis:**
- **Pros of Reality:** Domain-specific fields support existing crawler use cases, preserves backward compatibility
- **Cons of Reality:** Violates separation of concerns (job metadata mixed with crawler-specific config), makes table less generic

**Recommendation:** Accept current structure for pragmatism, but document that `crawl_jobs` serves dual purpose (generic jobs + crawler-specific data).

---

### 5.2: Job Logs Table

**Plan:** 5 fields (id, job_id, timestamp, level, message)
**Reality:** 6 fields (adds created_at)

**Analysis:**
Minor divergence. `created_at` is redundant with `timestamp` but harmless.

**Recommendation:** No action needed, accept as-is.

---

### 5.3: Dual Job Systems

**Plan:** Single queue-based system for all job execution
**Reality:** Two systems coexist:
- Queue-based (goqite + WorkerPool + Executors) for task execution
- JobExecutor (currently disabled) for workflow orchestration

**Analysis:**
- **Pros:** Separation of concerns (tasks vs workflows), supports complex multi-step jobs
- **Cons:** Complexity, two systems to maintain, currently one is disabled

**Recommendation:**
Accept dual system architecture. Document clearly that:
- **Queue System:** Low-level task execution (URL crawling, data processing)
- **JobExecutor:** High-level workflow orchestration (multi-step job definitions)

Both serve different purposes and should coexist.

---

## 6. Comparison Table: Plan vs Reality

| Component | Original Plan | Current Reality | Status |
|-----------|--------------|-----------------|--------|
| Queue Manager | goqite wrapper | ✅ Implemented | Complete |
| Message Struct | 3 fields (JobID, Type, Payload) | ✅ Implemented | Complete |
| Worker Pool | Executor pattern | ✅ Implemented | Complete |
| Job Manager | CRUD for `jobs` table | ⚠️ Queries `jobs`, schema has `crawl_jobs` | **MISMATCH** |
| CrawlerExecutor | Full implementation | ❌ Placeholder only | **INCOMPLETE** |
| SummarizerExecutor | Planned | ❌ Not created | Not Started |
| CleanupExecutor | Planned | ❌ Not created | Not Started |
| JobExecutor | Not mentioned in plan | ⚠️ Exists but disabled | **DISABLED** |
| Queue UI | Full management page | ❌ Auth/sources page only | **MISSING** |
| WebSocket Updates | Real-time job status | ⚠️ Partially implemented, disabled | **INCOMPLETE** |
| Database Migration | `jobs` table | ❌ `crawl_jobs` table, migration not applied | **NOT APPLIED** |
| Job Logs Table | 5 fields | ✅ 6 fields (extra created_at) | Minor Divergence |
| job_seen_urls Table | Not in plan | ✅ Implemented | Extra Feature |
| API Endpoints | Full CRUD | ⚠️ Implemented but unused | Untested |
| Tests | Unit + Integration | ⚠️ Partial coverage | Incomplete |

---

## 7. Recommendations by Priority

### Phase 1: Critical Fixes (P0) - 12-22 hours

**Goal:** Make system functional for basic queue operations

1. **Resolve database schema mismatch** (4-6 hours)
   - Update `internal/jobs/manager.go` to use `crawl_jobs` table
   - Map Job struct fields to crawl_jobs columns
   - Handle nullable/extra fields
   - Test all CRUD operations

2. **Implement CrawlerExecutor** (8-16 hours)
   - Integrate with `crawler.Service`
   - Implement URL fetching logic
   - Add child job creation
   - Implement URL deduplication
   - Add progress tracking
   - Test end-to-end job execution

**Success Criteria:**
- ✅ Jobs can be created via API without errors
- ✅ Worker pool processes jobs successfully
- ✅ CrawlerExecutor fetches URLs
- ✅ Job status transitions work (pending → running → completed/failed)
- ✅ Job logs are stored and retrievable

---

### Phase 2: UX Improvements (P1) - 18-28 hours

**Goal:** Provide operational visibility and monitoring

1. **Create queue management UI** (6-10 hours)
   - Parent/child jobs table
   - Job logs modal
   - Status filtering
   - Auto-refresh

2. **Re-enable WebSocket updates** (4-6 hours)
   - Job lifecycle event broadcasting
   - Progress updates
   - Log streaming
   - Queue statistics

3. **Re-enable JobExecutor system** (8-12 hours)
   - Update to work with new queue architecture
   - Test multi-step workflows
   - Ensure coexistence with queue-based jobs

**Success Criteria:**
- ✅ Queue management UI displays jobs
- ✅ WebSocket updates provide real-time progress
- ✅ JobExecutor can orchestrate workflows
- ✅ Users can monitor queue without manual refresh

---

### Phase 3: Technical Debt (P2) - 7-10 hours

**Goal:** Improve maintainability and confidence

1. **Update documentation** (2-3 hours)
   - Update `IMPLEMENTATION_TODO.md` status
   - Review `AGENTS.md` for outdated refs
   - Update `README.md` queue section

2. **Write comprehensive tests** (4-6 hours)
   - JobManager integration tests
   - CrawlerExecutor tests
   - Worker pool end-to-end tests
   - API endpoint tests

3. **Clean up migration file** (1 hour)
   - Archive or delete `008_redesign_job_queue.sql`
   - Document schema decision

**Success Criteria:**
- ✅ Documentation reflects current state
- ✅ All tests pass
- ✅ No confusing migration files

---

**Total Estimated Effort: 37-60 hours**

---

## 8. Decision Points

### Decision 1: Schema Migration Strategy

**Option A: Migrate to `jobs` Table**
- **Pros:** Cleaner separation, matches refactor plan, future-proof
- **Cons:** Breaking change, data migration required, 8-12 hours extra work
- **Use Case:** Long-term maintainability, clean architecture

**Option B: Keep `crawl_jobs` Table, Update Code**
- **Pros:** Preserves data, simpler, faster (4-6 hours), backward compatible
- **Cons:** Violates separation of concerns, domain-specific fields in generic table
- **Use Case:** Quick functionality, pragmatic solution

**Recommendation:** Option B for immediate functionality. Plan Option A for future cleanup in major version.

---

### Decision 2: JobExecutor System

**Option A: Re-enable with Queue Integration**
- **Pros:** Supports multi-step workflows, valuable feature for users
- **Cons:** Added complexity, 8-12 hours work, two systems to maintain
- **Use Case:** Users relying on job definitions, complex workflows

**Option B: Remove Entirely**
- **Pros:** Simpler architecture, one system, easier to maintain
- **Cons:** Loses workflow orchestration, users must implement externally
- **Use Case:** Minimal feature set, simplicity priority

**Recommendation:** Option A - workflows are valuable feature, worth the complexity.

---

### Decision 3: Additional Executors

**Option A: Implement SummarizerExecutor, CleanupExecutor Now**
- **Pros:** Complete refactor vision, full feature set
- **Cons:** 12-20 hours additional work
- **Use Case:** Feature completeness, production readiness

**Option B: Defer to Future Iterations**
- **Pros:** Focus on core functionality first, faster initial completion
- **Cons:** Incomplete system, users wait for features
- **Use Case:** MVP approach, iterative development

**Recommendation:** Option B - get core working first, add executors in future iterations.

---

## 9. Success Criteria

The refactor will be considered **complete** when:

1. ✅ Jobs can be created via API without errors
2. ✅ Worker pool processes jobs successfully
3. ✅ CrawlerExecutor fetches URLs and creates child jobs
4. ✅ Job status transitions correctly (pending → running → completed/failed)
5. ✅ Job logs are stored and retrievable
6. ✅ Queue management UI displays jobs and logs
7. ✅ WebSocket updates provide real-time progress
8. ✅ JobExecutor can orchestrate multi-step workflows
9. ✅ All unit and integration tests pass
10. ✅ Documentation reflects current architecture

---

## 10. Risk Assessment

### High Risk

- **Schema mismatch may have cascading effects** - Other systems may depend on table names
- **CrawlerExecutor integration may reveal missing dependencies** - crawler.Service may not support required operations
- **Re-enabling JobExecutor may conflict with queue-based jobs** - Dual systems may interfere

**Mitigation:** Thorough testing at each step, incremental rollout, feature flags.

---

### Medium Risk

- **UI implementation may require additional API endpoints** - Current endpoints may not provide all needed data
- **WebSocket updates may cause performance issues under load** - Many concurrent connections could impact server
- **Test coverage gaps may hide regressions** - Incomplete tests may miss critical bugs

**Mitigation:** Load testing, comprehensive test suite, monitoring.

---

### Low Risk

- **Documentation updates are straightforward** - No code changes required
- **Migration file cleanup has no functional impact** - Purely organizational

**Mitigation:** None needed.

---

## 11. Next Steps

### Immediate Actions (This Week)

1. **Review and approve** this gap analysis
2. **Create branch** for schema mismatch fix: `fix/schema-mismatch`
3. **Update `internal/jobs/manager.go`** to query `crawl_jobs` table
4. **Test job creation and retrieval** with actual database
5. **Commit and create PR** for review

---

### Short-Term (Next 2 Weeks)

1. **Implement CrawlerExecutor** with crawler.Service integration
2. **Test end-to-end job execution** (create job → process → complete)
3. **Create queue management UI** with Alpine.js and Bulma
4. **Re-enable WebSocket updates** for real-time monitoring

---

### Medium-Term (Next Month)

1. **Re-enable JobExecutor system** for workflow orchestration
2. **Write comprehensive test suite** (unit + integration)
3. **Update all documentation** to reflect current state
4. **Consider implementing additional executors** (summarizer, cleanup)

---

## 12. References

### Key Files

**Core Implementation:**
- Queue: `internal/queue/manager.go`, `internal/queue/types.go`
- Jobs: `internal/jobs/manager.go`
- Workers: `internal/worker/pool.go`
- Executors: `internal/worker/crawler_executor.go`
- App: `internal/app/app.go` (lines 288-353, 684-699)

**Database:**
- Schema: `internal/storage/sqlite/schema.go` (lines 114-173)
- Migration (not applied): `docs/migrations/008_redesign_job_queue.sql`

**Documentation:**
- Current Architecture: `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md` (comprehensive, updated 2025-11-03)
- Architecture Overview: `docs/architecture/README.md`
- Original Plan: `docs/development/refactor-queue-manager/01-refactor.md`
- Implementation Tracker: `docs/development/refactor-queue-manager/IMPLEMENTATION_TODO.md`

**UI:**
- Current Page: `pages/jobs.html` (auth/sources/job definitions)
- Missing: `pages/queue.html` (queue monitoring)

---

### Related Documents

- **Architecture Overview:** `docs/architecture/README.md`
- **Manager-Worker Architecture:** `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md`
- **Agent Guidelines:** `AGENTS.md` (needs update)
- **Main README:** `README.md` (needs update)

---

## 13. Conclusion

The queue refactor is **65% complete** with critical gaps blocking production use. The primary blocker is the database schema mismatch between code (`jobs` table) and schema (`crawl_jobs` table).

**System State:** Transition - core infrastructure is solid (goqite, WorkerPool, Message struct, JobManager) but business logic is incomplete (CrawlerExecutor is placeholder, JobExecutor disabled, UI missing).

**Path Forward:** Fix schema mismatch first (Option B - update code to use `crawl_jobs`), then implement CrawlerExecutor, then build UI. This approach provides incremental value and reduces risk.

**Timeline:** With focused effort on P0 and P1 gaps, the system can be production-ready in 3-4 weeks (37-60 hours total).

**Recommended Approach:**
1. **Week 1:** Fix schema mismatch, test CRUD operations
2. **Week 2:** Implement CrawlerExecutor, test end-to-end execution
3. **Week 3:** Build queue management UI, re-enable WebSocket updates
4. **Week 4:** Re-enable JobExecutor, write comprehensive tests, update docs

---

**Document Version:** 1.0
**Last Reviewed:** 2025-11-03
**Next Review:** After Phase 1 completion
