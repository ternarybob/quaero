I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Root Cause Analysis

**The Problem:**
Authentication cookies are not being injected into child crawler jobs because `auth_id` is missing from parent job metadata in the database.

**Current Flow (Broken):**
1. `job_executor.go` lines 110-116: Extracts `auth_id` from `jobDef.AuthID` and adds to `jobDefConfig` (Config field)
2. `job_executor.go` lines 315-324: Creates `parentMetadata` map and adds `auth_id` 
3. `job_executor.go` line 332: Sets `parentMetadata` on `parentJobModel` object
4. **CRITICAL GAP**: `parentJobModel` is only passed to `StartMonitoring` - metadata is NEVER persisted to database
5. `enhanced_crawler_executor_auth.go` lines 38-96: Tries to read `auth_id` from parent job metadata via `GetJob`
6. **FAILURE**: Database returns empty metadata because it was never saved

**Why This Happens:**
- The parent job record created at lines 63-80 uses the old `jobs.Job` format without metadata
- `UpdateJobConfig` at line 119 only updates the `config_json` field, not `metadata_json`
- The `parentJobModel` with metadata exists only in memory, never persisted
- Child jobs call `GetJob` which reads from database, finding no metadata

**Database Schema Confirmation:**
- `schema.go` line 78: `metadata_json TEXT` column exists in jobs table
- `manager.go` lines 949-952: `GetJob` already parses `metadata_json` from database
- `manager.go` lines 672-683: `UpdateJobConfig` exists but NO `UpdateJobMetadata` method

**Existing Inheritance Logic (Already Correct):**
- `enhanced_crawler_executor.go` lines 694-703: `spawnChildJob` copies parent metadata to child metadata
- This will work automatically once parent metadata is persisted to database

### Approach

## Solution Strategy

Add a new `UpdateJobMetadata` method to persist metadata to the database, then call it immediately after creating `parentMetadata` in the job executor.

**Two-File Fix:**
1. **Add persistence method** in `internal/jobs/manager.go`
2. **Call persistence method** in `internal/jobs/executor/job_executor.go`

**Why This Solution:**
- Minimal changes (2 files, ~20 lines total)
- Follows existing pattern (`UpdateJobConfig` at lines 672-683)
- No breaking changes to existing code
- Leverages existing `GetJob` metadata parsing (lines 949-952)
- Child job inheritance already works (lines 694-703 in enhanced_crawler_executor.go)

**Alternative Considered (Rejected):**
- Modify `CreateJobRecord` to accept metadata parameter
  - **Rejected**: Would require changing all callers, more invasive
  - Current approach is safer and more surgical

### Reasoning

I explored the repository structure and read the three key files mentioned by the user. I analyzed the job executor flow to understand how parent jobs are created and how metadata is supposed to be propagated. I searched for existing metadata update methods in the job manager and found only `UpdateJobConfig`. I examined the database schema to confirm the `metadata_json` column exists. I traced the child job spawning logic to verify that metadata inheritance is already implemented correctly. I reviewed the diagnostic logging in the auth injection code to understand why auth_id was not being found.

## Mermaid Diagram

sequenceDiagram
    participant UI as User/UI
    participant Executor as JobExecutor
    participant Manager as JobManager
    participant DB as SQLite Database
    participant Child as EnhancedCrawlerExecutor
    participant Auth as AuthStorage

    Note over UI,Auth: BEFORE FIX: auth_id lost in memory

    UI->>Executor: Execute(jobDef with auth_id)
    Executor->>Executor: Create parentMetadata map
    Executor->>Executor: Add auth_id to parentMetadata
    Note over Executor: ❌ Metadata only in memory!
    Executor->>Manager: CreateJobRecord (no metadata)
    Manager->>DB: INSERT jobs (metadata_json = NULL)
    Executor->>Executor: StartMonitoring (with in-memory metadata)
    
    Note over Child,Auth: Child job executes later
    Child->>Manager: GetJob(parentJobID)
    Manager->>DB: SELECT metadata_json
    DB-->>Manager: NULL (no metadata saved)
    Manager-->>Child: Job with empty metadata
    Child->>Child: Check metadata["auth_id"]
    Note over Child: ❌ auth_id NOT found!
    Child->>Child: Skip cookie injection

    Note over UI,Auth: AFTER FIX: auth_id persisted to database

    UI->>Executor: Execute(jobDef with auth_id)
    Executor->>Executor: Create parentMetadata map
    Executor->>Executor: Add auth_id to parentMetadata
    Executor->>Manager: UpdateJobMetadata(parentJobID, parentMetadata)
    Manager->>DB: UPDATE jobs SET metadata_json = ?
    DB-->>Manager: ✓ Metadata saved
    Note over Executor: ✓ Metadata persisted!
    Executor->>Executor: StartMonitoring
    
    Note over Child,Auth: Child job executes later
    Child->>Manager: GetJob(parentJobID)
    Manager->>DB: SELECT metadata_json
    DB-->>Manager: {"auth_id": "abc123", ...}
    Manager-->>Child: Job with metadata containing auth_id
    Child->>Child: Check metadata["auth_id"]
    Note over Child: ✓ auth_id FOUND!
    Child->>Auth: GetCredentialsByID(auth_id)
    Auth-->>Child: Cookies and tokens
    Child->>Child: Inject cookies into ChromeDP
    Note over Child: ✓ Authenticated crawling works!

## Proposed File Changes

### internal\jobs\manager.go(MODIFY)

**Add UpdateJobMetadata method after UpdateJobConfig (after line 683):**

Create a new method `UpdateJobMetadata` that follows the exact same pattern as `UpdateJobConfig` (lines 672-683):

- Method signature: `func (m *Manager) UpdateJobMetadata(ctx context.Context, jobID string, metadata map[string]interface{}) error`
- Marshal the metadata map to JSON using `json.Marshal`
- Return error if marshaling fails with context: `"marshal metadata: %w"`
- Execute SQL UPDATE statement: `UPDATE jobs SET metadata_json = ? WHERE id = ?`
- Pass marshaled JSON string and jobID as parameters
- Return any database execution error

**Implementation Notes:**
- Place this method immediately after `UpdateJobConfig` (line 683) for logical grouping
- Use identical error handling pattern as `UpdateJobConfig`
- No retry logic needed (unlike CreateJobRecord) - updates are idempotent
- This method will be called once per parent job creation to persist auth_id

**Why This Location:**
- Groups related update methods together (UpdateJobStatus, UpdateJobProgress, UpdateJobConfig, UpdateJobMetadata)
- Maintains consistency with existing code patterns
- Easy to find for future maintenance

### internal\jobs\executor\job_executor.go(MODIFY)

References: 

- internal\jobs\manager.go(MODIFY)
- internal\jobs\processor\enhanced_crawler_executor_auth.go

**Call UpdateJobMetadata to persist auth_id (after line 324, before line 326):**

Immediately after creating `parentMetadata` map (lines 315-324) and BEFORE creating `parentJobModel` (line 326), add a call to persist metadata to the database:

- Call `e.jobManager.UpdateJobMetadata(ctx, parentJobID, parentMetadata)`
- Check for error and log warning if update fails (non-fatal - job can continue)
- Use existing logger pattern: `parentLogger.Warn().Err(err).Str("parent_job_id", parentJobID).Msg("Failed to update job metadata, auth may not work for child jobs")`
- Add debug log on success: `parentLogger.Debug().Str("parent_job_id", parentJobID).Int("metadata_keys", len(parentMetadata)).Msg("Job metadata persisted to database")`

**Critical Timing:**
- MUST be called AFTER parentMetadata is fully populated (line 324)
- MUST be called BEFORE StartMonitoring (line 338) so database has metadata when child jobs execute
- Place between lines 324 and 326 for optimal flow

**Error Handling Strategy:**
- Log warning but DO NOT return error - this is non-fatal
- Job execution should continue even if metadata update fails
- Child jobs will fall back to job_definition_id lookup (lines 99-126 in enhanced_crawler_executor_auth.go)
- This maintains backward compatibility and graceful degradation

**Why Non-Fatal:**
- Parent job monitoring should not fail due to metadata persistence issues
- Existing fallback mechanism (job_definition_id) provides redundancy
- Allows debugging of metadata issues without breaking job execution

**Verification:**
- After this change, the diagnostic logs at lines 73-80 in `enhanced_crawler_executor_auth.go` will show auth_id in metadata_keys
- Child jobs will successfully inject cookies using auth_id from parent metadata
- No more "auth_id NOT found in job metadata" warnings