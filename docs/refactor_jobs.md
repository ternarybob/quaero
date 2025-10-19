I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Key Observations

1. **Existing Infrastructure:**
   - \`JobStorage\` has \`GetJobsByStatus()\` and \`UpdateJobStatus()\` methods
   - Crawler service tracks \`activeJobs\` in memory with \`jobsMu\` mutex protection
   - \`CancelJob()\` method exists but only updates in-memory state and DB
   - \`monitorCompletion()\` goroutine runs every 2 seconds per job
   - Scheduler has \`Start()\` and \`Stop()\` but no cleanup logic
   - App initialization in \`app.go\` follows strict order: storage → services → handlers

2. **Missing Components:**
   - No \`last_heartbeat\` column in \`crawl_jobs\` table
   - No bulk status update method for orphaned jobs
   - No method to get all active job IDs from crawler service
   - No heartbeat update mechanism during job execution
   - No stale job detection in scheduler
   - Scheduler \`Stop()\` doesn't cancel running crawler jobs

3. **Integration Points:**
   - Scheduler needs reference to \`CrawlerService\` for job cancellation
   - Scheduler needs reference to \`JobStorage\` for orphaned job cleanup
   - Heartbeat updates should happen in \`monitorCompletion()\` goroutine (already runs every 2s)
   - Stale job detection should run as separate goroutine in scheduler
   - App shutdown sequence: scheduler → crawler → event service → LLM → storage

4. **Design Decisions:**
   - Use existing \`GetJobsByStatus()\` for querying running jobs
   - Add new \`UpdateJobHeartbeat()\` method to JobStorage interface
   - Add new \`GetActiveJobIDs()\` method to crawler service for shutdown
   - Heartbeat interval: 30 seconds (update every 15 ticks of monitorCompletion)
   - Stale job threshold: 10 minutes without heartbeat
   - Cancellation timeout: 30 seconds during shutdown

### Approach

## Implementation Approach

**Phase 1: Database Schema Migration**
Add \`last_heartbeat\` column to \`crawl_jobs\` table via migration in \`schema.go\`. This enables tracking when jobs last reported activity.

**Phase 2: Storage Layer Enhancement**
Add \`UpdateJobHeartbeat()\` method to \`JobStorage\` interface and SQLite implementation. This provides efficient heartbeat updates without full job saves.

**Phase 3: Crawler Service Heartbeat**
Modify \`monitorCompletion()\` to update heartbeat every 30 seconds (every 15 ticks). Add \`GetActiveJobIDs()\` method for shutdown coordination.

**Phase 4: Scheduler Startup Cleanup**
Add \`CleanupOrphanedJobs()\` method to scheduler service that queries running jobs and marks them as failed. Call during app initialization before scheduler starts.

**Phase 5: Scheduler Stale Job Detection**
Add \`DetectStaleJobs()\` method and background goroutine to scheduler. Runs every 5 minutes, marks jobs with 10+ minute stale heartbeats as failed.

**Phase 6: Graceful Shutdown**
Enhance scheduler \`Stop()\` to cancel active crawler jobs with timeout. Update \`app.Close()\` to ensure proper shutdown order.

### Reasoning

Explored the codebase structure by reading scheduler service, app initialization, crawler service, job storage, and schema files. Searched for existing methods like \`GetJobsByStatus\`, \`UpdateJobStatus\`, and \`CancelJob\`. Examined the \`monitorCompletion\` goroutine lifecycle and \`activeJobs\` tracking. Reviewed app shutdown sequence in \`Close()\` method. Identified integration points between scheduler, crawler, and storage layers.

## Mermaid Diagram

sequenceDiagram
    participant App as App Initialization
    participant Scheduler as Scheduler Service
    participant Crawler as Crawler Service
    participant JobStorage as Job Storage
    participant DB as SQLite Database

    Note over App,DB: STARTUP SEQUENCE
    App->>JobStorage: Initialize storage
    App->>Crawler: Initialize crawler service
    App->>Scheduler: NewServiceWithDB(crawlerService)
    App->>Scheduler: CleanupOrphanedJobs()
    Scheduler->>JobStorage: GetJobsByStatus(\"running\")
    JobStorage->>DB: SELECT * WHERE status='running'
    DB-->>JobStorage: Orphaned jobs
    loop For each orphaned job
        Scheduler->>JobStorage: UpdateJobStatus(jobID, \"failed\", \"Service restarted\")
        JobStorage->>DB: UPDATE status, error, completed_at
    end
    Scheduler-->>App: Cleanup complete
    App->>Scheduler: Start()
    Scheduler->>Scheduler: Launch stale job detector goroutine

    Note over App,DB: RUNTIME - HEARTBEAT MONITORING
    Crawler->>Crawler: monitorCompletion() every 2s
    loop Every 30 seconds (15 ticks)
        Crawler->>JobStorage: UpdateJobHeartbeat(jobID)
        JobStorage->>DB: UPDATE last_heartbeat = NOW()
    end

    loop Every 5 minutes
        Scheduler->>JobStorage: GetStaleJobs(10 minutes)
        JobStorage->>DB: SELECT WHERE last_heartbeat < NOW()-10min
        DB-->>JobStorage: Stale jobs
        loop For each stale job
            Scheduler->>JobStorage: UpdateJobStatus(jobID, \"failed\", \"Job stale\")
        end
    end

    Note over App,DB: SHUTDOWN SEQUENCE
    App->>Scheduler: Stop()
    Scheduler->>Crawler: GetActiveJobIDs()
    Crawler-->>Scheduler: [jobID1, jobID2, ...]
    loop For each active job (30s timeout)
        Scheduler->>Crawler: CancelJob(jobID)
        Crawler->>JobStorage: SaveJob(status=cancelled)
        JobStorage->>DB: UPDATE status='cancelled'
    end
    Scheduler->>Scheduler: Stop stale job ticker
    Scheduler->>Scheduler: Stop cron scheduler
    App->>Crawler: Close()
    App->>JobStorage: Close()

## Proposed File Changes

### internal\\storage\\sqlite\\schema.go(MODIFY)

Add migration method \`migrateAddHeartbeatColumn()\` to add \`last_heartbeat INTEGER\` column to \`crawl_jobs\` table. Call this migration from \`runMigrations()\` after existing migrations. Use \`PRAGMA table_info(crawl_jobs)\` to check if column exists before adding. Set default value to current timestamp for existing rows using \`UPDATE crawl_jobs SET last_heartbeat = created_at WHERE last_heartbeat IS NULL\`.

### internal\\interfaces\\storage.go(MODIFY)

References: 

- internal\\storage\\sqlite\\job_storage.go(MODIFY)

Add new method signature to \`JobStorage\` interface: \`UpdateJobHeartbeat(ctx context.Context, jobID string) error\`. This method will update only the \`last_heartbeat\` column for efficient heartbeat tracking without full job serialization.

### internal\\storage\\sqlite\\job_storage.go(MODIFY)

References: 

- internal\\interfaces\\storage.go(MODIFY)

Implement \`UpdateJobHeartbeat()\` method that executes SQL: \`UPDATE crawl_jobs SET last_heartbeat = ? WHERE id = ?\` with current Unix timestamp. Use mutex lock for thread safety. Add method \`GetStaleJobs(ctx context.Context, staleThresholdMinutes int) ([]interface{}, error)\` that queries jobs with \`status='running' AND last_heartbeat < (current_time - threshold)\`. Return results using existing \`scanJobs()\` helper.

### internal\\services\\crawler\\service.go(MODIFY)

References: 

- internal\\storage\\sqlite\\job_storage.go(MODIFY)
- internal\\interfaces\\storage.go(MODIFY)

In \`monitorCompletion()\` goroutine (line 739), add heartbeat counter that increments each tick (2s interval). When counter reaches 15 (30 seconds), call \`s.jobStorage.UpdateJobHeartbeat(s.ctx, jobID)\` and reset counter. Add error logging if heartbeat update fails but continue execution. Add new method \`GetActiveJobIDs() []string\` that acquires \`jobsMu.RLock()\`, iterates \`activeJobs\` map, collects job IDs, releases lock, and returns slice. This enables scheduler to enumerate running jobs during shutdown.

### internal\\interfaces\\scheduler_service.go(MODIFY)

References: 

- internal\\services\\scheduler\\scheduler_service.go(MODIFY)

Add new method signature: \`CleanupOrphanedJobs() error\`. This will be called during app initialization to mark orphaned running jobs as failed after service restart.

### internal\\services\\scheduler\\scheduler_service.go(MODIFY)

References: 

- internal\\services\\crawler\\service.go(MODIFY)
- internal\\storage\\sqlite\\job_storage.go(MODIFY)
- internal\\interfaces\\storage.go(MODIFY)
- internal\\interfaces\\scheduler_service.go(MODIFY)

Add fields to \`Service\` struct: \`crawlerService *crawler.Service\` (for shutdown coordination) and \`staleJobTicker *time.Ticker\` (for cleanup goroutine). Update \`NewServiceWithDB()\` constructor to accept \`crawlerService\` parameter and store it. Implement \`CleanupOrphanedJobs()\` method: query \`jobStorage.GetJobsByStatus(ctx, \"running\")\`, iterate results, call \`jobStorage.UpdateJobStatus(ctx, jobID, \"failed\", \"Service restarted\")\` for each, log count of cleaned jobs. Implement \`DetectStaleJobs()\` method: call \`jobStorage.GetStaleJobs(ctx, 10)\`, iterate results, update status to failed with error \"Job stale (no heartbeat)\", emit progress events. In \`Start()\` method, launch goroutine that runs \`DetectStaleJobs()\` every 5 minutes using \`time.NewTicker(5 * time.Minute)\`, store ticker in \`staleJobTicker\` field. In \`Stop()\` method, before stopping cron: get active job IDs from \`crawlerService.GetActiveJobIDs()\`, iterate and call \`crawlerService.CancelJob(jobID)\` for each, wait up to 30 seconds for cancellation using \`time.After()\` and status polling, stop \`staleJobTicker\` if not nil, log cancellation results.

### internal\\app\\app.go(MODIFY)

References: 

- internal\\services\\scheduler\\scheduler_service.go(MODIFY)
- internal\\services\\crawler\\service.go(MODIFY)

In \`initServices()\` method (line 158), update scheduler initialization at line 277: change \`scheduler.NewServiceWithDB(a.EventService, a.Logger, a.StorageManager.DB().(*sql.DB))\` to \`scheduler.NewServiceWithDB(a.EventService, a.Logger, a.StorageManager.DB().(*sql.DB), a.CrawlerService)\` to pass crawler service reference. After scheduler initialization but before \`Start()\` call (around line 353), add: \`if err := a.SchedulerService.CleanupOrphanedJobs(); err != nil { a.Logger.Warn().Err(err).Msg(\"Failed to cleanup orphaned jobs\") }\`. This ensures orphaned jobs are marked failed before scheduler begins. In \`Close()\` method (line 423), verify shutdown order is: scheduler (line 426) → crawler (line 433) → event service (line 440) → LLM (line 447) → storage (line 454). This order ensures scheduler can cancel crawler jobs before crawler shuts down.