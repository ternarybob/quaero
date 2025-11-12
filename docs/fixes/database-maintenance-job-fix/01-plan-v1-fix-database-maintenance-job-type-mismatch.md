I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Root Cause Identified:**

The Database Maintenance job fails because `DatabaseMaintenanceManager` creates parent jobs with type `"database_maintenance_parent"` (lines 73 and 154), but `JobMonitor.validate()` expects parent jobs to have type `models.JobTypeParent` (which equals `"parent"`). This validation mismatch causes the monitor to fail with error: `"invalid job type: expected parent, got database_maintenance_parent"`.

**Evidence from Logs:**
- All three child jobs (VACUUM, ANALYZE, REINDEX) complete successfully
- Parent job is marked as "failed" due to monitor validation failure
- Error message: `"Invalid parent job model - cannot start monitoring"`

**Pattern Analysis:**
- `CrawlerManager` uses `models.JobTypeParent` indirectly via `CrawlerService.StartCrawl()`
- `AgentManager` doesn't create parent job records (returns passed-in parentJobID)
- `DatabaseMaintenanceManager` is the **only manager** using a custom parent type string

**Architecture Compliance:**
According to `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md`, all managers should follow the standard Manager/Worker/Monitor pattern with consistent job type conventions.

### Approach

Replace hardcoded `"database_maintenance_parent"` string with `models.JobTypeParent` constant in `DatabaseMaintenanceManager` to match the validation expectations of `JobMonitor`. This ensures parent jobs are properly monitored and marked as completed instead of failed.

### Reasoning

Analyzed the log file showing validation error, examined `DatabaseMaintenanceManager`, `JobMonitor`, and `crawler_job.go` to understand the type mismatch. Compared with `CrawlerManager` and `AgentManager` to confirm the correct pattern. Identified that `models.JobTypeParent` constant (value: `"parent"`) is the standard for all parent jobs.

## Mermaid Diagram

sequenceDiagram
    participant UI as User Interface
    participant Orch as JobDefinitionOrchestrator
    participant DBMgr as DatabaseMaintenanceManager
    participant Monitor as JobMonitor
    participant Queue as QueueManager
    participant Worker as DatabaseMaintenanceWorker

    UI->>Orch: Trigger "Database Maintenance" job
    Orch->>DBMgr: CreateParentJob(step, jobDef, parentJobID)
    
    Note over DBMgr: ✅ FIX: Use models.JobTypeParent<br/>instead of "database_maintenance_parent"
    
    DBMgr->>DBMgr: Create parent job record<br/>Type: string(models.JobTypeParent)
    DBMgr->>Queue: Enqueue child jobs<br/>(VACUUM, ANALYZE, REINDEX)
    DBMgr->>Monitor: StartMonitoring(parentJobModel)
    
    Note over Monitor: ✅ Validation passes:<br/>job.Type == models.JobTypeParent
    
    Monitor->>Monitor: Start monitoring goroutine
    
    loop For each child job
        Queue->>Worker: Receive child job
        Worker->>Worker: Execute operation<br/>(VACUUM/ANALYZE/REINDEX)
        Worker->>Monitor: Publish EventJobStatusChange
        Monitor->>Monitor: Update parent job progress
    end
    
    Monitor->>Monitor: All children completed
    Monitor->>UI: Parent job status: "completed" ✅

## Proposed File Changes

### internal\jobs\manager\database_maintenance_manager.go(MODIFY)

References: 

- internal\models\crawler_job.go
- internal\jobs\monitor\job_monitor.go

**Line 73 - Fix Parent Job Type in CreateParentJob():**

Replace the hardcoded string `"database_maintenance_parent"` with `string(models.JobTypeParent)` to match the validation expectations in `JobMonitor.validate()`.

**Before:**
```
Type:     "database_maintenance_parent",
```

**After:**
```
Type:     string(models.JobTypeParent),
```

**Line 154 - Fix Parent Job Type in JobModel Creation:**

Replace the hardcoded string `"database_maintenance_parent"` with `string(models.JobTypeParent)` to ensure consistency when passing the job model to `JobMonitor.StartMonitoring()`.

**Before:**
```
Type:     "database_maintenance_parent",
```

**After:**
```
Type:     string(models.JobTypeParent),
```

**Rationale:**
- `models.JobTypeParent` is defined in `internal/models/crawler_job.go` (line 27) as `JobType = "parent"`
- `JobMonitor.validate()` in `internal/jobs/monitor/job_monitor.go` (line 80) explicitly checks: `if job.Type != string(models.JobTypeParent)`
- Using the constant ensures type safety and consistency across all managers
- Follows the established pattern used by other managers in the codebase

**Impact:**
- Parent job will pass validation and monitoring will start successfully
- Job status will correctly transition from "running" → "completed" instead of "failed"
- Child job statistics will be properly aggregated and displayed in UI
- No changes needed to child job types (they correctly use `"database_maintenance_operation"`)