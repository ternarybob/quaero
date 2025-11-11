# Plan: Remove ALL Database Migrations (Database Rebuilt Every Time)

**Date:** 2025-11-08
**Author:** Agent 1 (Planner)
**Status:** Ready for Agent 2 Implementation
**User Clarification:** App is in single-user testing, database rebuilt from scratch EVERY time

## Executive Summary

**MAJOR CLARIFICATION FROM USER:**
- The application is in single-user testing phase
- Database is rebuilt from scratch on EVERY startup
- ALL migrations should be removed (not just 17)
- Only keep CREATE TABLE statements with final schema

**New Approach:**
- Remove ALL 29 migration functions
- Update CREATE TABLE statements to include all columns that migrations would have added
- Database initialization becomes: just execute CREATE statements
- Much simpler, cleaner approach

**Expected Impact:**
- Remove ~1,700 lines of migration code (~68% of file)
- Eliminate ALL migration logic
- Simpler, faster initialization
- Single source of truth: CREATE TABLE statements

---

## Current State Analysis

### Current Schema (Lines 14-200)

**Tables in schemaSQL constant:**
1. `auth_credentials` (Lines 17-33)
2. `documents` (Lines 38-55)
3. `llm_audit_log` (Lines 58-67)
4. `jobs` (Lines 73-93)
5. `job_seen_urls` (Lines 104-110)
6. `job_logs` (Lines 117-125)
7. `job_settings` (Lines 133-140)
8. `job_definitions` (Lines 143-165)

### All Migrations to Remove (Lines 232-395)

**ALL 29 migrations will be removed:**

| # | Function | Lines | Purpose |
|---|----------|-------|---------|
| 1 | `migrateCrawlJobsColumns()` | 446-517 | Add columns to removed crawl_jobs |
| 2 | `migrateToMarkdownOnly()` | 520-713 | Remove content column from documents |
| 3 | `migrateAddHeartbeatColumn()` | 718-769 | Add column to removed crawl_jobs |
| 4 | `migrateAddLastRunColumn()` | 772-808 | Add last_run to job_settings |
| 5 | `migrateAddJobLogsColumn()` | 814-818 | Deprecated |
| 6 | `migrateAddJobNameDescriptionColumns()` | 822-872 | Add columns to removed crawl_jobs |
| 7 | `migrateAddJobSettingsDescriptionColumn()` | 875-911 | Add description to job_settings |
| 8 | `migrateAddJobDefinitionsTable()` | 914-966 | Create job_definitions table |
| 9 | (commented) | 969-976 | Add seed_urls to removed sources |
| 10 | `migrateRemoveSourcesFilteringColumns()` | 980-1129 | Modify removed sources |
| 11 | `migrateAddJobDefinitionsTimeoutColumn()` | 399-441 | Add timeout to job_definitions |
| 12 | `migrateAddBackSourcesFiltersColumn()` | 1132-1168 | Add filters to removed sources |
| 13 | `migrateRemoveLogsColumn()` | 1178-1364 | Remove logs from removed crawl_jobs |
| 14 | `migrateAddPostJobsColumn()` | 1367-1409 | Add post_jobs to job_definitions |
| 15 | `migrateAddParentIdColumn()` | 1412-1461 | Add parent_id to removed crawl_jobs |
| 16 | `migrateEnableForeignKeysAndAddParentConstraint()` | 1465-1654 | FK constraint on removed crawl_jobs |
| 17 | `migrateAddPreJobsColumn()` | 1657-1699 | Add pre_jobs to job_definitions |
| 18 | `migrateAddJobTypeColumn()` | 1702-1757 | Add job_type to removed crawl_jobs |
| 19 | `migrateRenameParentIdIndex()` | 1760-1792 | Rename index on removed crawl_jobs |
| 20 | `migrateAddErrorToleranceColumn()` | 1795-1831 | Add error_tolerance to job_definitions |
| 21 | `migrateAddMetadataColumn()` | 1834-1878 | Add metadata to removed crawl_jobs |
| 22 | `migrateCleanupOrphanedOrchestrationJobs()` | 1886-2010 | Clean removed crawl_jobs |
| 23 | `migrateCleanupOrphanedJobSettings()` | 2016-2056 | Clean job_settings |
| 24 | `migrateAddFinishedAtColumn()` | 2060-2107 | Add finished_at to removed crawl_jobs |
| 25 | `migrateCrawlJobsToUnifiedJobs()` | 2113-2231 | Migrate removed crawl_jobs to jobs |
| 26 | `migrateRemoveSourcesTable()` | 2234-2400 | Drop removed sources |
| 27 | `migrateAddJobDefinitionTypeColumn()` | 2403-2433 | Add job_type to job_definitions |
| 28 | `migrateAddTomlColumn()` | 2436-2466 | Add toml to job_definitions |
| 29 | `migrateRemoveAtlassianTables()` | 2471-2493 | Drop Atlassian tables |

**Total to remove:** ALL 29 migrations (~1,700 lines)

---

## Schema Analysis: What Columns Need to be Added to CREATE Statements

### Table 1: `auth_credentials` (Lines 17-33)
**Current CREATE statement:** Already complete, no migrations affect it
**Action:** No changes needed

### Table 2: `documents` (Lines 38-55)
**Current CREATE statement:** Already complete
**Migration 2 removed:** Old `content` column (already gone from CREATE)
**Action:** No changes needed (CREATE already correct)

### Table 3: `llm_audit_log` (Lines 58-67)
**Current CREATE statement:** Already complete, no migrations affect it
**Action:** No changes needed

### Table 4: `jobs` (Lines 73-93)
**Current CREATE statement:** Already complete with all final columns
**Migrations that would have modified crawl_jobs before conversion:** All irrelevant (table is `jobs` not `crawl_jobs`)
**Action:** No changes needed (CREATE already has final schema)

### Table 5: `job_seen_urls` (Lines 104-110)
**Current CREATE statement:** Already complete, no migrations affect it
**Action:** No changes needed

### Table 6: `job_logs` (Lines 117-125)
**Current CREATE statement:** Already complete, no migrations affect it
**Action:** No changes needed

### Table 7: `job_settings` (Lines 133-140)
**Current CREATE statement:**
```sql
CREATE TABLE IF NOT EXISTS job_settings (
	job_name TEXT PRIMARY KEY,
	schedule TEXT NOT NULL,
	description TEXT DEFAULT '',
	enabled INTEGER DEFAULT 1,
	last_run INTEGER,
	updated_at INTEGER NOT NULL
);
```

**Columns added by migrations:**
- `last_run` - Migration 4 (already in CREATE)
- `description` - Migration 7 (already in CREATE)

**Action:** No changes needed (CREATE already complete)

### Table 8: `job_definitions` (Lines 143-165)
**Current CREATE statement:**
```sql
CREATE TABLE IF NOT EXISTS job_definitions (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	type TEXT NOT NULL,
	job_type TEXT NOT NULL DEFAULT 'user',
	description TEXT,
	source_type TEXT,
	base_url TEXT,
	auth_id TEXT,
	steps TEXT NOT NULL,
	schedule TEXT NOT NULL,
	timeout TEXT,
	enabled INTEGER DEFAULT 1,
	auto_start INTEGER DEFAULT 0,
	config TEXT,
	post_jobs TEXT,
	error_tolerance TEXT,
	toml TEXT,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	FOREIGN KEY (auth_id) REFERENCES auth_credentials(id) ON DELETE SET NULL,
	CHECK (job_type IN ('system', 'user'))
);
```

**Columns added by migrations:**
- `timeout` - Migration 11 (already in CREATE)
- `post_jobs` - Migration 14 (already in CREATE)
- `pre_jobs` - Migration 17 (MISSING from CREATE!)
- `error_tolerance` - Migration 20 (already in CREATE)
- `job_type` - Migration 27 (already in CREATE)
- `toml` - Migration 28 (already in CREATE)

**Action:** Add `pre_jobs TEXT` column to CREATE statement

---

## Updated Schema Required

### job_definitions Table - Add Missing Column

**Current CREATE (Lines 143-165):**
```sql
CREATE TABLE IF NOT EXISTS job_definitions (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	type TEXT NOT NULL,
	job_type TEXT NOT NULL DEFAULT 'user',
	description TEXT,
	source_type TEXT,
	base_url TEXT,
	auth_id TEXT,
	steps TEXT NOT NULL,
	schedule TEXT NOT NULL,
	timeout TEXT,
	enabled INTEGER DEFAULT 1,
	auto_start INTEGER DEFAULT 0,
	config TEXT,
	post_jobs TEXT,
	error_tolerance TEXT,
	toml TEXT,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	FOREIGN KEY (auth_id) REFERENCES auth_credentials(id) ON DELETE SET NULL,
	CHECK (job_type IN ('system', 'user'))
);
```

**NEW CREATE (add pre_jobs column):**
```sql
CREATE TABLE IF NOT EXISTS job_definitions (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	type TEXT NOT NULL,
	job_type TEXT NOT NULL DEFAULT 'user',
	description TEXT,
	source_type TEXT,
	base_url TEXT,
	auth_id TEXT,
	steps TEXT NOT NULL,
	schedule TEXT NOT NULL,
	timeout TEXT,
	enabled INTEGER DEFAULT 1,
	auto_start INTEGER DEFAULT 0,
	config TEXT,
	pre_jobs TEXT,
	post_jobs TEXT,
	error_tolerance TEXT,
	toml TEXT,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	FOREIGN KEY (auth_id) REFERENCES auth_credentials(id) ON DELETE SET NULL,
	CHECK (job_type IN ('system', 'user'))
);
```

**Change:** Add `pre_jobs TEXT,` after `config TEXT,` (line 157)

---

## Implementation Plan for Agent 2

### Step 1: Update job_definitions CREATE Statement

**File:** `C:\development\quaero\internal\storage\sqlite\schema.go`
**Lines:** 143-165

**Action:** Add missing `pre_jobs TEXT,` column

**Before (Line 157):**
```sql
	config TEXT,
	post_jobs TEXT,
```

**After:**
```sql
	config TEXT,
	pre_jobs TEXT,
	post_jobs TEXT,
```

### Step 2: Remove ALL Migration Function Calls

**File:** `C:\development\quaero\internal\storage\sqlite\schema.go`
**Function:** `runMigrations()` (Lines 232-396)

**Replace entire function with:**

```go
// runMigrations is deprecated - database is rebuilt from scratch on each startup
// Schema is fully defined in schemaSQL constant above
// This function is kept for backward compatibility but does nothing
func (s *SQLiteDB) runMigrations() error {
	s.logger.Debug().Msg("Migration system disabled - database built from scratch")
	return nil
}
```

### Step 3: Remove ALL 29 Migration Function Definitions

**Delete all migration functions:**

1. `migrateCrawlJobsColumns()` - Lines 443-517
2. `migrateToMarkdownOnly()` - Lines 520-713
3. `migrateAddHeartbeatColumn()` - Lines 715-769
4. `migrateAddLastRunColumn()` - Lines 772-808
5. `migrateAddJobLogsColumn()` - Lines 810-818
6. `migrateAddJobNameDescriptionColumns()` - Lines 820-872
7. `migrateAddJobSettingsDescriptionColumn()` - Lines 874-911
8. `migrateAddJobDefinitionsTable()` - Lines 913-966
9. Commented `migrateAddSourcesSeedURLsColumn()` - Lines 968-976
10. `migrateRemoveSourcesFilteringColumns()` - Lines 978-1129
11. `migrateAddJobDefinitionsTimeoutColumn()` - Lines 398-441
12. `migrateAddBackSourcesFiltersColumn()` - Lines 1131-1168
13. `migrateRemoveLogsColumn()` - Lines 1170-1364
14. `migrateAddPostJobsColumn()` - Lines 1366-1409
15. `migrateAddParentIdColumn()` - Lines 1411-1461
16. `migrateEnableForeignKeysAndAddParentConstraint()` - Lines 1463-1654
17. `migrateAddPreJobsColumn()` - Lines 1656-1699
18. `migrateAddJobTypeColumn()` - Lines 1701-1757
19. `migrateRenameParentIdIndex()` - Lines 1759-1792
20. `migrateAddErrorToleranceColumn()` - Lines 1794-1831
21. `migrateAddMetadataColumn()` - Lines 1833-1878
22. `migrateCleanupOrphanedOrchestrationJobs()` - Lines 1880-2010
23. `migrateCleanupOrphanedJobSettings()` - Lines 2012-2056
24. `migrateAddFinishedAtColumn()` - Lines 2058-2107
25. `migrateCrawlJobsToUnifiedJobs()` - Lines 2109-2231
26. `migrateRemoveSourcesTable()` - Lines 2233-2400
27. `migrateAddJobDefinitionTypeColumn()` - Lines 2402-2433
28. `migrateAddTomlColumn()` - Lines 2435-2466
29. `migrateRemoveAtlassianTables()` - Lines 2468-2493

**Delete everything from line 398 to end of file (after InitSchema function)**

### Step 4: Update InitSchema Function

**Current InitSchema (Lines 203-229):**
```go
// InitSchema initializes the database schema
func (s *SQLiteDB) InitSchema() error {
	_, err := s.db.Exec(schemaSQL)
	if err != nil {
		return err
	}
	s.logger.Info().Msg("Database schema initialized")

	// Run migrations for schema evolution
	if err := s.runMigrations(); err != nil {
		return err
	}

	// Create default job definitions after schema and migrations are complete
	// This ensures the job_definitions table exists and has the correct schema
	ctx := context.Background()
	jobDefStorage := NewJobDefinitionStorage(s, s.logger)
	if jds, ok := jobDefStorage.(*JobDefinitionStorage); ok {
		if err := jds.CreateDefaultJobDefinitions(ctx); err != nil {
			// Log warning but don't fail startup - default job definitions are a convenience feature
			s.logger.Warn().Err(err).Msg("Failed to create default job definitions")
		} else {
			s.logger.Debug().Msg("Default job definitions initialized")
		}
	}

	return nil
}
```

**New InitSchema (simplified):**
```go
// InitSchema initializes the database schema
// Database is built from scratch on each startup - no migrations needed
func (s *SQLiteDB) InitSchema() error {
	// Execute schema SQL to create all tables
	_, err := s.db.Exec(schemaSQL)
	if err != nil {
		return fmt.Errorf("failed to create database schema: %w", err)
	}
	s.logger.Info().Msg("Database schema initialized")

	// Create default job definitions
	ctx := context.Background()
	jobDefStorage := NewJobDefinitionStorage(s, s.logger)
	if jds, ok := jobDefStorage.(*JobDefinitionStorage); ok {
		if err := jds.CreateDefaultJobDefinitions(ctx); err != nil {
			// Log warning but don't fail startup - default job definitions are a convenience feature
			s.logger.Warn().Err(err).Msg("Failed to create default job definitions")
		} else {
			s.logger.Debug().Msg("Default job definitions initialized")
		}
	}

	return nil
}
```

**Changes:**
1. Remove call to `runMigrations()`
2. Update comment to reflect no-migration approach
3. Add fmt import if not already present

---

## Final File Structure

After cleanup, `schema.go` will have:

```
Lines 1-13:    Package declaration and imports
Lines 14-200:  const schemaSQL (CREATE TABLE statements)
Lines 201-230: InitSchema() function (simplified)
Lines 231-XXX: (end of file - all migration functions removed)
```

**Estimated final size:** ~230 lines (down from ~2,494 lines)
**Reduction:** ~90% smaller file

---

## Validation Steps for Agent 2

### Step 1: Update Schema
1. Add `pre_jobs TEXT,` to job_definitions CREATE statement

### Step 2: Verify Schema Completeness
```bash
cd C:\development\quaero
grep "CREATE TABLE IF NOT EXISTS" internal/storage/sqlite/schema.go
```
Expected: 8 tables, each with complete column definitions

### Step 3: Build Test
```powershell
cd C:\development\quaero
go build ./cmd/quaero
```
Expected: Clean build with no errors

### Step 4: Database Initialization Test
```powershell
# Delete existing database
Remove-Item -Path ".\quaero.db" -ErrorAction SilentlyContinue

# Run application
.\scripts\build.ps1 -Run
```
Expected: Clean startup, no migration messages

### Step 5: Verify Tables Created
```powershell
# Use SQLite CLI to verify all 8 tables exist with correct schema
sqlite3 quaero.db ".schema job_definitions"
```
Expected: job_definitions has pre_jobs column

### Step 6: Run Tests
```powershell
cd test/api
go test -v ./...
```
Expected: All tests pass

### Step 7: Verify Logs
Check startup logs for:
- ✅ "Database schema initialized"
- ✅ "Default job definitions initialized"
- ❌ No "Running migration:" messages
- ❌ No "Skipping migration:" messages

---

## Risk Assessment

### Risk Level: VERY LOW

**Justification:**
1. User confirmed database is ALWAYS rebuilt from scratch
2. No backward compatibility needed
3. schemaSQL CREATE statements already have final schema (except pre_jobs)
4. Simple, clean approach

### Rollback Plan

If issues occur:
1. `git revert` the commit
2. Rebuild application
3. Report issues

---

## Expected Benefits

### Code Quality
1. **Massive reduction:** Remove ~1,700 lines (68% of file)
2. **Single source of truth:** Only CREATE statements matter
3. **Zero migration complexity:** No conditional logic
4. **Faster startup:** No migration checks
5. **Cleaner logs:** No migration messages

### Developer Experience
1. **Crystal clear intent:** Database built from scratch
2. **Easy to understand:** Just read CREATE statements
3. **No confusion:** No migration history to trace

### Performance
1. **Instant initialization:** Just execute CREATE statements
2. **Smaller binary:** Much less code to compile

---

## Summary of Changes

**Files Modified:** 1 file
- `C:\development\quaero\internal\storage\sqlite\schema.go`

**Changes:**
1. Add `pre_jobs TEXT,` to job_definitions CREATE statement
2. Replace runMigrations() with no-op stub
3. Remove ALL 29 migration functions (~1,700 lines)
4. Simplify InitSchema() to just execute CREATE statements

**Testing:**
- Build test
- Database initialization test
- Schema verification
- Existing tests

**Result:**
- Clean, simple schema.go (~230 lines instead of ~2,494)
- No migration overhead
- Single source of truth: CREATE TABLE statements

---

## Ready for Agent 2 Implementation

**Status:** READY
**Estimated Time:** 15-20 minutes
**Complexity:** LOW (mostly deletions)
**Risk:** VERY LOW (database rebuilt from scratch)

Agent 2 should:
1. Add pre_jobs column to job_definitions CREATE
2. Gut runMigrations() function
3. Delete all migration functions
4. Simplify InitSchema()
5. Run all validation steps
6. Commit with clear message
