I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**Critical Finding: Foreign Keys Are DISABLED**
- Line 80 in `connection.go`: `PRAGMA foreign_keys = OFF`
- Comment states: "Disabled - this is a temporary document cache, not a relational database"
- This means even if we add FK constraints, they won't be enforced unless we enable foreign keys

**Existing Infrastructure:**
- `JobStorage.GetChildJobs()` already exists - can query children by parent_id
- `JobStorage.DeleteJob()` is a simple DELETE statement with no cascade logic
- `job_logs` and `job_seen_urls` tables already have `ON DELETE CASCADE` constraints (but not enforced due to disabled FKs)
- Migration pattern established - use `ALTER TABLE` for simple columns, table recreation for complex changes

**Parent/Child Hierarchy:**
- Two-level hierarchy: Job Definition Parent → Crawler Parent → Crawler Children
- Parent jobs have empty `parent_id`, children reference parent via `parent_id`
- No evidence of deeper nesting (grandchildren)

**Deletion Paths:**
1. UI: User clicks delete → `DeleteJobHandler` → `JobManager.DeleteJob`
2. Cleanup Job: `CleanupJob.Execute` → `JobStorage.DeleteJob` (bypasses JobManager!)
3. Tests: Mock implementations also call `DeleteJob`

**Risk Assessment:**
- **Low Risk**: Application-level cascade is safe - uses existing GetChildJobs method
- **Medium Risk**: Enabling foreign keys globally could break other parts of the system
- **High Risk**: Orphaned jobs may already exist in production databases

### Approach

## Solution Strategy

**Hybrid Approach: Application-Level Cascade + Database Foreign Key Constraint**

We'll implement both layers of protection for maximum robustness:

1. **Application-Level Cascade Deletion** in `JobManager.DeleteJob()` - Provides immediate fix, works with existing databases, includes audit logging
2. **Database-Level Foreign Key Constraint** via migration - Ensures data integrity at the database level, prevents orphaned records even if application logic is bypassed

**Why Hybrid?**
- Application layer provides explicit logging and control over deletion order
- Database layer provides fail-safe protection and automatic cleanup
- Both layers complement each other without conflict
- Graceful handling of existing orphaned data during migration

**Key Design Decisions:**
- Use recursive deletion in application layer to handle multi-level hierarchies
- Add migration to enable foreign keys and add constraint
- Clean up existing orphaned jobs during migration
- Maintain transaction safety for atomic deletions
- Log cascade deletions for audit trail

### Reasoning

I explored the codebase by reading:
1. `internal/jobs/manager.go` - Current DeleteJob implementation (no cascade logic)
2. `internal/storage/sqlite/connection.go` - Found foreign keys are DISABLED (line 80)
3. `internal/storage/sqlite/schema.go` - Confirmed no FK constraint on parent_id, but job_logs and job_seen_urls have proper CASCADE
4. `internal/storage/sqlite/job_storage.go` - Found GetChildJobs method already exists for querying children
5. `internal/handlers/job_handler.go` - Verified DeleteJobHandler calls JobManager.DeleteJob
6. `internal/jobs/types/cleanup.go` - Confirmed cleanup job uses JobStorage.DeleteJob directly
7. Migration patterns - Studied existing migrations to understand the schema evolution approach

## Mermaid Diagram

sequenceDiagram
    participant UI as Queue UI
    participant Handler as JobHandler
    participant Manager as JobManager
    participant Storage as JobStorage
    participant DB as SQLite Database
    
    Note over UI,DB: User Deletes Parent Job with Children
    
    UI->>Handler: DELETE /api/jobs/{parent_id}
    Handler->>Manager: DeleteJob(ctx, parent_id)
    
    Note over Manager: Application-Level Cascade
    Manager->>Storage: GetChildJobs(ctx, parent_id)
    Storage->>DB: SELECT * FROM crawl_jobs WHERE parent_id = ?
    DB-->>Storage: [child1, child2, child3]
    Storage-->>Manager: 3 child jobs
    
    Manager->>Manager: Log: "Cascading delete to 3 child jobs"
    
    loop For Each Child
        Manager->>Manager: DeleteJob(ctx, child_id) [Recursive]
        Manager->>Storage: GetChildJobs(ctx, child_id)
        Storage-->>Manager: [] (no grandchildren)
        Manager->>Storage: DeleteJob(ctx, child_id)
        Storage->>DB: DELETE FROM crawl_jobs WHERE id = child_id
        
        Note over DB: Database-Level Cascade
        DB->>DB: CASCADE: Delete from job_logs WHERE job_id = child_id
        DB->>DB: CASCADE: Delete from job_seen_urls WHERE job_id = child_id
        
        DB-->>Storage: Child deleted
        Storage-->>Manager: Success
        Manager->>Manager: Log: "Child job deleted"
    end
    
    Manager->>Manager: Log: "All children deleted, deleting parent"
    Manager->>Storage: DeleteJob(ctx, parent_id)
    Storage->>DB: DELETE FROM crawl_jobs WHERE id = parent_id
    
    Note over DB: Database-Level Cascade
    DB->>DB: CASCADE: Delete from job_logs WHERE job_id = parent_id
    DB->>DB: CASCADE: Delete from job_seen_urls WHERE job_id = parent_id
    
    DB-->>Storage: Parent deleted
    Storage-->>Manager: Success
    Manager-->>Handler: Success
    Handler-->>UI: 200 OK {"message": "Job deleted successfully"}
    
    Note over UI,DB: Alternative: Database-Only Cascade (if app logic bypassed)
    
    Storage->>DB: DELETE FROM crawl_jobs WHERE id = parent_id
    DB->>DB: FK Constraint: ON DELETE CASCADE
    DB->>DB: Auto-delete children WHERE parent_id = parent_id
    DB->>DB: Auto-delete job_logs for all deleted jobs
    DB->>DB: Auto-delete job_seen_urls for all deleted jobs
    DB-->>Storage: All deleted automatically

## Proposed File Changes

### internal\jobs\manager.go(MODIFY)

References: 

- internal\storage\sqlite\job_storage.go
- internal\interfaces\storage.go

## Update DeleteJob Method to Cascade Delete Children

**Current Implementation (Lines 136-173):**
- Gets job and checks status
- Cancels if running
- Deletes job from storage
- Deletes job logs
- No child handling

**New Implementation:**

### Step 1: Check for Child Jobs (Before Line 149)
After getting the job and before canceling, query for child jobs using `JobStorage.GetChildJobs(ctx, jobID)`.

If children exist, recursively delete them first by calling `DeleteJob` for each child job ID. This ensures:
- Grandchildren are deleted before children
- Each deletion is logged individually
- Logs are cleaned up for each job
- Atomic deletion per job

### Step 2: Add Logging for Cascade Deletion
Log the cascade deletion operation:
- Before starting: Log parent job ID and child count
- During deletion: Log each child job ID being deleted
- After completion: Log total children deleted
- On error: Log which child failed and continue with others (collect errors)

### Step 3: Error Handling Strategy
If child deletion fails:
- Log the error with child job ID
- Continue deleting other children (don't fail fast)
- Collect all errors
- After all children processed, if any errors occurred, return aggregated error
- Still attempt to delete parent job (partial cleanup better than none)

### Step 4: Update Method Signature (Optional)
Consider adding a `cascade` boolean parameter to control behavior:
- `DeleteJob(ctx, jobID string, cascade bool)`
- Default to `true` for safety
- Allows explicit non-cascade deletion if needed in future

**Implementation Notes:**
- Use `m.jobStorage.GetChildJobs(ctx, jobID)` to query children
- Recursive call: `m.DeleteJob(ctx, childID)` for each child
- Maximum recursion depth check (prevent infinite loops): limit to 10 levels
- Transaction consideration: Each job deletion is atomic, but cascade is not transactional across jobs

**Example Logic Flow:**
```
1. Get job by ID
2. Query children using GetChildJobs
3. If children exist:
   a. Log cascade operation start
   b. For each child:
      - Recursively call DeleteJob(child.ID)
      - Collect any errors
   c. Log cascade operation complete
4. Cancel parent if running
5. Delete parent job from storage
6. Delete parent job logs
7. Return aggregated errors if any
```

**Logging Examples:**
- `logger.Info().Str("parent_id", jobID).Int("child_count", len(children)).Msg("Cascading delete to child jobs")`
- `logger.Debug().Str("parent_id", jobID).Str("child_id", childID).Msg("Deleting child job")`
- `logger.Warn().Err(err).Str("parent_id", jobID).Str("child_id", childID).Msg("Failed to delete child job, continuing")`
- `logger.Info().Str("job_id", jobID).Int("children_deleted", successCount).Int("children_failed", errorCount).Msg("Cascade deletion completed")`

### internal\storage\sqlite\schema.go(MODIFY)

References: 

- internal\storage\sqlite\connection.go(MODIFY)

## Add Migration 16: Enable Foreign Keys and Add Parent ID Constraint

**Location:** Add after Migration 15 in `runMigrations()` method (after line 360)

### Migration Method: `migrateEnableForeignKeysAndAddParentConstraint()`

**Purpose:** Enable foreign key enforcement and add CASCADE constraint to parent_id

**Implementation Steps:**

### Step 1: Check if Migration Already Applied
Query `PRAGMA foreign_key_list(crawl_jobs)` to check if parent_id constraint exists.
If constraint exists, return early (migration already applied).

### Step 2: Clean Up Orphaned Jobs
Before enabling constraints, find and delete orphaned child jobs:
```sql
DELETE FROM crawl_jobs
WHERE parent_id IS NOT NULL
  AND parent_id != ''
  AND parent_id NOT IN (SELECT id FROM crawl_jobs WHERE parent_id IS NULL OR parent_id = '')
```
Log the count of orphaned jobs deleted.

### Step 3: Enable Foreign Keys Globally
Update `connection.go` line 80 from:
```go
"PRAGMA foreign_keys = OFF", // Disabled - this is a temporary document cache
```
To:
```go
"PRAGMA foreign_keys = ON", // Enabled for referential integrity
```

**Important:** This enables FK enforcement for ALL tables, including existing constraints on `job_logs` and `job_seen_urls`.

### Step 4: Recreate crawl_jobs Table with Foreign Key Constraint

SQLite doesn't support `ALTER TABLE ADD CONSTRAINT`, so we must recreate the table:

**Sub-steps:**
a. Create new table `crawl_jobs_new` with identical schema PLUS foreign key:
```sql
CREATE TABLE crawl_jobs_new (
  id TEXT PRIMARY KEY,
  parent_id TEXT,
  name TEXT DEFAULT '',
  description TEXT DEFAULT '',
  source_type TEXT NOT NULL,
  entity_type TEXT NOT NULL,
  config_json TEXT NOT NULL,
  source_config_snapshot TEXT,
  auth_snapshot TEXT,
  refresh_source INTEGER DEFAULT 0,
  seed_urls TEXT,
  status TEXT NOT NULL,
  progress_json TEXT,
  created_at INTEGER NOT NULL,
  started_at INTEGER,
  completed_at INTEGER,
  last_heartbeat INTEGER,
  error TEXT,
  result_count INTEGER DEFAULT 0,
  failed_count INTEGER DEFAULT 0,
  FOREIGN KEY (parent_id) REFERENCES crawl_jobs_new(id) ON DELETE CASCADE
)
```

b. Copy all data from old table to new table:
```sql
INSERT INTO crawl_jobs_new
SELECT * FROM crawl_jobs
```

c. Drop old table:
```sql
DROP TABLE crawl_jobs
```

d. Rename new table:
```sql
ALTER TABLE crawl_jobs_new RENAME TO crawl_jobs
```

e. Recreate all indexes:
```sql
CREATE INDEX idx_jobs_status ON crawl_jobs(status, created_at DESC);
CREATE INDEX idx_jobs_source ON crawl_jobs(source_type, entity_type, created_at DESC);
CREATE INDEX idx_jobs_created ON crawl_jobs(created_at DESC);
CREATE INDEX idx_jobs_parent_id ON crawl_jobs(parent_id, created_at DESC);
```

### Step 5: Verify Foreign Key Constraint
Query `PRAGMA foreign_key_list(crawl_jobs)` to confirm constraint was added.
Log success message with constraint details.

**Error Handling:**
- Wrap entire migration in transaction (BEGIN/COMMIT/ROLLBACK)
- If any step fails, rollback and return error
- Log detailed error messages for debugging

**Logging:**
- `logger.Info().Msg("Running migration: Enable foreign keys and add parent_id CASCADE constraint")`
- `logger.Info().Int("orphaned_jobs_deleted", count).Msg("Cleaned up orphaned child jobs")`
- `logger.Info().Msg("Recreating crawl_jobs table with foreign key constraint")`
- `logger.Info().Msg("Migration: Foreign key constraint added successfully")`

**Testing Considerations:**
- Test on database with orphaned jobs
- Test on database with valid parent/child relationships
- Test on fresh database (no existing jobs)
- Verify CASCADE works: delete parent, confirm children deleted automatically

### internal\storage\sqlite\connection.go(MODIFY)

References: 

- internal\storage\sqlite\schema.go(MODIFY)

## Enable Foreign Key Enforcement

**Current Implementation (Line 80):**
```go
"PRAGMA foreign_keys = OFF", // Disabled - this is a temporary document cache, not a relational database
```

**New Implementation:**
```go
"PRAGMA foreign_keys = ON", // Enabled for referential integrity (required for CASCADE constraints)
```

**Rationale:**
The comment stating "this is a temporary document cache" is outdated. The database now stores:
- Job definitions and execution history
- Parent-child job relationships
- Job logs with foreign key references
- URL deduplication with foreign key references

These are relational data that require referential integrity enforcement.

**Impact Analysis:**

### Existing Foreign Key Constraints (Will Now Be Enforced):

1. **job_seen_urls.job_id → crawl_jobs.id (ON DELETE CASCADE)**
   - Already designed for cascade deletion
   - No breaking changes expected
   - Benefit: Automatic cleanup when jobs deleted

2. **job_logs.job_id → crawl_jobs.id (ON DELETE CASCADE)**
   - Already designed for cascade deletion
   - No breaking changes expected
   - Benefit: Automatic cleanup when jobs deleted

3. **sources.auth_id → auth_credentials.id (ON DELETE SET NULL)**
   - Already designed for null handling
   - No breaking changes expected
   - Benefit: Prevents dangling auth references

4. **crawl_jobs.parent_id → crawl_jobs.id (ON DELETE CASCADE)** [NEW]
   - Added by migration
   - Prevents orphaned child jobs
   - Benefit: Automatic cascade deletion

**Testing Requirements:**
- Verify all existing FK constraints still work
- Test cascade deletion for job_logs and job_seen_urls
- Test SET NULL behavior for sources.auth_id
- Test new parent_id cascade constraint

**Rollback Plan:**
If issues arise, can temporarily disable FKs:
```go
"PRAGMA foreign_keys = OFF",
```
But this should only be a temporary measure while fixing the root cause.

**Documentation Update:**
Update the comment to reflect the new purpose:
```go
"PRAGMA foreign_keys = ON", // Enabled for referential integrity (CASCADE constraints for jobs, logs, URLs)
```

### internal\jobs\types\cleanup.go(MODIFY)

References: 

- internal\jobs\manager.go(MODIFY)
- internal\interfaces\queue_service.go(MODIFY)

## Update Cleanup Job to Use JobManager Instead of JobStorage

**Current Implementation (Lines 166-174):**
The cleanup job directly calls `JobStorage.DeleteJob`, which bypasses the cascade deletion logic in `JobManager.DeleteJob`.

**Problem:**
If a parent job is cleaned up, its children won't be cascade deleted because the cleanup job bypasses JobManager.

**Solution:**
Update `CleanupJobDeps` struct and cleanup logic to use `JobManager` instead of `JobStorage`.

### Step 1: Update CleanupJobDeps Struct (Lines 12-16)

**Current:**
```go
type CleanupJobDeps struct {
    JobStorage interfaces.JobStorage
    LogService interfaces.LogService
}
```

**New:**
```go
type CleanupJobDeps struct {
    JobManager interfaces.JobManager  // Changed from JobStorage
    LogService interfaces.LogService
}
```

### Step 2: Update Execute Method (Line 168)

**Current:**
```go
if err := c.deps.JobStorage.DeleteJob(ctx, jobID); err != nil {
```

**New:**
```go
if err := c.deps.JobManager.DeleteJob(ctx, jobID); err != nil {
```

### Step 3: Update Job Listing Logic (Line 113)

**Current:**
```go
jobs, err := c.deps.JobStorage.ListJobs(ctx, opts)
```

**New:**
```go
jobs, err := c.deps.JobManager.ListJobs(ctx, opts)
```

**Rationale:**
- Ensures cascade deletion logic is applied consistently
- Cleanup job will now properly delete parent jobs with children
- Maintains audit logging for cascade deletions
- Aligns with architectural pattern of using JobManager for job operations

**Impact:**
- All cleanup operations will now cascade delete children
- Cleanup logs will include cascade deletion messages
- No functional changes to cleanup criteria or logic

**Testing:**
- Test cleanup of parent jobs with children
- Verify children are deleted along with parent
- Verify cleanup logs include cascade deletion messages

### test\api\job_api_test.go(MODIFY)

References: 

- internal\handlers\job_handler.go
- test\api\job_completion_test.go

## Add Test Cases for Cascade Deletion

**Add New Test Function:** `TestJobCascadeDeletion`

**Test Scenarios:**

### Test 1: Delete Parent Job with Children
**Setup:**
- Create a parent job (no parent_id)
- Create 3 child jobs referencing the parent
- Verify all 4 jobs exist in database

**Action:**
- Call DELETE /api/jobs/{parent_id}

**Assertions:**
- Response status: 200 OK
- Parent job deleted from database
- All 3 child jobs deleted from database
- Job logs deleted for all 4 jobs

### Test 2: Delete Parent Job with Nested Children (Grandchildren)
**Setup:**
- Create parent job A
- Create child job B (parent_id = A)
- Create grandchild job C (parent_id = B)
- Verify all 3 jobs exist

**Action:**
- Call DELETE /api/jobs/{A}

**Assertions:**
- All 3 jobs deleted (A, B, C)
- Cascade deletion logged for each level

### Test 3: Delete Child Job (No Cascade)
**Setup:**
- Create parent job with 2 children

**Action:**
- Call DELETE /api/jobs/{child_id}

**Assertions:**
- Only child job deleted
- Parent and sibling remain

### Test 4: Delete Parent with Running Child
**Setup:**
- Create parent job (status: completed)
- Create child job (status: running)

**Action:**
- Call DELETE /api/jobs/{parent_id}

**Assertions:**
- Parent deleted
- Child cancelled then deleted
- Both jobs removed from database

### Test 5: Partial Cascade Failure
**Setup:**
- Create parent with 3 children
- Mock one child deletion to fail

**Action:**
- Call DELETE /api/jobs/{parent_id}

**Assertions:**
- 2 children deleted successfully
- 1 child deletion failed (logged)
- Parent still deleted (best effort)
- Error response includes details

**Test Helpers:**
- `createTestJobWithChildren(t, parentID, childCount)` - Helper to create job hierarchy
- `verifyJobDeleted(t, jobID)` - Helper to verify job doesn't exist
- `verifyJobExists(t, jobID)` - Helper to verify job exists
- `countChildJobs(t, parentID)` - Helper to count children

**Integration with Existing Tests:**
- Ensure existing job deletion tests still pass
- Update any tests that assume jobs can be deleted independently

### internal\interfaces\queue_service.go(MODIFY)

## Update JobManager Interface Documentation

**Current DeleteJob Method (Line 47):**
```go
DeleteJob(ctx context.Context, jobID string) error
```

**Add Documentation Comment:**
```go
// DeleteJob deletes a job and all its child jobs recursively.
// If the job has children, they are deleted first in a cascade operation.
// Each deletion is logged individually for audit purposes.
// If any child deletion fails, the error is logged but deletion continues.
// The parent job is deleted even if some children fail to delete.
// Returns an aggregated error if any deletions failed.
DeleteJob(ctx context.Context, jobID string) error
```

**Rationale:**
- Clarifies cascade deletion behavior
- Documents error handling strategy
- Helps developers understand the method's behavior
- Aligns with implementation in `manager.go`

**No Breaking Changes:**
- Method signature unchanged
- Behavior is backward compatible (previously would fail on parent with children, now succeeds)
- All existing callers benefit from cascade deletion automatically

### docs\general-refactors\CASCADE_DELETE_IMPLEMENTATION.md(NEW)

## Create Documentation for Cascade Delete Implementation

**File Purpose:** Document the cascade deletion implementation for future reference

**Content Structure:**

### 1. Overview
- Problem statement: Parent jobs couldn't be deleted when they had children
- Solution: Hybrid approach with application-level and database-level cascade
- Benefits: Data integrity, audit logging, automatic cleanup

### 2. Architecture

#### Application-Level Cascade (JobManager)
- Recursive deletion algorithm
- Error handling strategy
- Logging approach
- Maximum recursion depth protection

#### Database-Level Cascade (Foreign Key Constraint)
- Foreign key constraint on parent_id
- ON DELETE CASCADE behavior
- Interaction with other FK constraints
- Why foreign keys were enabled globally

### 3. Implementation Details

#### Files Modified
- `internal/jobs/manager.go` - Cascade deletion logic
- `internal/storage/sqlite/schema.go` - Migration 16
- `internal/storage/sqlite/connection.go` - Enable foreign keys
- `internal/jobs/types/cleanup.go` - Use JobManager
- `internal/interfaces/queue_service.go` - Documentation

#### Migration Process
- Orphaned job cleanup
- Table recreation with FK constraint
- Index recreation
- Verification steps

### 4. Testing

#### Test Scenarios
- Parent with children deletion
- Nested hierarchy deletion (grandchildren)
- Child-only deletion (no cascade)
- Partial failure handling
- Running job cancellation during cascade

#### Test Files
- `test/api/job_api_test.go` - API integration tests
- Manual testing checklist

### 5. Deployment Considerations

#### Pre-Deployment
- Backup database before migration
- Check for orphaned jobs in production
- Estimate migration time based on job count

#### Post-Deployment
- Verify foreign keys enabled: `PRAGMA foreign_keys`
- Verify constraint exists: `PRAGMA foreign_key_list(crawl_jobs)`
- Monitor deletion operations for errors
- Check logs for cascade deletion messages

#### Rollback Plan
- Disable foreign keys if issues arise
- Revert to previous version
- Restore database from backup if needed

### 6. Troubleshooting

#### Common Issues
- **Issue:** Migration fails with "FOREIGN KEY constraint failed"
  - **Cause:** Orphaned jobs still exist
  - **Solution:** Run orphaned job cleanup query manually

- **Issue:** Deletion takes too long
  - **Cause:** Large number of children
  - **Solution:** Increase timeout, consider batch deletion

- **Issue:** Partial cascade failure
  - **Cause:** Child job locked or in use
  - **Solution:** Check logs, retry deletion, cancel running jobs first

### 7. Future Enhancements

#### Potential Improvements
- Transactional cascade deletion (all-or-nothing)
- Soft delete option (mark as deleted instead of removing)
- Cascade deletion progress tracking
- Bulk deletion optimization
- WebSocket events for cascade deletion progress

#### API Enhancements
- Add `?cascade=false` query parameter to disable cascade
- Add `?dry_run=true` to preview what would be deleted
- Return deleted job IDs in response

### 8. References

#### Related Documentation
- SQLite Foreign Key Support: https://www.sqlite.org/foreignkeys.html
- Job Queue Architecture: `docs/architecture.md`
- Migration Pattern: `docs/general-refactors/QUEUE_MANAGER_IMPLEMENTATION_STATUS.md`

#### Related Code
- Job hierarchy: `internal/models/crawler_job.go`
- Job storage: `internal/storage/sqlite/job_storage.go`
- Queue management: `internal/queue/manager.go`