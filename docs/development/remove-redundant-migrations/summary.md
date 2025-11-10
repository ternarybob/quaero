# Remove Redundant Migrations - Planning Summary

**Date:** 2025-11-08
**Agent:** Agent 1 (Planner)
**Status:** ✅ Plan Complete - Ready for Agent 2

---

## Quick Overview

**Problem:** 17 out of 29 database migrations reference tables that no longer exist (`crawl_jobs`, `sources`, Atlassian tables), causing log noise and code bloat.

**Solution:** Remove redundant migrations since database is always built from scratch (breaking changes acceptable).

**Impact:**
- Remove ~1,410 lines of code (52% reduction)
- Eliminate "Skipping migration" log messages
- Cleaner, more maintainable codebase

**Risk:** LOW - Database rebuilt from scratch, no backward compatibility needed

---

## Analysis Results

### Current State
- **Total migrations:** 29
- **Active migrations (keep):** 11 (operate on existing tables)
- **Redundant migrations (remove):** 17 (reference non-existent tables)
- **Already commented:** 2 (sources table migrations)

### Tables in Current Schema (8 tables)
1. ✅ `auth_credentials` - Authentication
2. ✅ `documents` - Content and embeddings
3. ✅ `llm_audit_log` - LLM tracking
4. ✅ `jobs` - Unified job queue
5. ✅ `job_seen_urls` - URL deduplication
6. ✅ `job_logs` - Structured logs
7. ✅ `job_settings` - Scheduler settings
8. ✅ `job_definitions` - Job configuration

### Tables NOT in Schema (removed)
- ❌ `crawl_jobs` - Replaced by `jobs` table (Migration 25)
- ❌ `sources` - Merged into `job_definitions` (Migration 26)
- ❌ `jira_projects`, `jira_issues` - Removed (Migration 29)
- ❌ `confluence_spaces`, `confluence_pages` - Removed (Migration 29)

---

## Migrations to Remove

### crawl_jobs Migrations (13 migrations)
1. Migration 1: `migrateCrawlJobsColumns()` - 72 lines
2. Migration 3: `migrateAddHeartbeatColumn()` - 52 lines
3. Migration 5: `migrateAddJobLogsColumn()` - 5 lines (deprecated)
4. Migration 6: `migrateAddJobNameDescriptionColumns()` - 51 lines
5. Migration 13: `migrateRemoveLogsColumn()` - 187 lines
6. Migration 15: `migrateAddParentIdColumn()` - 50 lines
7. Migration 16: `migrateEnableForeignKeysAndAddParentConstraint()` - 190 lines
8. Migration 18: `migrateAddJobTypeColumn()` - 56 lines
9. Migration 19: `migrateRenameParentIdIndex()` - 33 lines
10. Migration 21: `migrateAddMetadataColumn()` - 45 lines
11. Migration 22: `migrateCleanupOrphanedOrchestrationJobs()` - 125 lines
12. Migration 24: `migrateAddFinishedAtColumn()` - 48 lines
13. Migration 25: `migrateCrawlJobsToUnifiedJobs()` - 119 lines

**Subtotal:** ~1,033 lines

### sources Migrations (3 migrations)
1. Migration 10: `migrateRemoveSourcesFilteringColumns()` - 150 lines (already commented)
2. Migration 12: `migrateAddBackSourcesFiltersColumn()` - 37 lines (already commented)
3. Migration 26: `migrateRemoveSourcesTable()` - 167 lines

**Subtotal:** ~354 lines

### Atlassian Migrations (1 migration)
1. Migration 29: `migrateRemoveAtlassianTables()` - 23 lines

**Subtotal:** ~23 lines

**Total lines removed:** ~1,410 lines

---

## Migrations to Keep (11 migrations)

These migrations operate on tables that exist in the current schema:

1. Migration 2: `migrateToMarkdownOnly()` - documents table
2. Migration 4: `migrateAddLastRunColumn()` - job_settings table
3. Migration 7: `migrateAddJobSettingsDescriptionColumn()` - job_settings table
4. Migration 8: `migrateAddJobDefinitionsTable()` - Creates job_definitions
5. Migration 11: `migrateAddJobDefinitionsTimeoutColumn()` - job_definitions table
6. Migration 14: `migrateAddPostJobsColumn()` - job_definitions table
7. Migration 17: `migrateAddPreJobsColumn()` - job_definitions table
8. Migration 20: `migrateAddErrorToleranceColumn()` - job_definitions table
9. Migration 23: `migrateCleanupOrphanedJobSettings()` - job_settings table
10. Migration 27: `migrateAddJobDefinitionTypeColumn()` - job_definitions table
11. Migration 28: `migrateAddTomlColumn()` - job_definitions table

---

## Validation Checklist for Agent 2

### Pre-Implementation
- [ ] Verify 29 migration functions exist
- [ ] Verify 8 tables in schema
- [ ] Backup current schema.go file

### Post-Implementation
- [ ] Verify 12 migration functions remain (11 active + ToMarkdownOnly)
- [ ] Build compiles successfully
- [ ] Fresh database initializes cleanly
- [ ] No "Skipping migration" log messages
- [ ] Schema has correct 8 tables
- [ ] API tests pass
- [ ] UI tests pass

---

## Evidence from Logs

Current startup logs show migrations being skipped:

```
Skipping crawl_jobs migration - table does not exist (using unified jobs table)
Skipping heartbeat migration - crawl_jobs table does not exist
Migration: jobs table already exists, crawl_jobs migration complete
sources table not found, skipping migration
```

After cleanup, these messages will disappear.

---

## Implementation Steps

1. **Remove migration calls** from `runMigrations()` function (lines 232-395)
2. **Remove migration function definitions** (~1,410 lines)
3. **Renumber remaining migrations** (1-11 instead of scattered numbers)
4. **Run validation tests** (build, initialize, test)
5. **Document changes** in commit message

---

## Why This Is Safe

1. **Database always rebuilt** - Breaking changes acceptable per user
2. **Migrations already skip** - All check for table existence first
3. **Schema is authoritative** - `schemaSQL` constant defines truth
4. **Comprehensive validation** - Multiple test layers catch issues
5. **Easy rollback** - Git revert if needed

---

## Expected Results

### Before
- File size: 2,494 lines
- Migrations: 29 total (17 always skip)
- Startup logs: Multiple "Skipping migration" messages

### After
- File size: ~1,084 lines (56% reduction)
- Migrations: 11 total (all active)
- Startup logs: Clean, no skip messages

---

## Files Affected

**Primary:**
- `C:\development\quaero\internal\storage\sqlite\schema.go`

**No other files need modification** - All migration logic is contained in this single file.

---

## Next Steps for Agent 2

1. Read full plan: `docs/remove-redundant-migrations/plan.md`
2. Follow implementation steps exactly
3. Run validation checklist
4. Report results

**Estimated time:** 30-45 minutes
**Risk level:** LOW
**Confidence:** HIGH

---

## Questions for Agent 2

None - Plan is comprehensive and ready for implementation. All edge cases have been considered and documented in the full plan.
