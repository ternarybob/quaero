I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The task requires extending the `CrawlJob` model and database schema to support URL exclusion tracking. The codebase already has well-established patterns for:

- JSON serialization of complex fields (see `CrawlConfig.ToJSON()`, `CrawlProgress.ToJSON()`)
- Database migrations with column existence checks (see `migrateCrawlJobsColumns()`)
- Nullable field handling in storage layer (see `SaveJob()` and `scanJob()`)

The exclusion feature needs three data points:
1. **ExcludedURLs map** - Store URL → reason mappings for audit trail
2. **ExclusionCount** - Top-level counter for quick access without deserializing JSON
3. **Progress counter** - Track exclusions alongside other URL metrics

The implementation follows existing patterns: `SeenURLs` field already demonstrates map serialization, and `ResultCount`/`FailedCount` show the counter pattern.

### Approach

Extend the `CrawlJob` model with exclusion tracking fields, add corresponding database columns with a migration, and update the storage layer to serialize/deserialize the new fields. This follows the established pattern used for `SeenURLs`, `ResultCount`, and other job metadata fields.

### Reasoning

Explored the repository structure to understand the codebase organization. Read the three key files mentioned in the task: `internal/models/crawler_job.go` for model structure, `internal/storage/sqlite/schema.go` for database schema and migration patterns, and `internal/storage/sqlite/job_storage.go` for serialization/deserialization logic. Analyzed existing patterns for JSON field handling, nullable columns, and migration implementation.

## Mermaid Diagram

sequenceDiagram
    participant Model as CrawlJob Model
    participant Schema as Database Schema
    participant Storage as JobStorage
    participant DB as SQLite Database

    Note over Model: Phase 1: Model Extension
    Model->>Model: Add ExcludedURLs map[string]string
    Model->>Model: Add ExclusionCount int
    Model->>Model: Add ExcludedURLs to CrawlProgress

    Note over Schema: Phase 2: Schema Migration
    Schema->>DB: Check if excluded_urls column exists
    DB-->>Schema: Column status
    alt Column doesn't exist
        Schema->>DB: ALTER TABLE ADD COLUMN excluded_urls TEXT
        Schema->>DB: ALTER TABLE ADD COLUMN exclusion_count INTEGER DEFAULT 0
        Schema->>DB: UPDATE crawl_jobs SET exclusion_count = 0 WHERE NULL
    end

    Note over Storage: Phase 3: Serialization (SaveJob)
    Storage->>Storage: Marshal ExcludedURLs map to JSON
    Storage->>DB: INSERT/UPDATE with excluded_urls, exclusion_count
    DB-->>Storage: Success

    Note over Storage: Phase 4: Deserialization (scanJob)
    Storage->>DB: SELECT including excluded_urls, exclusion_count
    DB-->>Storage: Row data
    Storage->>Storage: Unmarshal excluded_urls JSON to map
    Storage->>Storage: Build CrawlJob with exclusion fields
    Storage-->>Model: Populated CrawlJob

## Proposed File Changes

### internal\models\crawler_job.go(MODIFY)

**Add ExcludedURLs field to CrawlJob struct:**

Add a new field `ExcludedURLs map[string]string` after the `SeenURLs` field (around line 62). This map stores excluded URLs as keys with exclusion reasons as values (e.g., "empty_content", "no_data_table", "whitespace_only"). Use `json:"excluded_urls,omitempty"` tag for JSON serialization.

**Add ExclusionCount field to CrawlJob struct:**

Add a new field `ExclusionCount int` after the `FailedCount` field (around line 59). This provides a top-level counter for quick access without deserializing the JSON map. Use `json:"exclusion_count"` tag.

**Add ExcludedURLs counter to CrawlProgress struct:**

In the `CrawlProgress` struct (around line 80), add a new field `ExcludedURLs int` after the `FailedURLs` field (around line 83). This tracks exclusions alongside other URL metrics (total, completed, failed, pending). Use `json:"excluded_urls"` tag.

**Implementation Notes:**
- The `ExcludedURLs` map follows the same pattern as the existing `SeenURLs map[string]bool` field
- The `ExclusionCount` field follows the same pattern as `ResultCount` and `FailedCount`
- The progress counter enables real-time tracking during job execution
- JSON serialization is automatic via struct tags; no custom methods needed
- The map structure allows storing reasons for audit and debugging purposes

### internal\storage\sqlite\schema.go(MODIFY)

**Update crawl_jobs table schema definition:**

In the `schemaSQL` constant (around line 108-129), add two new columns to the `crawl_jobs` table CREATE statement:
- `excluded_urls TEXT` - Stores the JSON-serialized map of excluded URLs with reasons
- `exclusion_count INTEGER DEFAULT 0` - Stores the total count of excluded URLs

Add these columns after the `failed_count` column (around line 128) to maintain logical grouping with other count fields.

**Add migration to runMigrations():**

In the `runMigrations()` method (around line 278-363), add a new migration call after the existing MIGRATION 15 (around line 360):

```
// MIGRATION 16: Add excluded_urls and exclusion_count columns to crawl_jobs table
if err := s.migrateAddExclusionColumns(); err != nil {
    return err
}
```

**Create migrateAddExclusionColumns() method:**

Add a new migration method following the pattern of `migrateCrawlJobsColumns()` (see lines 410-473). The method should:

1. Query `PRAGMA table_info(crawl_jobs)` to check for existing columns
2. Scan rows to detect if `excluded_urls` and `exclusion_count` columns exist
3. If `excluded_urls` doesn't exist, execute `ALTER TABLE crawl_jobs ADD COLUMN excluded_urls TEXT`
4. If `exclusion_count` doesn't exist, execute `ALTER TABLE crawl_jobs ADD COLUMN exclusion_count INTEGER DEFAULT 0`
5. Backfill existing rows with `UPDATE crawl_jobs SET exclusion_count = 0 WHERE exclusion_count IS NULL`
6. Log migration progress using `s.logger.Info().Msg("Running migration: Adding exclusion columns to crawl_jobs")`

**Implementation Notes:**
- Follow the exact pattern used in `migrateCrawlJobsColumns()` for consistency
- Use `sql.NullString` for `excluded_urls` since it's optional (can be NULL or empty)
- Use `INTEGER DEFAULT 0` for `exclusion_count` to ensure non-null values
- The migration is idempotent (safe to run multiple times)
- Place the new method near other crawl_jobs migration methods (around line 410-473)

### internal\storage\sqlite\job_storage.go(MODIFY)

**Update SaveJob() method to serialize exclusion data:**

In the `SaveJob()` method (around lines 54-199), add serialization logic for the new fields:

1. **Serialize ExcludedURLs map** (add after seed URLs serialization around line 103):
   - Create `excludedURLsJSON` variable with default value `"{}"`
   - If `len(crawlJob.ExcludedURLs) > 0`, marshal the map using `json.Marshal(crawlJob.ExcludedURLs)`
   - Store the JSON string in `excludedURLsJSON`
   - Handle errors with `fmt.Errorf("failed to serialize excluded URLs: %w", err)`

2. **Add columns to INSERT statement** (around line 122-126):
   - Add `excluded_urls` and `exclusion_count` to the column list
   - Add corresponding placeholders `?` to the VALUES clause

3. **Add columns to UPDATE clause** (around line 127-141):
   - Add `excluded_urls = excluded.excluded_urls` to the ON CONFLICT DO UPDATE SET clause
   - Add `exclusion_count = excluded.exclusion_count` to the same clause

4. **Add parameters to ExecContext call** (around line 144-164):
   - Add `excludedURLsJSON` parameter after `seedURLsJSON`
   - Add `crawlJob.ExclusionCount` parameter after `excludedURLsJSON`

**Update scanJob() method to deserialize exclusion data:**

In the `scanJob()` method (around lines 586-676), add deserialization logic:

1. **Add scan variables** (around line 587-597):
   - Add `excludedURLsJSON sql.NullString` variable
   - Add `exclusionCount int` variable

2. **Update Scan() call** (around line 599-602):
   - Add `&excludedURLsJSON` and `&exclusionCount` to the scan parameters after `seedURLsJSON`

3. **Build CrawlJob with new fields** (around line 623-637):
   - Add `ExclusionCount: exclusionCount` to the struct initialization

4. **Deserialize ExcludedURLs map** (add after seed URLs deserialization around line 665-673):
   - Check if `excludedURLsJSON.Valid && excludedURLsJSON.String != ""`
   - Declare `var excludedURLs map[string]string`
   - Unmarshal using `json.Unmarshal([]byte(excludedURLsJSON.String), &excludedURLs)`
   - Assign to `job.ExcludedURLs = excludedURLs`
   - Log warning on error: `s.logger.Warn().Err(err).Str("job_id", id).Msg("Failed to deserialize excluded URLs")`

**Update scanJobs() method to deserialize exclusion data:**

In the `scanJobs()` method (around lines 679-774), apply the same changes as `scanJob()`:

1. Add scan variables (around line 683-693)
2. Update Scan() call (around line 695-698)
3. Build CrawlJob with new fields (around line 718-732)
4. Deserialize ExcludedURLs map (around line 760-768)

**Update query SELECT statements:**

Update all SELECT queries to include the new columns:
- `GetJob()` query (around line 203-207)
- `ListJobs()` query (around line 216-220)
- `GetJobsByStatus()` query (around line 307-312)
- `GetChildJobs()` query (around line 328-336)

Add `excluded_urls, exclusion_count` to each SELECT column list after `seed_urls`.

**Implementation Notes:**
- Follow the exact pattern used for `seed_urls` serialization/deserialization
- Use `sql.NullString` for `excluded_urls` to handle NULL values gracefully
- Use plain `int` for `exclusion_count` since it has a DEFAULT 0 constraint
- The map serialization is straightforward: empty map → `"{}"`, populated map → JSON object
- Error handling should log warnings but not fail the entire operation for deserialization errors