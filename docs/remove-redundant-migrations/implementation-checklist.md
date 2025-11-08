# Implementation Checklist for Agent 2

**Date:** 2025-11-08
**Task:** Remove redundant database migrations
**File:** `C:\development\quaero\internal\storage\sqlite\schema.go`

---

## Quick Stats

- **Remove:** 17 migrations (~1,410 lines)
- **Keep:** 11 migrations (operate on existing tables)
- **Result:** File shrinks from 2,494 → ~1,084 lines (56% reduction)

---

## Step-by-Step Implementation

### STEP 1: Remove Migration Calls from runMigrations()

**Location:** Lines 232-395 in `runMigrations()` function

**Actions:**
```
☐ Remove MIGRATION 1 call (migrateCrawlJobsColumns)
☐ Remove MIGRATION 3 call (migrateAddHeartbeatColumn)
☐ Remove MIGRATION 5 call (migrateAddJobLogsColumn)
☐ Remove MIGRATION 6 call (migrateAddJobNameDescriptionColumns)
☐ Delete MIGRATION 10 comment block (already commented)
☐ Delete MIGRATION 12 comment block (already commented)
☐ Remove MIGRATION 13 call (migrateRemoveLogsColumn)
☐ Remove MIGRATION 15 call (migrateAddParentIdColumn)
☐ Remove MIGRATION 16 call (migrateEnableForeignKeysAndAddParentConstraint)
☐ Remove MIGRATION 18 call (migrateAddJobTypeColumn)
☐ Remove MIGRATION 19 call (migrateRenameParentIdIndex)
☐ Remove MIGRATION 21 call (migrateAddMetadataColumn)
☐ Remove MIGRATION 22 call (migrateCleanupOrphanedOrchestrationJobs)
☐ Remove MIGRATION 24 call (migrateAddFinishedAtColumn)
☐ Remove MIGRATION 25 call (migrateCrawlJobsToUnifiedJobs)
☐ Remove MIGRATION 26 call (migrateRemoveSourcesTable)
☐ Remove MIGRATION 29 call (migrateRemoveAtlassianTables)
```

**Keep these calls:**
- Migration 2: migrateToMarkdownOnly
- Migration 4: migrateAddLastRunColumn
- Migration 7: migrateAddJobSettingsDescriptionColumn
- Migration 8: migrateAddJobDefinitionsTable
- Migration 11: migrateAddJobDefinitionsTimeoutColumn
- Migration 14: migrateAddPostJobsColumn
- Migration 17: migrateAddPreJobsColumn
- Migration 20: migrateAddErrorToleranceColumn
- Migration 23: migrateCleanupOrphanedJobSettings
- Migration 27: migrateAddJobDefinitionTypeColumn
- Migration 28: migrateAddTomlColumn

### STEP 2: Remove Function Definitions

**Locate and delete these complete functions:**

```
☐ migrateCrawlJobsColumns() - ~lines 446-517
☐ migrateAddHeartbeatColumn() - ~lines 718-769
☐ migrateAddJobLogsColumn() - ~lines 814-818
☐ migrateAddJobNameDescriptionColumns() - ~lines 822-872
☐ migrateRemoveSourcesFilteringColumns() - ~lines 980-1129
☐ migrateAddBackSourcesFiltersColumn() - ~lines 1132-1168
☐ migrateRemoveLogsColumn() - ~lines 1178-1364
☐ migrateAddParentIdColumn() - ~lines 1412-1461
☐ migrateEnableForeignKeysAndAddParentConstraint() - ~lines 1465-1654
☐ migrateAddJobTypeColumn() - ~lines 1702-1757
☐ migrateRenameParentIdIndex() - ~lines 1760-1792
☐ migrateAddMetadataColumn() - ~lines 1834-1878
☐ migrateCleanupOrphanedOrchestrationJobs() - ~lines 1886-2010
☐ migrateAddFinishedAtColumn() - ~lines 2060-2107
☐ migrateCrawlJobsToUnifiedJobs() - ~lines 2113-2231
☐ migrateRemoveSourcesTable() - ~lines 2234-2400
☐ migrateRemoveAtlassianTables() - ~lines 2471-2493
```

### STEP 3: Remove Commented Code

```
☐ Delete commented migrateAddSourcesSeedURLsColumn() - ~lines 969-976
```

### STEP 4: Renumber Remaining Migrations

Update `runMigrations()` comments to sequential numbering:

```
☐ MIGRATION 1: migrateToMarkdownOnly (was 2)
☐ MIGRATION 2: migrateAddLastRunColumn (was 4)
☐ MIGRATION 3: migrateAddJobSettingsDescriptionColumn (was 7)
☐ MIGRATION 4: migrateAddJobDefinitionsTable (was 8)
☐ MIGRATION 5: migrateAddJobDefinitionsTimeoutColumn (was 11)
☐ MIGRATION 6: migrateAddPostJobsColumn (was 14)
☐ MIGRATION 7: migrateAddPreJobsColumn (was 17)
☐ MIGRATION 8: migrateAddErrorToleranceColumn (was 20)
☐ MIGRATION 9: migrateCleanupOrphanedJobSettings (was 23)
☐ MIGRATION 10: migrateAddJobDefinitionTypeColumn (was 27)
☐ MIGRATION 11: migrateAddTomlColumn (was 28)
```

---

## Validation Tests

### Pre-Implementation Verification

```powershell
☐ Count migration functions (should be 29):
   grep -c "func (s \*SQLiteDB) migrate" internal/storage/sqlite/schema.go

☐ Verify current schema (should show 8 tables):
   grep "CREATE TABLE IF NOT EXISTS" internal/storage/sqlite/schema.go

☐ Backup schema.go:
   Copy-Item internal/storage/sqlite/schema.go internal/storage/sqlite/schema.go.backup
```

### Post-Implementation Validation

```powershell
☐ Count remaining migration functions (should be 12):
   grep -c "func (s \*SQLiteDB) migrate" internal/storage/sqlite/schema.go

☐ Build test (should succeed):
   cd C:\development\quaero
   go build ./cmd/quaero

☐ Delete existing database:
   Remove-Item -Path ".\quaero.db" -ErrorAction SilentlyContinue

☐ Initialize fresh database:
   .\scripts\build.ps1 -Run

☐ Check for skip messages in logs (should be none):
   # Look for "Skipping" in console output
   # Should NOT see any "table does not exist" messages

☐ Verify schema with SQLite:
   sqlite3 quaero.db ".schema"
   # Should show 8 tables with correct structure

☐ Run API tests:
   cd test/api
   go test -v ./...

☐ Run UI tests:
   cd test/ui
   go test -v ./...

☐ Check final file size:
   # Should be ~1,084 lines (down from 2,494)
   (Get-Content internal/storage/sqlite/schema.go).Length
```

---

## Success Criteria

### All Must Pass ✅

1. **Build succeeds** with no errors
2. **Fresh database initializes** without errors
3. **No "Skipping migration" messages** in logs
4. **Schema has 8 tables** (auth_credentials, documents, llm_audit_log, jobs, job_seen_urls, job_logs, job_settings, job_definitions)
5. **All tests pass** (API and UI)
6. **File size reduced** by ~50%
7. **12 migration functions remain** (11 active + ToMarkdownOnly)

---

## Rollback Plan

If validation fails:

```powershell
☐ Restore backup:
   Copy-Item internal/storage/sqlite/schema.go.backup internal/storage/sqlite/schema.go

☐ Rebuild:
   go build ./cmd/quaero

☐ Verify rollback works:
   .\scripts\build.ps1 -Run

☐ Report issue to Agent 1
```

---

## Common Pitfalls to Avoid

❌ **Don't remove these migrations:**
- migrateToMarkdownOnly
- migrateAddLastRunColumn
- migrateAddJobSettingsDescriptionColumn
- migrateAddJobDefinitionsTable
- migrateAddJobDefinitionsTimeoutColumn
- migrateAddPostJobsColumn
- migrateAddPreJobsColumn
- migrateAddErrorToleranceColumn
- migrateCleanupOrphanedJobSettings
- migrateAddJobDefinitionTypeColumn
- migrateAddTomlColumn

❌ **Don't modify:**
- schemaSQL constant (lines 14-200)
- InitSchema() function (lines 202-229)

❌ **Don't forget:**
- Renumber remaining migrations sequentially
- Run ALL validation tests
- Check logs for skip messages

---

## Time Estimate

- **Reading plan:** 10 minutes
- **Implementation:** 15-20 minutes
- **Validation:** 10-15 minutes
- **Total:** 30-45 minutes

---

## Ready to Start?

1. ✅ Read this checklist
2. ✅ Read full plan (plan.md)
3. ✅ Understand validation steps
4. ✅ Have rollback plan ready

**Start implementation!**

---

## Notes Section

Use this space to track progress or issues:

```
[Your notes here]
```

---

## Final Checklist

Before marking complete:

```
☐ All migration calls removed from runMigrations()
☐ All migration functions deleted
☐ Commented code removed
☐ Migrations renumbered
☐ Build succeeds
☐ Database initializes cleanly
☐ No skip messages in logs
☐ Schema correct (8 tables)
☐ API tests pass
☐ UI tests pass
☐ File size reduced ~50%
☐ Commit changes with descriptive message
```

**Status:** [ ] Complete
**Date completed:** _____________
**Issues found:** _____________
**Resolution:** _____________
