# Plan: Fix Database Maintenance Job Type Mismatch

## Problem Summary
Database Maintenance job fails with validation error because `DatabaseMaintenanceManager` creates parent jobs with type `"database_maintenance_parent"`, but `JobMonitor.validate()` expects parent jobs to have type `models.JobTypeParent` (which equals `"parent"`).

## Root Cause
- All three child jobs (VACUUM, ANALYZE, REINDEX) complete successfully
- Parent job is marked as "failed" due to monitor validation failure
- Error: `"invalid job type: expected parent, got database_maintenance_parent"`
- `DatabaseMaintenanceManager` is the only manager using a custom parent type string

## Steps

### Step 1: Fix Parent Job Type Constants
- Skill: @go-coder
- Files: `internal/jobs/manager/database_maintenance_manager.go`
- User decision: no
- Replace hardcoded `"database_maintenance_parent"` strings with `string(models.JobTypeParent)` constant at:
  - Line 73: Parent job record creation
  - Line 154: JobModel creation for monitoring

### Step 2: Verify Compilation and Type Safety
- Skill: @go-coder
- Files: `internal/jobs/manager/database_maintenance_manager.go`
- User decision: no
- Compile the modified file to ensure no type errors
- Verify that `models.JobTypeParent` is properly imported and available

## Success Criteria
- Code compiles without errors
- Parent job type matches `models.JobTypeParent` constant
- Follows the established pattern used by other managers (CrawlerManager)
- Job monitor validation will pass, allowing proper monitoring
- Parent job status will correctly transition from "running" to "completed"
