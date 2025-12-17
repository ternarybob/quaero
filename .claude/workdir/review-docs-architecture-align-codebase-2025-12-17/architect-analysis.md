# Architect Analysis: Documentation vs Codebase Alignment

**Date:** 2025-12-17
**Task:** Comprehensively review `docs/architecture` and align to the current codebase

## Executive Summary

The architecture documentation has significant drift from the current codebase. Major discrepancies include:

1. **SQLite references are obsolete** - All storage now uses BadgerDB exclusively
2. **Atlassian transformers don't exist** - The `internal/services/atlassian/` directory is missing entirely
3. **WORKERS.md is outdated** - Missing 2 new workers, includes 2 non-existent workers
4. **Worker type constants exist without implementations** - `transform` and `reindex` types are defined but unused

## Detailed Findings

### 1. Storage Architecture (CRITICAL)

**Documentation Claims:** (ARCHITECTURE.md, lines 588-600)
- SQLite for document storage with FTS5 full-text search
- `internal/storage/sqlite/document_storage.go`

**Actual Implementation:**
- BadgerDB exclusively (via badgerhold wrapper)
- All storage in `/internal/storage/badger/`
- **NO SQLite directory or implementation exists**

**Files Affected:**
- `ARCHITECTURE.md` - Lines 588-600, SQL examples throughout
- `QUEUE_SERVICES.md` - Line 21-24 (references BadgerDB correctly)

**Resolution:** MODIFY - Update ARCHITECTURE.md to reflect BadgerDB storage

---

### 2. Atlassian Transformers (CRITICAL)

**Documentation Claims:** (ARCHITECTURE.md, lines 111-125, 166-174, 362, 385)
- `internal/services/atlassian/jira_transformer.go`
- `internal/services/atlassian/confluence_transformer.go`
- `internal/services/atlassian/helpers.go`
- Transformers subscribe to events and process HTML inline

**Actual Implementation:**
- **Directory `internal/services/atlassian/` does NOT exist**
- HTML-to-Markdown conversion moved to `internal/services/transform/service.go`
- Crawler handles document creation directly (immediate save feature)

**Files Affected:**
- `ARCHITECTURE.md` - Lines 111-125, 166-174, 362, 385, 421-471

**Resolution:** MODIFY - Remove transformer references, update to reflect current crawler-based architecture

---

### 3. WORKERS.md Discrepancies

**Workers in Docs but NOT in Code:**

| Worker | Status | Resolution |
|--------|--------|------------|
| Extract Structure Worker | NOT IMPLEMENTED | Remove from docs |
| Database Maintenance Worker | NOT IMPLEMENTED (deprecated note) | Remove from docs |

**Workers in Code but NOT in Docs:**

| Worker | File | Type | Resolution |
|--------|------|------|------------|
| Email Worker | `email_worker.go` | `email` | Add to docs |
| Test Job Generator Worker | `test_job_generator_worker.go` | `test_job_generator` | Add to docs |

**Worker Types Defined but No Implementation:**

| Type | Notes |
|------|-------|
| `WorkerTypeTransform` | Defined in `worker_type.go` but no implementation |
| `WorkerTypeReindex` | Defined in `worker_type.go` but no implementation |

**Files Affected:**
- `WORKERS.md` - Table of Contents (lines 5-25), Worker sections, Classification tables

---

### 4. Crawler Service Structure

**Documentation Claims:** (ARCHITECTURE.md, Section 3)
- Lists 6 files in crawler/atlassian services

**Actual Implementation:**
- 3 documented files exist in crawler/
- 13 additional undocumented files exist
- 3 atlassian files don't exist

**Crawler Files NOT Documented:**
- `chromedp_pool.go` - Browser pool management
- `content_processor.go` - Content processing
- `crawled_document.go` - Document model
- `document_persister.go` - Persistence logic
- `executor.go` - Execution handling
- `filters.go` - URL filtering
- `hybrid_scraper.go` - Hybrid scraping
- `image_storage.go` - Image handling
- `link_extractor.go` - Link extraction
- `rate_limiter.go` - Rate limiting
- `types.go` - Type definitions

**Resolution:** MODIFY - Update ARCHITECTURE.md Section 3 to reflect current crawler structure

---

### 5. Manager/Worker Architecture (QUEUE_SERVICES.md, MANAGER_WORKER_ARCHITECTURE.md)

**Status:** ACCURATE with minor issues

**Correct:**
- JobManager at `internal/queue/job_manager.go` - VERIFIED
- StepManager at `internal/queue/step_manager.go` - VERIFIED
- Orchestrator at `internal/queue/orchestrator.go` - VERIFIED
- Event service architecture - VERIFIED

**Minor Updates Needed:**
- QUEUE_SERVICES.md line 141: References `queue.NewBadgerQueueManager` - verify function name

---

### 6. README.md Linking Issues

**Issues Found:**
- Line 34 references `architecture.md` (lowercase) - file is `ARCHITECTURE.md`
- Line 84 references `manager_worker_architecture.md` (lowercase) - file is `MANAGER_WORKER_ARCHITECTURE.md`

---

## Resolution Plan

### Changes Required

| File | Action | Priority |
|------|--------|----------|
| `ARCHITECTURE.md` | Major rewrite - remove SQLite, remove atlassian, update crawler | HIGH |
| `WORKERS.md` | Remove 2 missing workers, add 2 new workers | HIGH |
| `README.md` | Fix file path casing | LOW |
| `QUEUE_SERVICES.md` | Minor verification | LOW |
| `QUEUE_LOGGING.md` | No changes needed | - |
| `QUEUE_UI.md` | No changes needed | - |
| `BUSINESS-CASE.md` | No changes needed (strategy doc) | - |

### Anti-Creation Bias Check

**No new files needed.** All changes are MODIFICATIONS to existing documentation.

### Worker Type Constants Recommendation

The following constants in `internal/models/worker_type.go` have no implementations:
- `WorkerTypeTransform`
- `WorkerTypeReindex`

**Recommendation:** Either implement these workers OR remove the constants. This is outside the scope of documentation alignment but should be noted.

## Summary

The documentation requires significant updates but no new files. The core architecture (Manager/Worker pattern, event system, queue services) is accurately documented. The main issues are:

1. Storage backend changed from SQLite to BadgerDB
2. Atlassian transformers were removed/refactored
3. WORKERS.md is missing 2 workers and includes 2 non-existent ones
4. Crawler service has many undocumented files

**Recommendation:** Proceed with documentation updates focusing on ARCHITECTURE.md and WORKERS.md as highest priority.
