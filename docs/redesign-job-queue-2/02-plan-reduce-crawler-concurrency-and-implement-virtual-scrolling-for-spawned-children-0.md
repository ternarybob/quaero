I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The task addresses two critical performance issues in the Quaero queue management system:

**Backend Issue**: Queue concurrency is set to 5 workers, causing SQLite locking errors under heavy load. SQLite's WAL mode (enabled in config) supports concurrent reads but has limitations with concurrent writes. With 5 workers processing jobs simultaneously, write contention becomes problematic.

**Frontend Issue**: The child jobs list renders all children at once (up to 200 displayed, 1000 fetched), causing:
- DOM bloat with 200+ elements
- Poor UI responsiveness
- Overwhelming visual complexity
- Memory pressure in the browser

**Current Architecture**:
- Queue concurrency: Defined in `internal/queue/config.go` (line 27) and `internal/common/config.go` (line 220)
- Child jobs rendering: Alpine.js component in `pages/queue.html` (lines 262-294, 1274-1471)
- Child jobs are fetched via `loadChildJobs()` with limit of 1000 (line 1443)
- All fetched children are rendered immediately using `x-for` directive (line 269)

**Key Constraints**:
- SQLite busy timeout is 5000ms (5 seconds) - reducing concurrency will decrease lock contention
- WAL mode is enabled for better concurrency, but still has limits
- Child jobs are displayed inline within parent job cards
- Real-time updates via WebSocket must continue to work with pagination

### Approach

**Two-Phase Approach**:

**Phase 1 - Backend**: Reduce queue worker concurrency from 5 to 3 workers to minimize SQLite write contention while maintaining reasonable throughput. This is a conservative middle-ground that balances performance with stability.

**Phase 2 - Frontend**: Implement pagination for child jobs list with "Load More" functionality. This approach is simpler than virtual scrolling, more user-friendly, and maintains compatibility with WebSocket real-time updates. Initial page size of 25 children provides good balance between information density and performance.

### Reasoning

I explored the repository structure, read the queue configuration files (`internal/queue/config.go`, `internal/common/config.go`), examined the queue initialization in `internal/app/app.go`, and analyzed the frontend child jobs rendering logic in `pages/queue.html`. I identified the exact locations where concurrency is configured and where child jobs are rendered, understanding both the current implementation and the performance bottlenecks.

## Mermaid Diagram

sequenceDiagram
    participant User
    participant UI as Queue UI (Alpine.js)
    participant API as Backend API
    participant Queue as Queue Manager (3 workers)
    participant SQLite as SQLite Database

    Note over Queue,SQLite: Backend: Reduced Concurrency (5→3 workers)
    
    User->>UI: Expand parent job
    UI->>API: GET /api/jobs?parent_id=X&limit=500
    API->>SQLite: Query child jobs
    SQLite-->>API: Return 500 children
    API-->>UI: Child jobs data
    
    Note over UI: Store all 500 in childJobsList<br/>Initialize visibleCount = 25
    
    UI->>UI: Render first 25 children only
    UI->>User: Display 25 children + "Load More" button
    
    User->>UI: Click "Load More"
    UI->>UI: Increase visibleCount by 25
    UI->>UI: Re-render (now showing 50)
    UI->>User: Display 50 children + "Load More" button
    
    Note over Queue,SQLite: Meanwhile: Queue processes jobs with 3 workers
    
    Queue->>SQLite: Worker 1: Write job result
    Queue->>SQLite: Worker 2: Write job result
    Queue->>SQLite: Worker 3: Write job result
    
    Note over SQLite: Reduced write contention<br/>Fewer busy timeout errors
    
    SQLite-->>Queue: Success (no locking errors)
    
    Note over UI: WebSocket receives child_spawned event
    UI->>UI: Add new child to top of list
    UI->>UI: Re-render visible subset
    UI->>User: New child appears (if in visible range)

## Proposed File Changes

### internal\queue\config.go(MODIFY)

**Reduce default queue concurrency from 5 to 3 workers**:

1. Locate the `NewDefaultConfig()` function (line 24)
2. Change the `Concurrency` field value from `5` to `3` (line 27)

**Rationale**: 
- SQLite with WAL mode handles concurrent reads well but struggles with concurrent writes
- 5 workers create excessive write contention, causing busy timeout errors
- 3 workers provide a balance between throughput and stability
- This is a conservative reduction that can be tuned further if needed
- Users can still override via `QUAERO_QUEUE_CONCURRENCY` environment variable if they have different requirements

**Impact**:
- Reduces SQLite locking errors significantly
- Slightly increases job processing time (acceptable tradeoff)
- Maintains reasonable throughput for typical workloads

### internal\common\config.go(MODIFY)

References: 

- internal\queue\config.go(MODIFY)

**Update default queue concurrency in main config to match queue config**:

1. Locate the `NewDefaultConfig()` function (line 210)
2. Find the `Queue` configuration section (lines 218-224)
3. Change the `Concurrency` field value from `5` to `3` (line 220)

**Rationale**:
- Maintains consistency with `internal/queue/config.go`
- Both files define default concurrency, so they must match
- This is the configuration that gets loaded by the application

**Note**: The comment on line 208 states "Technical parameters are hardcoded here for production stability" - this change aligns with that philosophy by choosing a more stable default value.

### pages\queue.html(MODIFY)

**Implement pagination for child jobs list with "Load More" functionality**:

**1. Add pagination state to Alpine.js `jobList` component** (around line 1271):
   - Add `childJobsPageSize: 25` - Number of children to show per page
   - Add `childJobsVisibleCount: new Map()` - Map of parentId -> number of visible children
   - Initialize visible count to `childJobsPageSize` for each parent when expanded

**2. Modify `loadChildJobs()` function** (lines 1439-1471):
   - Reduce the fetch limit from `1000` to `500` (line 1443) to prevent excessive data transfer
   - When storing children in `childJobsList`, initialize the visible count for this parent to `childJobsPageSize`
   - Keep the full list in `childJobsList` but only render a subset

**3. Add helper function `getVisibleChildJobs(parentId)`**:
   - Returns a slice of `childJobsList.get(parentId)` from index 0 to `childJobsVisibleCount.get(parentId)`
   - Handles cases where visible count exceeds total children
   - This function will be called in the template to get the subset to render

**4. Add helper function `loadMoreChildJobs(parentId)`**:
   - Increases `childJobsVisibleCount` for the given parent by `childJobsPageSize`
   - Caps at the total number of children available
   - Triggers re-render via `this.renderJobs()`

**5. Modify child jobs rendering section** (lines 262-294):
   - Change the `x-for` directive from `child in childJobsList.get(item.job.id)` to `child in getVisibleChildJobs(item.job.id)`
   - This ensures only the visible subset is rendered in the DOM

**6. Add "Load More" button after the child jobs list** (after line 292):
   - Add a conditional template that shows when `childJobsVisibleCount.get(item.job.id) < childJobsList.get(item.job.id).length`
   - Button text: "Load More (showing X of Y)"
   - Button click handler: `@click.stop="loadMoreChildJobs(item.job.id)"`
   - Style the button to match existing Bulma CSS patterns

**7. Update `handleChildSpawned()` function** (lines 1303-1328):
   - When a new child is added to the list, check if the parent's visible count needs initialization
   - If the parent is expanded and visible count is not set, initialize it to `childJobsPageSize`
   - Ensure new children appear in the visible set if there's room

**8. Update header text** (line 266):
   - Change from `'Spawned URLs (' + childJobsList.get(item.job.id).length + ')'`
   - To: `'Spawned URLs (showing ' + getVisibleChildJobs(item.job.id).length + ' of ' + childJobsList.get(item.job.id).length + ')'`
   - This provides clear feedback about pagination state

**Benefits of this approach**:
- **Simple implementation**: No complex virtual scrolling library needed
- **User-friendly**: Clear "Load More" button with progress indicator
- **Performance**: Only renders 25 children initially, reducing DOM size by 87% (25 vs 200)
- **WebSocket compatible**: Real-time updates continue to work, new children appear at the top
- **Progressive disclosure**: Users can load more if needed, but most won't need to
- **Memory efficient**: Fetches 500 instead of 1000, but still enough for most use cases

**Alternative considered - Virtual scrolling**:
- Pros: Better performance with very large lists, smooth scrolling
- Cons: Complex implementation, requires scroll container height calculations, harder to maintain, potential issues with WebSocket updates
- Decision: "Load More" is simpler and sufficient for this use case