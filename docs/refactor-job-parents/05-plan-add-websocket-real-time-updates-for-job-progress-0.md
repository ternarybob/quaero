I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**Backend Event Publishing (Working):**
- ✅ `EventJobSpawn` is published in `crawler.go` (lines 454-468) when child jobs are enqueued
- ✅ `EventJobStarted` is published (lines 92-124) when job transitions from pending to running
- ✅ `EventJobCompleted` is published (lines 704-728) after successful completion
- ❌ `EventJobFailed` is NOT published when jobs fail (missing in error handling paths)

**Backend WebSocket Infrastructure (Partially Working):**
- ✅ `WebSocketHandler` has `BroadcastJobSpawn()` method (lines 628-660 in websocket.go)
- ✅ `JobSpawnUpdate` struct exists (lines 172-179 in websocket.go)
- ✅ `WebSocketHandler.SubscribeToCrawlerEvents()` subscribes to `EventJobSpawn` (lines 792-821)
- ✅ Throttling support exists for `job_spawn` events (lines 88-102)
- ❌ `EventSubscriber` in `websocket_events.go` does NOT subscribe to `EventJobSpawn`

**Frontend WebSocket Handling (Partially Working):**
- ✅ WebSocket connection established in `queue.html` (lines 663-759)
- ✅ Handles `job_status_change`, `job_created`, `job_progress`, `job_completed` events
- ❌ Does NOT handle `job_spawn` events
- ✅ `updateJobInList()` method exists for updating jobs from WebSocket

**Frontend UI State (Working):**
- ✅ Alpine.js `jobList` component manages parent/child state
- ✅ `childJobsCache` Map stores loaded children
- ✅ `expandedParents` Set tracks expanded parents
- ✅ `loadChildJobs()` method fetches children from API
- ✅ `toggleParentExpansion()` method handles expand/collapse

**Frontend UI Display (Needs Work):**
- ✅ Parent jobs show progress text via `getParentProgressText()` (lines 1303-1323)
- ✅ Progress bar exists with `parent-progress-bar` CSS (lines 870-882 in quaero.css)
- ❌ Progress bar shows percentage, not accumulated count
- ❌ Progress bar is single-color (green), not multi-segment (completed/failed/running)
- ❌ Child jobs are loaded on-demand, not displayed in real-time as they spawn
- ❌ No visual list of spawned children with individual status badges

**API Enrichment (Working):**
- ✅ `JobHandler.ListJobsHandler` enriches parent jobs with `child_count`, `completed_children`, `failed_children` (lines 173-181)
- ✅ `JobStorage.GetJobChildStats()` aggregates child statistics
- ✅ WebSocket `JobStatusUpdate` includes `result_count`, `failed_count`, `total_urls`, `completed_urls`, `pending_urls`

**CSS Styling (Partially Ready):**
- ✅ `.parent-progress` container exists (lines 855-864)
- ✅ `.parent-progress-bar` and `.parent-progress-bar-fill` exist (lines 870-882)
- ✅ Status badge classes exist: `.label-orchestrating`, `.label-queued`, `.label-processing`, `.label-done`
- ❌ No multi-segment progress bar styles
- ❌ No scrollable child list container styles

### Approach

## Implementation Strategy

**Phase 1: Backend Event Publishing**
Add missing `EventJobFailed` publication in crawler job error paths to ensure complete job lifecycle event coverage.

**Phase 2: Backend WebSocket Integration**
Add `handleJobSpawn` handler in `EventSubscriber` to bridge `EventJobSpawn` events to WebSocket broadcasts. This completes the event pipeline: CrawlerJob → EventService → EventSubscriber → WebSocketHandler → Browser.

**Phase 3: Frontend WebSocket Handler**
Add `job_spawn` message handler in `queue.html` WebSocket connection to receive spawn events and update UI state in real-time.

**Phase 4: Frontend UI State Management**
Enhance Alpine.js `jobList` component to maintain a real-time child job list that updates as spawn events arrive, without requiring API calls.

**Phase 5: Frontend UI Display**
- Replace progress bar percentage display with accumulated count display
- Add multi-segment progress bar visualization (completed/failed/running)
- Add scrollable child job list with individual status badges
- Update progress text to show "X child jobs spawned" format

**Phase 6: CSS Enhancements**
Add styles for multi-segment progress bars and scrollable child job lists with proper overflow handling.

### Reasoning

I explored the codebase by reading:
1. Event publishing in `crawler.go` - confirmed spawn/started/completed events are published
2. WebSocket infrastructure in `websocket.go` and `websocket_events.go` - found spawn handler missing in EventSubscriber
3. Frontend WebSocket handling in `queue.html` - confirmed spawn event handler is missing
4. Alpine.js component structure - understood state management and rendering logic
5. CSS styling in `quaero.css` - found existing progress bar styles
6. API enrichment in `job_handler.go` - confirmed child statistics are available
7. Common utilities in `common.js` - understood notification and WebSocket manager patterns

## Mermaid Diagram

sequenceDiagram
    participant CJ as CrawlerJob
    participant ES as EventService
    participant ESub as EventSubscriber
    participant WSH as WebSocketHandler
    participant Browser as Browser (queue.html)
    participant Alpine as Alpine.js jobList
    
    Note over CJ,Alpine: Child Job Spawn Flow
    
    CJ->>CJ: Discover link, create child job
    CJ->>ES: Publish EventJobSpawn
    Note right of ES: payload: parent_job_id,<br/>child_job_id, url, depth
    
    ES->>ESub: Notify handleJobSpawn
    ESub->>ESub: Check whitelist & throttle
    ESub->>ESub: Transform to JobSpawnUpdate
    ESub->>WSH: BroadcastJobSpawn(update)
    
    WSH->>Browser: WebSocket message<br/>type: "job_spawn"
    
    Browser->>Browser: Parse message
    Browser->>Alpine: Dispatch 'jobList:childSpawned'
    
    Alpine->>Alpine: handleChildSpawned(spawnData)
    Alpine->>Alpine: Add to childJobsList Map
    Alpine->>Alpine: Increment parent.child_count
    Alpine->>Alpine: renderJobs()
    
    Note over Alpine: UI Updates
    Alpine->>Browser: Update progress text<br/>"X child jobs spawned"
    Alpine->>Browser: Update progress bar<br/>(multi-segment)
    Alpine->>Browser: Add child to scrollable list<br/>(URL, status badge, depth)
    
    Note over CJ,Alpine: Child Job Status Update Flow
    
    CJ->>ES: Publish EventJobCompleted
    ES->>ESub: Notify handleJobCompleted
    ESub->>WSH: BroadcastJobStatusChange
    WSH->>Browser: WebSocket message<br/>type: "job_status_change"
    
    Browser->>Alpine: updateJobInList(update)
    Alpine->>Alpine: handleChildJobStatus(jobId, status)
    Alpine->>Alpine: Update child in childJobsList
    Alpine->>Alpine: Increment parent.completed_children
    Alpine->>Alpine: renderJobs()
    
    Alpine->>Browser: Update child status badge<br/>(pending → completed)
    Alpine->>Browser: Update progress bar<br/>(green segment grows)
    Alpine->>Browser: Update progress text<br/>"X completed, Y failed, Z running"

## Proposed File Changes

### internal\jobs\types\crawler.go(MODIFY)

References: 

- internal\interfaces\event_service.go

## Add EventJobFailed Publication in Error Paths

**Location 1: Scraping Failure (After Line 231)**
When URL scraping fails, publish `EventJobFailed` event before returning error.

**Add after line 231:**
```go
// Publish EventJobFailed if this is a critical failure
if c.deps.EventService != nil {
    failedEvent := interfaces.Event{
        Type: interfaces.EventJobFailed,
        Payload: map[string]interface{}{
            "job_id":      msg.ParentID,
            "status":      "failed",
            "source_type": sourceType,
            "entity_type": entityType,
            "error":       err.Error(),
            "timestamp":   time.Now(),
        },
    }
    if err := c.deps.EventService.Publish(ctx, failedEvent); err != nil {
        c.logger.Warn().Err(err).Msg("Failed to publish job failed event")
    }
}
```

**Location 2: Non-Success Status (After Line 275)**
When scraping returns non-2xx status, publish `EventJobFailed` event.

**Add after line 275:**
```go
// Publish EventJobFailed for non-success status
if c.deps.EventService != nil {
    failedEvent := interfaces.Event{
        Type: interfaces.EventJobFailed,
        Payload: map[string]interface{}{
            "job_id":      msg.ParentID,
            "status":      "failed",
            "source_type": sourceType,
            "entity_type": entityType,
            "error":       fmt.Sprintf("HTTP %d: %s", scrapeResult.StatusCode, scrapeResult.Error),
            "timestamp":   time.Now(),
        },
    }
    if err := c.deps.EventService.Publish(ctx, failedEvent); err != nil {
        c.logger.Warn().Err(err).Msg("Failed to publish job failed event")
    }
}
```

**Rationale:** Complete the job lifecycle event coverage by publishing `EventJobFailed` when jobs encounter errors. This ensures the UI receives real-time updates for all job state transitions, not just successful ones.

### internal\handlers\websocket_events.go(MODIFY)

References: 

- internal\handlers\websocket.go
- internal\interfaces\event_service.go

## Add handleJobSpawn Handler to EventSubscriber

**Location: After handleJobCancelled method (After Line 291)**

Add new handler method:
```go
func (s *EventSubscriber) handleJobSpawn(ctx context.Context, event interfaces.Event) error {
    // Check if event should be broadcast (filtering + throttling)
    if !s.shouldBroadcastEvent("job_spawn") {
        return nil
    }

    payload, ok := event.Payload.(map[string]interface{})
    if !ok {
        s.logger.Warn().Msg("Invalid job spawn event payload type")
        return nil
    }

    // Extract spawn details
    spawnUpdate := handlers.JobSpawnUpdate{
        ParentJobID: getStringWithFallback(payload, "parent_job_id", "parentJobId"),
        ChildJobID:  getStringWithFallback(payload, "child_job_id", "childJobId"),
        JobType:     getStringWithFallback(payload, "job_type", "jobType"),
        URL:         getString(payload, "url"),
        Depth:       getIntWithFallback(payload, "depth", "depth"),
        Timestamp:   getTimestamp(payload),
    }

    // Broadcast to WebSocket clients
    s.handler.BroadcastJobSpawn(spawnUpdate)
    return nil
}
```

**Location: In SubscribeAll method (After Line 97)**

Add subscription to `EventJobSpawn`:
```go
// Subscribe to job spawn events
s.eventService.Subscribe(interfaces.EventJobSpawn, s.handleJobSpawn)
```

**Update logging in SubscribeAll (Line 99):**
Change log message to reflect all subscribed events:
```go
s.logger.Info().Msg("EventSubscriber registered for all job lifecycle events (created, started, completed, failed, cancelled, spawn)")
```

**Rationale:** This bridges the gap between backend event publishing and WebSocket broadcasting. When `CrawlerJob` publishes `EventJobSpawn`, this handler transforms it into a `JobSpawnUpdate` and broadcasts it to all connected WebSocket clients. The handler respects the existing filtering and throttling infrastructure.
## Import handlers Package for JobSpawnUpdate Type

**Location: At top of file (After Line 10)**

Ensure the handlers package is imported (it should already be imported as the file is in the handlers package, but verify the struct reference):

The `handleJobSpawn` method references `handlers.JobSpawnUpdate`, but since this file is already in the `handlers` package, it should just be `JobSpawnUpdate` without the package prefix.

**Correction in handleJobSpawn method:**
Change:
```go
spawnUpdate := handlers.JobSpawnUpdate{
```
To:
```go
spawnUpdate := JobSpawnUpdate{
```

**Rationale:** Avoid redundant package prefix when referencing types within the same package. The `JobSpawnUpdate` struct is defined in `websocket.go` which is in the same `handlers` package.

### pages\queue.html(MODIFY)

## Add job_spawn WebSocket Message Handler

**Location: In connectJobsWebSocket function, inside jobsWS.onmessage handler (After Line 717)**

Add new message type handler:
```javascript
// Handle job spawn events
if (message.type === 'job_spawn' && message.payload) {
    const spawnData = message.payload;
    // Dispatch event to Alpine component
    window.dispatchEvent(new CustomEvent('jobList:childSpawned', {
        detail: {
            parent_job_id: spawnData.parent_job_id,
            child_job_id: spawnData.child_job_id,
            job_type: spawnData.job_type,
            url: spawnData.url,
            depth: spawnData.depth,
            timestamp: spawnData.timestamp
        }
    }));
}
```

**Rationale:** This handler receives `job_spawn` WebSocket messages and dispatches them as custom events to the Alpine.js component. This decouples WebSocket handling from component logic and follows the existing pattern used for other job events.
## Enhance Alpine.js jobList Component for Real-Time Child Job Updates

**Location 1: Add childJobsList to Component State (After Line 1133)**

Add new state property:
```javascript
childJobsList: new Map(), // Map: parentID -> array of child job metadata (not full jobs)
```

**Location 2: Add Event Listener in init() Method (After Line 1145)**

Add listener for child spawn events:
```javascript
window.addEventListener('jobList:childSpawned', (e) => this.handleChildSpawned(e.detail));
```

**Location 3: Add handleChildSpawned Method (After Line 1253)**

Add new method:
```javascript
handleChildSpawned(spawnData) {
    const parentId = spawnData.parent_job_id;
    
    // Initialize child list for parent if not exists
    if (!this.childJobsList.has(parentId)) {
        this.childJobsList.set(parentId, []);
    }
    
    // Add child metadata to list
    const childMeta = {
        id: spawnData.child_job_id,
        url: spawnData.url,
        depth: spawnData.depth,
        status: 'pending', // Initial status
        timestamp: spawnData.timestamp
    };
    
    this.childJobsList.get(parentId).push(childMeta);
    
    // Update parent job child_count in allJobs
    const parentJob = this.allJobs.find(j => j.id === parentId);
    if (parentJob) {
        parentJob.child_count = (parentJob.child_count || 0) + 1;
    }
    
    // Re-render to show updated count
    this.renderJobs();
    
    console.log('[Queue] Child job spawned:', spawnData.child_job_id.substring(0, 8), 'for parent:', parentId.substring(0, 8));
}
```

**Location 4: Update handleChildJobStatus Method (New Method After handleChildSpawned)**

Add method to update child status from WebSocket events:
```javascript
handleChildJobStatus(jobId, status) {
    // Find parent that contains this child
    for (const [parentId, children] of this.childJobsList.entries()) {
        const child = children.find(c => c.id === jobId);
        if (child) {
            child.status = status;
            
            // Update parent statistics
            const parentJob = this.allJobs.find(j => j.id === parentId);
            if (parentJob) {
                if (status === 'completed') {
                    parentJob.completed_children = (parentJob.completed_children || 0) + 1;
                } else if (status === 'failed') {
                    parentJob.failed_children = (parentJob.failed_children || 0) + 1;
                }
            }
            
            this.renderJobs();
            break;
        }
    }
}
```

**Location 5: Update updateJobInList Method (Around Line 1617)**

Add call to handleChildJobStatus when child job status changes:
```javascript
// After updating job status (around line 1645)
if (job.parent_id && (update.status === 'completed' || update.status === 'failed')) {
    this.handleChildJobStatus(job.id, update.status);
}
```

**Rationale:** This creates a lightweight real-time child job tracking system. Instead of fetching full child job data from the API, we maintain metadata (URL, status, timestamp) in memory and update it as spawn and status events arrive. This reduces API load and provides instant UI updates.
## Update Progress Bar to Show Accumulated Count

**Location 1: Update getParentProgressText Method (Replace Lines 1303-1323)**

Replace entire method:
```javascript
getParentProgressText(job) {
    // Check if currently loading children
    if (this.loadingParents.has(job.id)) {
        return 'Loading children...';
    }

    // Use child statistics from job
    const total = job.child_count || 0;
    const completed = job.completed_children || 0;
    const failed = job.failed_children || 0;
    const running = total - completed - failed;

    if (total === 0) {
        return 'No child jobs spawned yet';
    }

    // Build status summary
    const parts = [];
    if (completed > 0) parts.push(`${completed} completed`);
    if (failed > 0) parts.push(`${failed} failed`);
    if (running > 0) parts.push(`${running} running`);

    return `${total} child jobs spawned (${parts.join(', ')})`;
}
```

**Location 2: Update getParentProgressBarStyle Method (Replace Lines 1326-1334)**

Replace entire method to support multi-segment progress bar:
```javascript
getParentProgressBarStyle(job) {
    const total = job.child_count || 0;
    if (total === 0) {
        return { width: '0%' };
    }

    const completed = job.completed_children || 0;
    const failed = job.failed_children || 0;
    const running = total - completed - failed;

    const completedPercent = (completed / total) * 100;
    const failedPercent = (failed / total) * 100;
    const runningPercent = (running / total) * 100;

    // Return CSS for multi-segment bar using linear-gradient
    return {
        background: `linear-gradient(to right, 
            var(--color-success) 0% ${completedPercent}%, 
            var(--color-danger) ${completedPercent}% ${completedPercent + failedPercent}%, 
            var(--color-primary) ${completedPercent + failedPercent}% 100%)`
    };
}
```

**Location 3: Remove getParentProgressStyle Method (Delete Lines 1336-1338)**

This method is no longer needed.

**Rationale:** The progress bar now shows accumulated counts instead of percentages, with visual segments for completed (green), failed (red), and running (blue) child jobs. The text clearly communicates the job spawning progress without implying a known total upfront.
## Add Scrollable Child Job List Display

**Location: In HTML Template, Replace Parent Progress Section (Lines 243-254)**

Replace the existing parent progress display with:
```html
<!-- Parent progress display -->
<template x-if="item.type === 'parent'">
    <div class="parent-progress-container">
        <!-- Progress summary -->
        <div class="parent-progress">
            <div class="parent-progress-text" 
                 x-text="getParentProgressText(item.job)"></div>
            <div class="parent-progress-bar">
                <div class="parent-progress-bar-fill" 
                     :style="getParentProgressBarStyle(item.job)"></div>
            </div>
        </div>
        
        <!-- Spawned child jobs list (if any) -->
        <template x-if="childJobsList.has(item.job.id) && childJobsList.get(item.job.id).length > 0">
            <div class="child-jobs-list-container">
                <div class="child-jobs-list-header">
                    <span x-text="'Spawned URLs (' + childJobsList.get(item.job.id).length + ')'"></span>
                </div>
                <div class="child-jobs-list">
                    <template x-for="child in childJobsList.get(item.job.id)" :key="child.id">
                        <div class="child-job-item">
                            <span class="label label-sm" 
                                  :class="{
                                      'label-secondary': child.status === 'pending',
                                      'label-primary': child.status === 'running',
                                      'label-success': child.status === 'completed',
                                      'label-error': child.status === 'failed'
                                  }"
                                  x-text="child.status"></span>
                            <span class="child-job-url" :title="child.url" x-text="child.url"></span>
                            <span class="child-job-depth" x-text="'Depth: ' + child.depth"></span>
                        </div>
                    </template>
                </div>
            </div>
        </template>
    </div>
</template>
```

**Rationale:** This replaces the static progress display with a dynamic list that shows spawned child jobs in real-time. Each child displays its URL, status badge, and depth. The list is scrollable to handle 100+ children without breaking the layout.
## Add Smooth Transition for Progress Bar Updates

**Location: Update parent-progress-bar-fill Style Binding (In HTML Template, Line 251)**

Update the style binding to include transition:
```html
<div class="parent-progress-bar-fill" 
     :style="Object.assign(getParentProgressBarStyle(item.job), { transition: 'background 0.3s ease' })"></div>
```

**Rationale:** This ensures smooth visual transitions when the progress bar segments change as child jobs complete or fail. The transition is applied inline to work with Alpine.js reactive updates.

### pages\static\quaero.css(MODIFY)

## Add Styles for Multi-Segment Progress Bar and Child Job List

**Location 1: Update Progress Bar Styles (Replace Lines 870-882)**

Replace existing progress bar styles:
```css
.parent-progress-bar {
    flex: 1;
    height: 8px;
    background-color: #e0e0e0;
    border-radius: 4px;
    overflow: hidden;
}

.parent-progress-bar-fill {
    height: 100%;
    /* Background set dynamically via Alpine.js for multi-segment display */
    transition: background 0.3s ease;
}
```

**Location 2: Add Child Jobs List Container Styles (After Line 882)**

Add new styles:
```css
/* Child jobs list container */
.parent-progress-container {
    margin-top: 0.5rem;
}

.child-jobs-list-container {
    margin-top: 0.75rem;
    border: 1px solid var(--border-color);
    border-radius: var(--border-radius);
    background-color: var(--content-bg);
}

.child-jobs-list-header {
    padding: 0.5rem 0.75rem;
    background-color: var(--page-bg);
    border-bottom: 1px solid var(--border-color);
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--text-secondary);
}

.child-jobs-list {
    max-height: 200px;
    overflow-y: auto;
    padding: 0.5rem;
}

.child-job-item {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.4rem 0.5rem;
    margin-bottom: 0.3rem;
    background-color: var(--page-bg);
    border-radius: var(--border-radius);
    font-size: 0.75rem;
}

.child-job-item:last-child {
    margin-bottom: 0;
}

.child-job-url {
    flex: 1;
    font-family: 'SF Mono', Monaco, Consolas, monospace;
    color: var(--text-secondary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0; /* Allow flex item to shrink */
}

.child-job-depth {
    font-size: 0.7rem;
    color: var(--text-secondary);
    white-space: nowrap;
}

/* Scrollbar styling for child jobs list */
.child-jobs-list::-webkit-scrollbar {
    width: 6px;
}

.child-jobs-list::-webkit-scrollbar-track {
    background: var(--page-bg);
}

.child-jobs-list::-webkit-scrollbar-thumb {
    background: var(--border-color);
    border-radius: 3px;
}

.child-jobs-list::-webkit-scrollbar-thumb:hover {
    background: var(--text-secondary);
}
```

**Location 3: Add Small Label Variant (After Line 226)**

Add size variant for status badges:
```css
.label-sm {
    font-size: 0.65rem;
    padding: 0.1rem 0.3rem;
}
```

**Rationale:** These styles create a clean, scrollable child job list with proper overflow handling, status badges, and URL display. The multi-segment progress bar uses CSS linear-gradient for smooth color transitions. The scrollbar is styled to match the application theme.