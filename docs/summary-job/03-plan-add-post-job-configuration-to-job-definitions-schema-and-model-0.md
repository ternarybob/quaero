I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The task requires adding post-job execution capability to job definitions, allowing jobs to trigger other jobs upon completion. This is a database schema evolution task that follows well-established patterns in the codebase.

**Current State:**
- `job_definitions` table has 13 columns (id, name, type, description, sources, steps, schedule, timeout, enabled, auto_start, config, created_at, updated_at)
- `JobDefinition` model has corresponding fields with marshal/unmarshal helpers for JSON arrays (Sources, Steps) and maps (Config)
- 13 migrations already exist in `runMigrations()` sequence
- CRUD operations in `JobDefinitionStorage` handle serialization/deserialization consistently

**Key Patterns Identified:**
1. **Migration Pattern:** Check column existence via `PRAGMA table_info`, add column with `ALTER TABLE`, backfill if needed, log progress
2. **Model Pattern:** Add field to struct, create `MarshalPostJobs()` and `UnmarshalPostJobs()` methods following `MarshalSources()` pattern
3. **Storage Pattern:** Update INSERT/UPDATE queries to include new column, update scan methods to deserialize JSON

**Design Decision:**
Store `post_jobs` as TEXT column containing JSON array of job definition IDs (e.g., `["job-id-1", "job-id-2"]`). This matches the existing pattern for `sources` field and allows flexible post-job configuration without schema changes.

**Subsequent Phases:**
The implementation plan notes that post-job execution logic (JobExecutor changes) and UI updates are handled by other engineers. This phase focuses solely on the data model and persistence layer.

### Approach

Add `post_jobs` column to the database schema, create a migration function to update existing databases, extend the `JobDefinition` model with the new field and helper methods, and update all CRUD operations to handle the new column. Follow existing patterns for array serialization and migration implementation.

### Reasoning

I examined the repository structure, read the schema definition in `internal/storage/sqlite/schema.go`, analyzed the `JobDefinition` model in `internal/models/job_definition.go`, and reviewed CRUD operations in `internal/storage/sqlite/job_definition_storage.go`. I identified the migration pattern by studying existing migrations like `migrateAddJobDefinitionsTimeoutColumn` and the marshal/unmarshal pattern from `MarshalSources` and `MarshalSteps` methods.

## Mermaid Diagram

sequenceDiagram
    participant App as Application Startup
    participant Schema as schema.go
    participant Migration as migrateAddPostJobsColumn
    participant Model as JobDefinition Model
    participant Storage as JobDefinitionStorage
    participant DB as SQLite Database

    Note over App,DB: Schema Initialization & Migration
    App->>Schema: InitSchema()
    Schema->>Schema: Execute schemaSQL (with post_jobs column)
    Schema->>Schema: runMigrations()
    Schema->>Migration: migrateAddPostJobsColumn()
    Migration->>DB: PRAGMA table_info(job_definitions)
    DB-->>Migration: Column list
    
    alt post_jobs column missing
        Migration->>DB: ALTER TABLE ADD COLUMN post_jobs TEXT
        Migration->>DB: UPDATE SET post_jobs = '[]' WHERE NULL
        Migration-->>Schema: Success
    else post_jobs column exists
        Migration-->>Schema: Skip (already migrated)
    end

    Note over App,DB: Job Definition CRUD Operations
    App->>Storage: SaveJobDefinition(jobDef)
    Storage->>Model: MarshalPostJobs()
    Model-->>Storage: JSON string ["job-id-1", "job-id-2"]
    Storage->>DB: INSERT with post_jobs column
    DB-->>Storage: Success

    App->>Storage: GetJobDefinition(id)
    Storage->>DB: SELECT including post_jobs
    DB-->>Storage: Row with post_jobs JSON
    Storage->>Model: UnmarshalPostJobs(json)
    Model-->>Storage: []string array
    Storage-->>App: JobDefinition with PostJobs field

## Proposed File Changes

### internal\storage\sqlite\schema.go(MODIFY)

**Add post_jobs column to job_definitions table schema (line 194-208):**

In the `schemaSQL` constant, add a new column to the `job_definitions` table definition:
- After the `config TEXT` line (line 205), add: `post_jobs TEXT`
- This column will store a JSON array of job definition IDs to execute after job completion
- Use TEXT type to match the pattern of `sources` and `steps` columns

**Add migration function migrateAddPostJobsColumn (after line 394):**

Create a new migration function following the exact pattern of `migrateAddJobDefinitionsTimeoutColumn` (lines 352-394):
- Function signature: `func (s *SQLiteDB) migrateAddPostJobsColumn() error`
- Query `PRAGMA table_info(job_definitions)` to check if `post_jobs` column exists
- Iterate through rows to find column named "post_jobs"
- If column exists, return nil (migration already completed)
- If column doesn't exist:
  - Log: "Running migration: Adding post_jobs column to job_definitions"
  - Execute: `ALTER TABLE job_definitions ADD COLUMN post_jobs TEXT`
  - Backfill existing rows: `UPDATE job_definitions SET post_jobs = '[]' WHERE post_jobs IS NULL`
  - Log: "Migration: post_jobs column added successfully"
- Return nil on success, error on failure

**Register migration in runMigrations sequence (after line 346):**

Add the new migration to the `runMigrations()` function:
- After MIGRATION 13 (migrateRemoveLogsColumn), add comment: `// MIGRATION 14: Add post_jobs column to job_definitions table`
- Call: `if err := s.migrateAddPostJobsColumn(); err != nil { return err }`
- This ensures the migration runs on existing databases during startup

### internal\models\job_definition.go(MODIFY)

**Add PostJobs field to JobDefinition struct (after line 93):**

Add a new field to the `JobDefinition` struct:
- After the `Config map[string]interface{}` field (line 93), add: `PostJobs []string` with JSON tag `json:"post_jobs"`
- This field stores an array of job definition IDs to execute after this job completes
- Empty array means no post-jobs configured
- Example: `["default-corpus-summary", "custom-notification-job"]`

**Add MarshalPostJobs method (after line 225):**

Create a new method following the exact pattern of `MarshalSources` (lines 186-193):
- Function signature: `func (j *JobDefinition) MarshalPostJobs() (string, error)`
- Marshal `j.PostJobs` to JSON using `json.Marshal`
- Return error with message "failed to marshal post_jobs" if marshaling fails
- Return JSON string on success
- This method serializes the post_jobs array for database storage

**Add UnmarshalPostJobs method (after MarshalPostJobs):**

Create a new method following the exact pattern of `UnmarshalSources` (lines 195-201):
- Function signature: `func (j *JobDefinition) UnmarshalPostJobs(data string) error`
- Unmarshal JSON string into `j.PostJobs` using `json.Unmarshal`
- Return error with message "failed to unmarshal post_jobs" if unmarshaling fails
- Return nil on success
- This method deserializes the post_jobs JSON from database

**Update documentation comment (lines 81-96):**

Add documentation for the new field in the struct comment:
- After the `Config` field description, add: `PostJobs` - Array of job definition IDs to execute after this job completes successfully. Jobs are executed independently (no parent/child relationship).

### internal\storage\sqlite\job_definition_storage.go(MODIFY)

References: 

- internal\models\job_definition.go(MODIFY)

**Update SaveJobDefinition method (lines 40-122):**

Add post_jobs serialization and persistence:
- After line 71 (configJSON marshaling), add:
  - Call `jobDef.MarshalPostJobs()` to serialize the PostJobs array
  - Store result in `postJobsJSON` variable
  - Handle error by returning it (following same pattern as other marshal calls)
- Update INSERT query (lines 88-105):
  - Add `post_jobs` to column list (line 91, after `config`)
  - Add `?` placeholder to VALUES clause (line 92)
  - Add `post_jobs = excluded.post_jobs` to ON CONFLICT UPDATE clause (line 104)
- Update ExecContext call (lines 107-111):
  - Add `postJobsJSON` parameter after `configJSON` parameter

**Update UpdateJobDefinition method (lines 124-211):**

Add post_jobs serialization and update:
- After line 164 (configJSON marshaling), add:
  - Call `jobDef.MarshalPostJobs()` to serialize the PostJobs array
  - Store result in `postJobsJSON` variable
  - Handle error by returning it
- Update UPDATE query (lines 180-194):
  - Add `post_jobs = ?` to SET clause (line 191, after `config = ?`)
- Update ExecContext call (lines 196-200):
  - Add `postJobsJSON` parameter after `configJSON` parameter (before `updatedAt`)

**Update GetJobDefinition method (lines 213-232):**

Add post_jobs to SELECT query:
- Update SELECT query (lines 215-220):
  - Add `post_jobs` to column list (after `config`, before `created_at`)
  - Use `COALESCE(post_jobs, '[]') AS post_jobs` to handle NULL values

**Update ListJobDefinitions method (lines 234-297):**

Add post_jobs to SELECT query:
- Update SELECT query (lines 236-241):
  - Add `post_jobs` to column list (after `config`, before `created_at`)
  - Use `COALESCE(post_jobs, '[]') AS post_jobs` to handle NULL values

**Update GetJobDefinitionsByType method (lines 299-316):**

Add post_jobs to SELECT query:
- Update SELECT query (lines 301-307):
  - Add `post_jobs` to column list (after `config`, before `created_at`)
  - Use `COALESCE(post_jobs, '[]') AS post_jobs` to handle NULL values

**Update GetEnabledJobDefinitions method (lines 318-335):**

Add post_jobs to SELECT query:
- Update SELECT query (lines 320-326):
  - Add `post_jobs` to column list (after `config`, before `created_at`)
  - Use `COALESCE(post_jobs, '[]') AS post_jobs` to handle NULL values

**Update CreateDefaultJobDefinitions method (lines 376-565):**

Initialize PostJobs field for default jobs:
- For `dbMaintenanceJob` (line 381-404):
  - After `Config: make(map[string]interface{})` (line 401), add: `PostJobs: []string{}`
- For `corpusSummaryJob` (line 474-495):
  - After `Config: make(map[string]interface{})` (line 492), add: `PostJobs: []string{}`
- For both jobs, add marshaling after config marshaling:
  - Call `MarshalPostJobs()` and store in `postJobsJSON` variable
  - Handle errors with appropriate logging
- Update INSERT queries for both jobs:
  - Add `post_jobs` to column list
  - Add `?` placeholder to VALUES clause
  - Add `postJobsJSON` parameter to ExecContext call

**Update scanJobDefinition method (lines 567-625):**

Add post_jobs deserialization:
- Update variable declarations (line 570):
  - Add `postJobsJSON` to the list of string variables
- Update Scan call (lines 575-578):
  - Add `&postJobsJSON` parameter (after `&configJSON`, before `&createdAt`)
- After UnmarshalConfig call (lines 616-622), add:
  - Call `jobDef.UnmarshalPostJobs(postJobsJSON)` to deserialize post_jobs
  - Log warning if unmarshaling fails (following same pattern as other unmarshal calls)
  - Set `jobDef.PostJobs = []string{}` as fallback on error

**Update scanJobDefinitions method (lines 627-698):**

Add post_jobs deserialization:
- Update variable declarations (line 633):
  - Add `postJobsJSON` to the list of string variables
- Update Scan call (lines 638-641):
  - Add `&postJobsJSON` parameter (after `&configJSON`, before `&createdAt`)
- After UnmarshalConfig call (lines 681-687), add:
  - Call `jobDef.UnmarshalPostJobs(postJobsJSON)` to deserialize post_jobs
  - Log warning if unmarshaling fails
  - Set `jobDef.PostJobs = []string{}` as fallback on error