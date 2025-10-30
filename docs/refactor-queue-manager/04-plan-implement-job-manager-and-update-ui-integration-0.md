I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current Architecture

**JobManager** (internal/jobs/manager.go):
- CreateJob, GetJob, ListJobs, UpdateJob, DeleteJob, CopyJob methods
- Returns interface{} types (requires type assertion to *crawler.CrawlJob)
- Flat hierarchy model (no GetJobWithChildren - removed by design)

**JobHandler** (internal/handlers/job_handler.go):
- Uses CrawlerService.ListJobs, GetJobStatus, RerunJob, CancelJob directly
- Uses JobStorage for GetJob, DeleteJob, UpdateJob
- Already has LogService integration (lines 28, 276)
- MaskSensitiveData() called before returning jobs

**JobDefinitionHandler** (internal/handlers/job_definition_handler.go):
- ExecuteJobDefinitionHandler runs JobExecutor.Execute in goroutine (lines 372-379)
- JobExecutor polls crawl jobs asynchronously (executor.go lines 458-655)
- No queue integration yet

**WebSocket** (internal/handlers/websocket.go):
- BroadcastCrawlProgress exists (lines 582-613)
- Subscribes to EventCrawlProgress (lines 655-700)
- CrawlProgressUpdate struct has all needed fields (lines 102-116)

**UI** (pages/queue.html, pages/jobs.html):
- queue.html has job list with filters, pagination, job details (lines 70-202)
- jobs.html manages auth, sources, job definitions (no queue visualization)
- common.js has Alpine.js components (serviceLogs, appStatus, sourceManagement, jobDefinitionsManagement)
- websocket-manager.js handles WebSocket subscriptions

**Queue Infrastructure**:
- QueueManager.GetQueueStats() returns total/pending/in-flight counts (queue/manager.go lines 162-194)
- JobMessage has Type, ParentID, Depth, URL, Config fields (queue/types.go lines 11-53)
- WorkerPool registers handlers for crawler_url, summarizer, cleanup (app.go lines 380-419)

### Approach

## Implementation Strategy

**Core Principle**: Integrate JobManager into handlers, add queue-based job execution for JobDefinitions, enhance WebSocket with queue stats, and update UI to display job progress with flat hierarchy visualization.

**Key Design Decisions**:
1. **Flat Hierarchy Model**: All crawler_url messages point to root job ID (no nested tree)
2. **JobManager Integration**: Replace direct CrawlerService calls in handlers with JobManager
3. **Queue-Based Execution**: JobDefinitions enqueue messages instead of goroutine execution
4. **Real-Time Updates**: WebSocket broadcasts queue stats and job spawning events
5. **UI Enhancement**: Display job progress, queue status, and parent-child relationships (flat view)

### Reasoning

Explored the codebase to understand:
- JobManager exists with CRUD operations but isn't integrated into handlers yet
- JobHandler uses CrawlerService directly for all operations
- JobDefinitionHandler executes via JobExecutor in goroutine (no queue integration)
- WebSocket infrastructure exists with BroadcastCrawlProgress and event subscriptions
- UI has Alpine.js components but no queue stats or job spawning visualization
- Flat hierarchy model chosen (all child jobs point to root job ID via ParentID)
- CrawlJob has Progress struct with TotalURLs, CompletedURLs, PendingURLs, Percentage
- QueueManager has GetQueueStats() returning total/pending/in-flight message counts

## Mermaid Diagram

sequenceDiagram
    participant UI as Web UI (Alpine.js)
    participant Handler as Job Handler
    participant JobMgr as Job Manager
    participant QueueMgr as Queue Manager
    participant WS as WebSocket Handler
    participant Worker as Queue Worker
    participant CrawlerJob as Crawler Job Type
    participant EventSvc as Event Service

    Note over UI,EventSvc: Job Management Flow (CRUD via JobManager)

    UI->>Handler: GET /api/jobs
    Handler->>JobMgr: ListJobs(ctx, opts)
    JobMgr->>JobMgr: Query JobStorage
    JobMgr-->>Handler: []interface{} (jobs)
    Handler->>Handler: Type assert to []*CrawlJob
    Handler->>Handler: MaskSensitiveData()
    Handler-->>UI: JSON response

    UI->>Handler: POST /api/jobs/{id}/copy
    Handler->>JobMgr: CopyJob(ctx, jobID)
    JobMgr->>JobMgr: Get original job
    JobMgr->>JobMgr: Create new job with new ID
    JobMgr->>JobMgr: Save to JobStorage
    JobMgr-->>Handler: newJobID
    Handler-->>UI: 201 Created {newJobID}

    Note over UI,EventSvc: Job Definition Execution (Queue-Based)

    UI->>Handler: POST /api/job-definitions/{id}/execute
    Handler->>Handler: Load JobDefinition
    Handler->>Handler: Create parent JobMessage
    Handler->>QueueMgr: Enqueue(parentMsg)
    QueueMgr->>QueueMgr: Store in goqite table
    Handler-->>UI: 202 Accepted {job_id, status: queued}

    Note over UI,EventSvc: Real-Time Queue Stats Broadcasting

    loop Every 5 seconds
        QueueMgr->>QueueMgr: GetQueueStats()
        QueueMgr-->>WS: stats (total/pending/in-flight)
        WS->>WS: BroadcastQueueStats()
        WS-->>UI: WebSocket: queue_stats
        UI->>UI: Update queue stats display
    end

    Note over UI,EventSvc: Job Spawning with Real-Time Updates

    Worker->>QueueMgr: Receive() message
    QueueMgr-->>Worker: JobMessage (crawler_url)
    Worker->>CrawlerJob: Execute(ctx, msg)
    CrawlerJob->>CrawlerJob: Fetch URL, discover links
    
    loop For each discovered link
        CrawlerJob->>QueueMgr: Enqueue(childMsg, depth+1)
        CrawlerJob->>EventSvc: Publish(EventJobSpawn)
        EventSvc->>WS: Notify subscribers
        WS->>WS: BroadcastJobSpawn()
        WS-->>UI: WebSocket: job_spawn
        UI->>UI: Show spawn notification
    end
    
    CrawlerJob->>JobMgr: UpdateJob(progress)
    CrawlerJob->>EventSvc: Publish(EventCrawlProgress)
    EventSvc->>WS: Notify subscribers
    WS->>WS: BroadcastCrawlProgress()
    WS-->>UI: WebSocket: crawl_progress
    UI->>UI: Update progress bar

    Note over UI,EventSvc: UI Components (Alpine.js)

    UI->>UI: queueStats component
    UI->>UI: - Displays pending/in-flight/total
    UI->>UI: - Connection status indicator
    UI->>UI: jobSpawnNotifications component
    UI->>UI: - Shows recent spawns
    UI->>UI: - Toast notifications
    UI->>UI: Job progress bars
    UI->>UI: - Percentage complete
    UI->>UI: - Total/completed/failed/pending

## Proposed File Changes

### internal\handlers\job_handler.go(MODIFY)

References: 

- internal\jobs\manager.go
- internal\interfaces\queue_service.go
- internal\app\app.go(MODIFY)

**Add JobManager Dependency:**

1. Add `jobManager` field to JobHandler struct (after line 23):
   ```go
   jobManager interfaces.JobManager
   ```

2. Update NewJobHandler constructor (line 34) to accept jobManager parameter:
   - Add parameter: `jobManager interfaces.JobManager`
   - Initialize field: `jobManager: jobManager`

3. Update handler initialization in `internal/app/app.go` (line 562) to pass JobManager:
   ```go
   a.JobHandler = handlers.NewJobHandler(a.CrawlerService, a.StorageManager.JobStorage(), a.SourceService, a.StorageManager.AuthStorage(), a.SchedulerService, a.LogService, a.JobManager, a.Config, a.Logger)
   ```

**Update ListJobsHandler (lines 50-149):**

Replace line 95 `h.crawlerService.ListJobs(ctx, opts)` with:
```go
jobsInterface, err := h.jobManager.ListJobs(ctx, opts)
```

Keep type assertion and masking logic (lines 102-114) unchanged.

**Update GetJobHandler (lines 153-211):**

Replace lines 170-183 (active jobs check + storage fallback) with:
```go
jobInterface, err := h.jobManager.GetJob(ctx, jobID)
if err != nil {
    h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to get job")
    http.Error(w, "Job not found", http.StatusNotFound)
    return
}
```

Keep type assertion and masking (lines 185-210) unchanged.

**Update DeleteJobHandler (lines 441-485):**

Replace line 471 `h.jobStorage.DeleteJob(ctx, jobID)` with:
```go
err = h.jobManager.DeleteJob(ctx, jobID)
```

Keep running job check (lines 458-469) unchanged.

**Add CopyJobHandler (new method):**

Add new handler after DeleteJobHandler:
```go
// CopyJobHandler duplicates a job with a new ID
// POST /api/jobs/{id}/copy
func (h *JobHandler) CopyJobHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Extract job ID from path
    pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
    if len(pathParts) < 3 {
        http.Error(w, "Job ID is required", http.StatusBadRequest)
        return
    }
    jobID := pathParts[2]
    
    // Copy job via JobManager
    newJobID, err := h.jobManager.CopyJob(ctx, jobID)
    if err != nil {
        h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to copy job")
        http.Error(w, "Failed to copy job", http.StatusInternalServerError)
        return
    }
    
    h.logger.Info().Str("original_job_id", jobID).Str("new_job_id", newJobID).Msg("Job copied")
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "original_job_id": jobID,
        "new_job_id":      newJobID,
        "message":         "Job copied successfully",
    })
}
```

**Keep Unchanged:**
- RerunJobHandler (uses CrawlerService.RerunJob for backward compatibility)
- CancelJobHandler (uses CrawlerService.CancelJob for active job cancellation)
- GetJobResultsHandler (uses CrawlerService.GetJobResults for crawl-specific results)
- GetJobLogsHandler (already uses LogService)
- GetJobStatsHandler, GetJobQueueHandler, UpdateJobHandler (use JobStorage directly)

### internal\handlers\job_definition_handler.go(MODIFY)

References: 

- internal\queue\types.go
- internal\interfaces\queue_service.go
- internal\app\app.go(MODIFY)

**Add QueueManager Dependency:**

1. Add `queueManager` field to JobDefinitionHandler struct (after line 22):
   ```go
   queueManager interfaces.QueueManager
   ```

2. Update NewJobDefinitionHandler constructor (line 31) to accept queueManager parameter:
   - Add parameter: `queueManager interfaces.QueueManager`
   - Initialize field: `queueManager: queueManager`
   - Add nil check: `if queueManager == nil { panic("queueManager cannot be nil") }`

3. Update handler initialization in `internal/app/app.go` (line 577) to pass QueueManager:
   ```go
   a.JobDefinitionHandler = handlers.NewJobDefinitionHandler(
       a.StorageManager.JobDefinitionStorage(),
       a.JobExecutor,
       a.SourceService,
       a.JobRegistry,
       a.QueueManager,  // ADD THIS
       a.Logger,
   )
   ```

**Refactor ExecuteJobDefinitionHandler (lines 333-391):**

Replace the goroutine execution (lines 370-379) with queue-based execution:

```go
// Create parent job message for job definition execution
parentMsg := queue.NewParentJobMessage(
    string(jobDef.Type),
    jobDef.ID,
    map[string]interface{}{
        "job_definition_id": jobDef.ID,
        "job_name":          jobDef.Name,
        "sources":           jobDef.Sources,
        "steps":             jobDef.Steps,
        "timeout":           jobDef.Timeout,
    },
)

// Enqueue parent message
if err := h.queueManager.Enqueue(ctx, parentMsg); err != nil {
    h.logger.Error().Err(err).Str("job_def_id", jobDef.ID).Msg("Failed to enqueue job definition")
    WriteError(w, http.StatusInternalServerError, "Failed to start job execution")
    return
}

h.logger.Info().Str("job_def_id", id).Str("message_id", parentMsg.ID).Msg("Job definition enqueued")

response := map[string]interface{}{
    "job_id":     parentMsg.ID,
    "job_name":   jobDef.Name,
    "status":     "queued",
    "message":    "Job execution queued successfully",
}

WriteJSON(w, http.StatusAccepted, response)
```

**Rationale:**
- Replaces goroutine-based execution with queue-based execution
- Parent message contains job definition metadata for worker processing
- Worker pool will route message to appropriate handler based on job type
- Maintains async execution model but with queue persistence and retry capabilities
- JobExecutor remains for backward compatibility with existing job definitions

**Note:** This change requires a new handler in WorkerPool to process parent job messages. The handler should:
1. Extract job definition from message config
2. Call JobExecutor.Execute() with the job definition
3. Handle errors and update job status

### internal\handlers\websocket.go(MODIFY)

References: 

- internal\queue\manager.go
- internal\jobs\types\crawler.go(MODIFY)

**Add Queue Stats Message Type:**

Add new struct after AppStatusUpdate (line 122):
```go
type QueueStatsUpdate struct {
    TotalMessages   int    `json:"total_messages"`
    PendingMessages int    `json:"pending_messages"`
    InFlightMessages int   `json:"in_flight_messages"`
    QueueName       string `json:"queue_name"`
    Concurrency     int    `json:"concurrency"`
    Timestamp       time.Time `json:"timestamp"`
}
```

**Add BroadcastQueueStats Method:**

Add new method after BroadcastAppStatus (line 647):
```go
// BroadcastQueueStats sends queue statistics to all connected clients
func (h *WebSocketHandler) BroadcastQueueStats(stats QueueStatsUpdate) {
    msg := WSMessage{
        Type:    "queue_stats",
        Payload: stats,
    }
    
    data, err := json.Marshal(msg)
    if err != nil {
        h.logger.Error().Err(err).Msg("Failed to marshal queue stats message")
        return
    }
    
    h.mu.RLock()
    clients := make([]*websocket.Conn, 0, len(h.clients))
    mutexes := make([]*sync.Mutex, 0, len(h.clients))
    for conn := range h.clients {
        clients = append(clients, conn)
        mutexes = append(mutexes, h.clientMutex[conn])
    }
    h.mu.RUnlock()
    
    for i, conn := range clients {
        mutex := mutexes[i]
        mutex.Lock()
        err := conn.WriteMessage(websocket.TextMessage, data)
        mutex.Unlock()
        
        if err != nil {
            h.logger.Warn().Err(err).Msg("Failed to send queue stats to client")
        }
    }
}
```

**Add Job Spawning Event Message Type:**

Add new struct after QueueStatsUpdate:
```go
type JobSpawnUpdate struct {
    ParentJobID string `json:"parent_job_id"`
    ChildJobID  string `json:"child_job_id"`
    JobType     string `json:"job_type"`
    URL         string `json:"url,omitempty"`
    Depth       int    `json:"depth"`
    Timestamp   time.Time `json:"timestamp"`
}
```

**Add BroadcastJobSpawn Method:**

Add new method after BroadcastQueueStats:
```go
// BroadcastJobSpawn sends job spawning events to all connected clients
func (h *WebSocketHandler) BroadcastJobSpawn(spawn JobSpawnUpdate) {
    msg := WSMessage{
        Type:    "job_spawn",
        Payload: spawn,
    }
    
    data, err := json.Marshal(msg)
    if err != nil {
        h.logger.Error().Err(err).Msg("Failed to marshal job spawn message")
        return
    }
    
    h.mu.RLock()
    clients := make([]*websocket.Conn, 0, len(h.clients))
    mutexes := make([]*sync.Mutex, 0, len(h.clients))
    for conn := range h.clients {
        clients = append(clients, conn)
        mutexes = append(mutexes, h.clientMutex[conn])
    }
    h.mu.RUnlock()
    
    for i, conn := range clients {
        mutex := mutexes[i]
        mutex.Lock()
        err := conn.WriteMessage(websocket.TextMessage, data)
        mutex.Unlock()
        
        if err != nil {
            h.logger.Warn().Err(err).Msg("Failed to send job spawn to client")
        }
    }
}
```

**Update SubscribeToCrawlerEvents (lines 650-733):**

Keep existing EventCrawlProgress subscription unchanged (lines 655-700).

Keep existing EventStatusChanged subscription unchanged (lines 702-732).

**Note:** Queue stats broadcasting will be triggered by a periodic ticker in the server initialization (internal/app/app.go). Job spawn events will be published by CrawlerJob.Execute when enqueueing child jobs.

### internal\app\app.go(MODIFY)

References: 

- internal\handlers\job_handler.go(MODIFY)
- internal\handlers\job_definition_handler.go(MODIFY)
- internal\handlers\websocket.go(MODIFY)

**Update JobHandler Initialization (line 562):**

Change from:
```go
a.JobHandler = handlers.NewJobHandler(a.CrawlerService, a.StorageManager.JobStorage(), a.SourceService, a.StorageManager.AuthStorage(), a.SchedulerService, a.LogService, a.Config, a.Logger)
```

To:
```go
a.JobHandler = handlers.NewJobHandler(a.CrawlerService, a.StorageManager.JobStorage(), a.SourceService, a.StorageManager.AuthStorage(), a.SchedulerService, a.LogService, a.JobManager, a.Config, a.Logger)
```

Add `a.JobManager` parameter before `a.Config`.

**Update JobDefinitionHandler Initialization (line 577):**

Change from:
```go
a.JobDefinitionHandler = handlers.NewJobDefinitionHandler(
    a.StorageManager.JobDefinitionStorage(),
    a.JobExecutor,
    a.SourceService,
    a.JobRegistry,
    a.Logger,
)
```

To:
```go
a.JobDefinitionHandler = handlers.NewJobDefinitionHandler(
    a.StorageManager.JobDefinitionStorage(),
    a.JobExecutor,
    a.SourceService,
    a.JobRegistry,
    a.QueueManager,
    a.Logger,
)
```

Add `a.QueueManager` parameter before `a.Logger`.

**Add Queue Stats Broadcaster (after line 589):**

Add periodic queue stats broadcasting:
```go
// Start queue stats broadcaster
go func() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            // Get queue stats
            stats, err := a.QueueManager.GetQueueStats(context.Background())
            if err != nil {
                a.Logger.Warn().Err(err).Msg("Failed to get queue stats")
                continue
            }
            
            // Broadcast to WebSocket clients
            update := handlers.QueueStatsUpdate{
                TotalMessages:    getInt(stats, "total_messages"),
                PendingMessages:  getInt(stats, "pending_messages"),
                InFlightMessages: getInt(stats, "in_flight_messages"),
                QueueName:        getString(stats, "queue_name"),
                Concurrency:      getInt(stats, "concurrency"),
                Timestamp:        time.Now(),
            }
            a.WSHandler.BroadcastQueueStats(update)
        case <-a.ctx.Done():
            return
        }
    }
}()
a.Logger.Info().Msg("Queue stats broadcaster started")
```

Add helper functions at the end of the file:
```go
func getInt(m map[string]interface{}, key string) int {
    if val, ok := m[key]; ok {
        switch v := val.(type) {
        case int:
            return v
        case int64:
            return int(v)
        case float64:
            return int(v)
        }
    }
    return 0
}

func getString(m map[string]interface{}, key string) string {
    if val, ok := m[key]; ok {
        if str, ok := val.(string); ok {
            return str
        }
    }
    return ""
}
```

**Note:** The queue stats broadcaster runs in a goroutine and broadcasts stats every 5 seconds to all connected WebSocket clients. This provides real-time queue monitoring in the UI.

### internal\server\routes.go(MODIFY)

References: 

- internal\handlers\job_handler.go(MODIFY)

**Add CopyJob Route:**

Add new route after the existing job routes (around line where job routes are defined):
```go
mux.HandleFunc("/api/jobs/{id}/copy", app.JobHandler.CopyJobHandler)
```

This enables the POST /api/jobs/{id}/copy endpoint for job duplication.

**Verify Existing Routes:**

Ensure these routes exist:
- `POST /api/job-definitions/{id}/execute` - for ExecuteJobDefinitionHandler
- `GET /api/jobs` - for ListJobsHandler
- `GET /api/jobs/{id}` - for GetJobHandler
- `DELETE /api/jobs/{id}` - for DeleteJobHandler
- `POST /api/jobs/{id}/rerun` - for RerunJobHandler
- `POST /api/jobs/{id}/cancel` - for CancelJobHandler

No changes needed if routes already exist.

### pages\queue.html(MODIFY)

References: 

- pages\static\websocket-manager.js
- internal\handlers\websocket.go(MODIFY)

**Add Queue Stats Display (after line 23, before Job Statistics Section):**

Add new section:
```html
<!-- Queue Statistics -->
<section style="margin-top: 1.5rem;" x-data="queueStats()">
    <div class="card">
        <div class="card-header">
            <header class="navbar">
                <section class="navbar-section">
                    <h3>Queue Status</h3>
                </section>
                <section class="navbar-section">
                    <span class="label" :class="connectionStatus ? 'label-success' : 'label-error'" x-text="connectionStatus ? 'Connected' : 'Disconnected'"></span>
                </section>
            </header>
        </div>
        <div class="card-body">
            <div class="columns">
                <div class="column col-3">
                    <div class="tile tile-centered">
                        <div class="tile-content">
                            <div class="tile-title text-bold" x-text="stats.pending_messages">0</div>
                            <div class="tile-subtitle text-gray">Pending Messages</div>
                        </div>
                    </div>
                </div>
                <div class="column col-3">
                    <div class="tile tile-centered">
                        <div class="tile-content">
                            <div class="tile-title text-bold" x-text="stats.in_flight_messages">0</div>
                            <div class="tile-subtitle text-gray">In-Flight Messages</div>
                        </div>
                    </div>
                </div>
                <div class="column col-3">
                    <div class="tile tile-centered">
                        <div class="tile-content">
                            <div class="tile-title text-bold" x-text="stats.total_messages">0</div>
                            <div class="tile-subtitle text-gray">Total Messages</div>
                        </div>
                    </div>
                </div>
                <div class="column col-3">
                    <div class="tile tile-centered">
                        <div class="tile-content">
                            <div class="tile-title text-bold" x-text="stats.concurrency">0</div>
                            <div class="tile-subtitle text-gray">Workers</div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</section>
```

**Add Job Progress Visualization (in Job Detail Section, after line 180):**

Add progress bar inside the job detail card:
```html
<!-- Job Progress Bar -->
<div x-show="selectedJob && selectedJob.progress" style="margin-bottom: 1rem;">
    <div class="progress-container">
        <div class="progress-bar" :style="`width: ${selectedJob.progress?.percentage || 0}%`"></div>
    </div>
    <div class="columns" style="margin-top: 0.5rem;">
        <div class="column col-3">
            <small class="text-gray">Total: <span x-text="selectedJob.progress?.total_urls || 0"></span></small>
        </div>
        <div class="column col-3">
            <small class="text-gray">Completed: <span x-text="selectedJob.progress?.completed_urls || 0"></span></small>
        </div>
        <div class="column col-3">
            <small class="text-gray">Failed: <span x-text="selectedJob.progress?.failed_urls || 0"></span></small>
        </div>
        <div class="column col-3">
            <small class="text-gray">Pending: <span x-text="selectedJob.progress?.pending_urls || 0"></span></small>
        </div>
    </div>
</div>
```

**Add Alpine.js Component for Queue Stats (in script section, after line 287):**

Add new component:
```javascript
function queueStats() {
    return {
        stats: {
            total_messages: 0,
            pending_messages: 0,
            in_flight_messages: 0,
            concurrency: 0,
            queue_name: ''
        },
        connectionStatus: false,
        
        init() {
            this.subscribeToWebSocket();
        },
        
        subscribeToWebSocket() {
            if (typeof WebSocketManager !== 'undefined') {
                // Subscribe to queue stats updates
                WebSocketManager.subscribe('queue_stats', (data) => {
                    this.stats = data;
                });
                
                // Subscribe to connection status
                WebSocketManager.onConnectionChange((isConnected) => {
                    this.connectionStatus = isConnected;
                });
            }
        }
    }
}
```

**Update Job List to Show Parent-Child Relationships (modify renderJobRow function around line 800):**

Add parent job indicator in job name column:
```javascript
// In renderJobRow function, update the name cell
const nameCell = row.insertCell();
if (job.seed_urls && job.seed_urls.length > 0) {
    // This is a parent job (has seed URLs)
    nameCell.innerHTML = `
        <strong>${job.name || job.id}</strong>
        <br>
        <small class="text-gray">
            <i class="fas fa-sitemap"></i> Parent Job
            <span class="label label-secondary" style="margin-left: 0.25rem;">${job.progress?.total_urls || 0} URLs</span>
        </small>
    `;
} else {
    nameCell.innerHTML = `<strong>${job.name || job.id}</strong>`;
}
```

**Add CSS for Progress Bar (in <style> section or external CSS):**

```css
.progress-container {
    width: 100%;
    height: 20px;
    background-color: #f0f0f0;
    border-radius: 4px;
    overflow: hidden;
}

.progress-bar {
    height: 100%;
    background-color: #5755d9;
    transition: width 0.3s ease;
}
```

### pages\jobs.html(MODIFY)

References: 

- pages\static\websocket-manager.js
- internal\handlers\websocket.go(MODIFY)

**Add Queue Status Indicator (after Job Definitions section, before Service Logs):**

Add new section after line 434:
```html
<!-- Queue Status Overview -->
<section style="margin-top: 1.5rem;" x-data="queueStatusOverview()">
    <div class="card">
        <div class="card-header">
            <header class="navbar">
                <section class="navbar-section">
                    <h3>Queue Status</h3>
                </section>
                <section class="navbar-section">
                    <a href="/queue" class="btn btn-sm btn-primary">
                        <i class="fas fa-list"></i> View Queue
                    </a>
                </section>
            </header>
        </div>
        <div class="card-body">
            <div class="columns">
                <div class="column col-4">
                    <div class="tile tile-centered">
                        <div class="tile-icon">
                            <i class="fas fa-clock fa-2x text-primary"></i>
                        </div>
                        <div class="tile-content">
                            <div class="tile-title text-bold" x-text="stats.pending_messages">0</div>
                            <div class="tile-subtitle text-gray">Pending Jobs</div>
                        </div>
                    </div>
                </div>
                <div class="column col-4">
                    <div class="tile tile-centered">
                        <div class="tile-icon">
                            <i class="fas fa-spinner fa-2x text-warning"></i>
                        </div>
                        <div class="tile-content">
                            <div class="tile-title text-bold" x-text="stats.in_flight_messages">0</div>
                            <div class="tile-subtitle text-gray">Running Jobs</div>
                        </div>
                    </div>
                </div>
                <div class="column col-4">
                    <div class="tile tile-centered">
                        <div class="tile-icon">
                            <i class="fas fa-users fa-2x text-success"></i>
                        </div>
                        <div class="tile-content">
                            <div class="tile-title text-bold" x-text="stats.concurrency">0</div>
                            <div class="tile-subtitle text-gray">Active Workers</div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</section>
```

**Add Alpine.js Component for Queue Status (in script section, after line 519):**

Add new component:
```javascript
function queueStatusOverview() {
    return {
        stats: {
            total_messages: 0,
            pending_messages: 0,
            in_flight_messages: 0,
            concurrency: 0
        },
        
        init() {
            this.subscribeToWebSocket();
        },
        
        subscribeToWebSocket() {
            if (typeof WebSocketManager !== 'undefined') {
                WebSocketManager.subscribe('queue_stats', (data) => {
                    this.stats = data;
                });
            }
        }
    }
}
```

**Update Job Definitions Table to Show Execution Status (modify table around line 295):**

Add status column after SCHEDULE column:
```html
<th>LAST RUN</th>
```

Add corresponding cell in tbody:
```html
<td>
    <template x-if="jobDef.last_execution">
        <div>
            <span class="label" :class="getExecutionStatusClass(jobDef.last_execution.status)" x-text="jobDef.last_execution.status"></span>
            <br>
            <small class="text-gray" x-text="formatDate(jobDef.last_execution.timestamp)"></small>
        </div>
    </template>
    <template x-if="!jobDef.last_execution">
        <span class="text-gray">Never</span>
    </template>
</td>
```

Add helper method to jobDefinitionsManagement component:
```javascript
getExecutionStatusClass(status) {
    const statusMap = {
        'queued': 'label-primary',
        'running': 'label-warning',
        'completed': 'label-success',
        'failed': 'label-error'
    };
    return statusMap[status] || 'label';
}
```

**Note:** The last_execution data would need to be tracked in the backend (future enhancement). For now, the UI structure is prepared for when this data becomes available.

### pages\static\common.js(MODIFY)

References: 

- pages\static\websocket-manager.js
- internal\handlers\websocket.go(MODIFY)

**Add Queue Stats Component (after jobDefinitionsManagement component, around line 802):**

Add new Alpine.js component:
```javascript
// Queue Stats Component
Alpine.data('queueStats', () => ({
    stats: {
        total_messages: 0,
        pending_messages: 0,
        in_flight_messages: 0,
        concurrency: 0,
        queue_name: ''
    },
    connectionStatus: false,
    lastUpdate: null,
    
    init() {
        window.debugLog('QueueStats', 'Initializing component');
        this.subscribeToWebSocket();
    },
    
    subscribeToWebSocket() {
        if (typeof WebSocketManager !== 'undefined') {
            // Subscribe to queue stats updates
            WebSocketManager.subscribe('queue_stats', (data) => {
                window.debugLog('QueueStats', 'Stats update received:', data);
                this.stats = data;
                this.lastUpdate = new Date();
            });
            
            // Subscribe to connection status
            WebSocketManager.onConnectionChange((isConnected) => {
                window.debugLog('QueueStats', 'Connection status changed:', isConnected);
                this.connectionStatus = isConnected;
            });
            
            window.debugLog('QueueStats', 'WebSocket subscriptions established');
        } else {
            window.debugError('QueueStats', 'WebSocketManager not loaded', new Error('WebSocketManager undefined'));
        }
    },
    
    formatLastUpdate() {
        if (!this.lastUpdate) return 'Never';
        const now = new Date();
        const diffMs = now - this.lastUpdate;
        const diffSecs = Math.floor(diffMs / 1000);
        
        if (diffSecs < 10) return 'Just now';
        if (diffSecs < 60) return `${diffSecs}s ago`;
        return this.lastUpdate.toLocaleTimeString();
    }
}));

// Queue Status Overview Component (for jobs.html)
Alpine.data('queueStatusOverview', () => ({
    stats: {
        total_messages: 0,
        pending_messages: 0,
        in_flight_messages: 0,
        concurrency: 0
    },
    
    init() {
        window.debugLog('QueueStatusOverview', 'Initializing component');
        this.subscribeToWebSocket();
    },
    
    subscribeToWebSocket() {
        if (typeof WebSocketManager !== 'undefined') {
            WebSocketManager.subscribe('queue_stats', (data) => {
                window.debugLog('QueueStatusOverview', 'Stats update received:', data);
                this.stats = data;
            });
            window.debugLog('QueueStatusOverview', 'WebSocket subscription established');
        }
    }
}));

// Job Spawn Notifications Component
Alpine.data('jobSpawnNotifications', () => ({
    recentSpawns: [],
    maxSpawns: 10,
    
    init() {
        window.debugLog('JobSpawnNotifications', 'Initializing component');
        this.subscribeToWebSocket();
    },
    
    subscribeToWebSocket() {
        if (typeof WebSocketManager !== 'undefined') {
            WebSocketManager.subscribe('job_spawn', (data) => {
                window.debugLog('JobSpawnNotifications', 'Job spawn event received:', data);
                this.addSpawn(data);
            });
            window.debugLog('JobSpawnNotifications', 'WebSocket subscription established');
        }
    },
    
    addSpawn(spawnData) {
        // Add to beginning of array
        this.recentSpawns.unshift({
            ...spawnData,
            timestamp: new Date(spawnData.timestamp)
        });
        
        // Limit array size
        if (this.recentSpawns.length > this.maxSpawns) {
            this.recentSpawns = this.recentSpawns.slice(0, this.maxSpawns);
        }
        
        // Show notification
        if (spawnData.url) {
            window.showNotification(
                `Job spawned: ${spawnData.url} (depth ${spawnData.depth})`,
                'info'
            );
        }
    },
    
    formatTimestamp(timestamp) {
        if (!timestamp) return '';
        const date = new Date(timestamp);
        return date.toLocaleTimeString();
    },
    
    clearSpawns() {
        this.recentSpawns = [];
    }
}));
```

**Update jobDefinitionsManagement Component:**

Add method to handle execution status display (after formatSourcesList method, around line 801):
```javascript
getExecutionStatusClass(status) {
    const statusMap = {
        'queued': 'label-primary',
        'running': 'label-warning',
        'completed': 'label-success',
        'failed': 'label-error',
        'cancelled': 'label'
    };
    return statusMap[status] || 'label';
}
```

**Note:** These components provide real-time queue monitoring and job spawning notifications across the UI. They integrate with the WebSocket infrastructure to receive updates from the server.

### internal\jobs\types\crawler.go(MODIFY)

References: 

- internal\interfaces\event_service.go(MODIFY)
- internal\handlers\websocket.go(MODIFY)
- internal\app\app.go(MODIFY)

**Add Job Spawn Event Publishing (in Execute method, around line 360):**

After enqueueing child jobs (around line 360, in the loop that creates child messages), add WebSocket event publishing:

```go
// Enqueue child job
if err := c.deps.QueueManager.Enqueue(ctx, childMsg); err != nil {
    c.logger.Warn().Err(err).Str("url", link).Msg("Failed to enqueue child job")
    continue
}

// Publish job spawn event for real-time UI updates
if c.deps.EventService != nil {
    spawnEvent := interfaces.Event{
        Type: interfaces.EventJobSpawn,
        Payload: map[string]interface{}{
            "parent_job_id": msg.ParentID,
            "child_job_id":  childMsg.ID,
            "job_type":      "crawler_url",
            "url":           link,
            "depth":         newDepth,
            "timestamp":     time.Now(),
        },
    }
    c.deps.EventService.Publish(ctx, spawnEvent)
}

c.logger.Debug().
    Str("parent_id", msg.ParentID).
    Str("child_id", childMsg.ID).
    Str("url", link).
    Int("depth", newDepth).
    Msg("Child job spawned")
```

**Add EventService to CrawlerJobDeps (line 19-24):**

Update struct:
```go
type CrawlerJobDeps struct {
    CrawlerService  *crawler.Service
    LogService      interfaces.LogService
    DocumentStorage interfaces.DocumentStorage
    QueueManager    interfaces.QueueManager
    JobStorage      interfaces.JobStorage
    EventService    interfaces.EventService  // ADD THIS
}
```

**Update app.go to pass EventService (line 381-386):**

Update deps initialization:
```go
crawlerJobDeps := &jobtypes.CrawlerJobDeps{
    CrawlerService:  a.CrawlerService,
    LogService:      a.LogService,
    DocumentStorage: a.StorageManager.DocumentStorage(),
    QueueManager:    a.QueueManager,
    JobStorage:      a.StorageManager.JobStorage(),
    EventService:    a.EventService,  // ADD THIS
}
```

**Add EventJobSpawn Constant (in internal/interfaces/event_service.go):**

Add new event type:
```go
const (
    EventCollectionTriggered EventType = "collection_triggered"
    EventEmbeddingTriggered  EventType = "embedding_triggered"
    EventCrawlProgress       EventType = "crawl_progress"
    EventJobProgress         EventType = "job_progress"
    EventStatusChanged       EventType = "status_changed"
    EventJobSpawn            EventType = "job_spawn"  // ADD THIS
)
```

**Subscribe to EventJobSpawn in WebSocket Handler:**

Add subscription in `internal/handlers/websocket.go` SubscribeToCrawlerEvents method (after line 732):
```go
h.eventService.Subscribe(interfaces.EventJobSpawn, func(ctx context.Context, event interfaces.Event) error {
    payload, ok := event.Payload.(map[string]interface{})
    if !ok {
        h.logger.Warn().Msg("Invalid job spawn event payload type")
        return nil
    }
    
    spawn := JobSpawnUpdate{
        ParentJobID: getString(payload, "parent_job_id"),
        ChildJobID:  getString(payload, "child_job_id"),
        JobType:     getString(payload, "job_type"),
        URL:         getString(payload, "url"),
        Depth:       getInt(payload, "depth"),
        Timestamp:   time.Now(),
    }
    
    h.BroadcastJobSpawn(spawn)
    return nil
})
```

### internal\interfaces\event_service.go(MODIFY)

References: 

- internal\jobs\types\crawler.go(MODIFY)
- internal\handlers\websocket.go(MODIFY)

**Add EventJobSpawn Constant:**

Add new event type to the EventType constants (after existing event types):
```go
const (
    EventCollectionTriggered EventType = "collection_triggered"
    EventEmbeddingTriggered  EventType = "embedding_triggered"
    EventCrawlProgress       EventType = "crawl_progress"
    EventJobProgress         EventType = "job_progress"
    EventStatusChanged       EventType = "status_changed"
    EventJobSpawn            EventType = "job_spawn"  // NEW: Published when a job spawns child jobs
)
```

This event type is used by CrawlerJob to notify the UI when child jobs are spawned during URL discovery.