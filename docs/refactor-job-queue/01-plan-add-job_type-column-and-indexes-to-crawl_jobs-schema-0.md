I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current Architecture:**
- Migration system uses PRAGMA table_info checks for idempotency
- Last migration is #17 (`migrateAddPreJobsColumn`)
- `crawl_jobs` table already has `parent_id` column with CASCADE DELETE constraint
- SaveJob/GetJob methods use explicit column lists (not SELECT *)
- Job type constants follow pattern: `JobStatus` enum with string values

**Key Findings:**
- `parent_id` index already exists: `idx_jobs_parent_id`
- Foreign key constraint on `parent_id` already enforced (MIGRATION 16)
- No `job_type` column exists in schema or model
- Breaking changes acceptable per user requirement
- Migration is simple (add column with default) - no fresh database needed

### Approach

Add `job_type` column to `crawl_jobs` table with migration support, update the `CrawlJob` model with type constants, and modify storage layer to handle the new field. This enables hierarchical job tracking (parent, pre-validation, crawler_url, post-summary) for the Queue Management UI refactor.

### Reasoning

Listed repository structure to understand project layout, read schema.go to understand table structure and migration patterns, examined crawler_job.go model to understand current fields, reviewed job_storage.go to understand how fields are persisted and retrieved, and identified the exact locations where changes are needed.

## Mermaid Diagram

sequenceDiagram
    participant App as Application Startup
    participant Schema as schema.go
    participant Migration as MIGRATION 18
    participant DB as SQLite Database
    participant Model as CrawlJob Model
    participant Storage as JobStorage

    App->>Schema: InitSchema()
    Schema->>Schema: Execute schemaSQL (CREATE TABLE with job_type)
    Schema->>Migration: runMigrations()
    Migration->>DB: PRAGMA table_info(crawl_jobs)
    
    alt job_type column missing
        Migration->>DB: ALTER TABLE ADD COLUMN job_type
        Migration->>DB: UPDATE crawl_jobs SET job_type='parent'
        Migration->>DB: CREATE INDEX idx_crawl_jobs_type_status
        Migration-->>Schema: Migration complete
    else job_type exists
        Migration-->>Schema: Skip (idempotent)
    end

    Note over App,Storage: Runtime Operations

    App->>Storage: SaveJob(crawlJob)
    Storage->>Model: Read crawlJob.JobType
    Storage->>DB: INSERT/UPDATE with job_type column
    DB-->>Storage: Success

    App->>Storage: GetJob(jobID)
    Storage->>DB: SELECT including job_type
    DB-->>Storage: Row data with job_type
    Storage->>Model: Populate CrawlJob.JobType
    Storage-->>App: CrawlJob with JobType field

## Proposed File Changes

### internal\storage\sqlite\schema.go(MODIFY)

**1. Update CREATE TABLE statement for crawl_jobs (lines 108-129):**
- Add `job_type TEXT DEFAULT 'parent'` column after `parent_id TEXT` line
- This ensures new installations have the column from the start

**2. Add new index in schema SQL (after line 135):**
- Add `CREATE INDEX IF NOT EXISTS idx_crawl_jobs_type_status ON crawl_jobs(job_type, status, created_at DESC);`
- This index optimizes queries filtering by job type and status (common UI pattern)

**3. Add MIGRATION 18 in runMigrations method (after line 371):**
- Add call to `s.migrateAddJobTypeColumn()` after the `migrateAddPreJobsColumn()` call
- Follow the sequential migration pattern established in the codebase

**4. Implement migrateAddJobTypeColumn method (after line 1656):**
- Check if `job_type` column exists using `PRAGMA table_info(crawl_jobs)`
- If column exists, return nil (idempotent migration)
- Log migration start: "Running migration: Adding job_type column to crawl_jobs"
- Execute `ALTER TABLE crawl_jobs ADD COLUMN job_type TEXT DEFAULT 'parent'`
- Backfill existing rows: `UPDATE crawl_jobs SET job_type = 'parent' WHERE job_type IS NULL`
- Create index: `CREATE INDEX IF NOT EXISTS idx_crawl_jobs_type_status ON crawl_jobs(job_type, status, created_at DESC)`
- Log migration completion: "Migration: job_type column added successfully"
- Return nil on success, error on failure

**Pattern Reference:** Follow the exact pattern used in `migrateAddParentIdColumn()` (lines 1384-1426) which adds a column and creates an index in a single migration.

### internal\models\crawler_job.go(MODIFY)

**1. Add JobType constants (after line 17):**
- Define new constant type: `type JobType string`
- Add constants following the JobStatus pattern:
  - `JobTypeParent JobType = "parent"` - Root job that orchestrates workflow
  - `JobTypePreValidation JobType = "pre_validation"` - Pre-flight validation job
  - `JobTypeCrawlerURL JobType = "crawler_url"` - Individual URL crawling job
  - `JobTypePostSummary JobType = "post_summary"` - Post-processing summarization job

**2. Add JobType field to CrawlJob struct (after line 36):**
- Add `JobType JobType \`json:"job_type"\`` field after the `ParentID` field
- This maintains logical grouping (ParentID and JobType are both hierarchy-related)
- Default value will be handled by database DEFAULT constraint

**Pattern Reference:** Follow the exact pattern used for `JobStatus` type definition (lines 8-17) with string-based constants.

### internal\storage\sqlite\job_storage.go(MODIFY)

References: 

- internal\models\crawler_job.go(MODIFY)

**1. Update SaveJob INSERT/UPDATE query (lines 122-142):**
- Add `job_type` to column list in INSERT statement (after `parent_id`)
- Add `?` placeholder to VALUES clause (after `parent_id` placeholder)
- Add `job_type = excluded.job_type` to ON CONFLICT UPDATE SET clause
- Add `string(crawlJob.JobType)` to ExecContext args list (after `parentID` argument at line 146)
- This ensures job_type is persisted when creating or updating jobs

**2. Update GetJob SELECT query (lines 213-218):**
- Add `job_type` to SELECT column list (after `parent_id`)
- Maintain column order consistency with SaveJob

**3. Update ListJobs SELECT query (lines 226-229):**
- Add `job_type` to SELECT column list (after `parent_id`)
- Maintain column order consistency with other queries

**4. Update GetJobsByStatus SELECT query (lines 317-323):**
- Add `job_type` to SELECT column list (after `parent_id`)
- Maintain column order consistency

**5. Update GetChildJobs SELECT query (lines 338-346):**
- Add `job_type` to SELECT column list (after `parent_id`)
- Maintain column order consistency

**6. Update scanJob method (lines 595-686):**
- Add `jobType string` variable to the var declaration block (line 598)
- Add `&jobType` to row.Scan() call (after `&parentID` at line 610)
- Add `JobType: models.JobType(jobType)` to CrawlJob struct initialization (after `ParentID` field at line 658)

**7. Update scanJobs method (lines 688-784):**
- Add `jobType string` variable to the var declaration block (line 694)
- Add `&jobType` to rows.Scan() call (after `&parentID` at line 706)
- Add `JobType: models.JobType(jobType)` to CrawlJob struct initialization (after `ParentID` field at line 753)

**Important:** All SELECT queries must include `job_type` in the same position to maintain consistency with Scan() operations. The order is: `id, parent_id, job_type, name, description, ...`