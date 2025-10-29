I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The codebase uses a **flat parent-child hierarchy** where:
- Parent jobs have `Type: "parent"` and empty `ParentID`
- Child jobs have `Type: "crawler_url"` and `ParentID` pointing to the parent job ID
- The `parent_id` column already exists in the database (Phase 1 completed)
- `GetChildJobs()` method exists but is not exposed via API
- No aggregate statistics calculation exists yet

**Current API response structure:**
```json
{
  "jobs": [...],
  "total_count": 150,
  "limit": 50,
  "offset": 0
}
```

**Key findings:**
1. All handlers use simple `map[string]interface{}` for responses
2. SQL aggregation patterns use `COUNT(*)` and `GROUP BY`
3. The UI (queue.html) currently displays jobs as flat cards
4. `ListOptions` struct is shared across document and job listings
5. Child jobs reference parent via `msg.ParentID` throughout the worker code

**Design constraints:**
- Must maintain backward compatibility (default response unchanged)
- Should use efficient SQL aggregation for statistics
- Need to support both flat (with stats) and grouped response modes

### Approach

Add parent-child filtering and aggregate statistics to the job listing API by:

1. **Extend `ListOptions`** with `ParentID` and `Grouped` fields
2. **Create SQL aggregation helper** `GetJobChildStats()` to efficiently calculate child counts and status aggregates using a single query with LEFT JOIN and GROUP BY
3. **Update storage layer** to support `ParentID` filtering in `ListJobs()`
4. **Enhance handler response** to include statistics fields on each job object
5. **Add grouped mode** that returns parent jobs with their children nested

The solution uses SQL aggregation for performance and maintains backward compatibility by defaulting to flat list mode.

### Reasoning

I explored the codebase structure by reading the main files (`job_handler.go`, `manager.go`, `job_storage.go`), examined the `ListOptions` interface, reviewed SQL aggregation patterns in other storage files, analyzed the queue.html UI expectations, studied the `JobMessage` structure to understand parent-child relationships, and traced how `ParentID` is used throughout the worker code to update parent job progress.

## Mermaid Diagram

sequenceDiagram
    participant UI as Queue UI
    participant Handler as JobHandler
    participant Manager as JobManager
    participant Storage as JobStorage
    participant DB as SQLite Database

    Note over UI,DB: Scenario 1: Flat List with Statistics (Default)
    UI->>Handler: GET /api/jobs?limit=50&offset=0
    Handler->>Manager: ListJobs(opts)
    Manager->>Storage: ListJobs(opts)
    Storage->>DB: SELECT * FROM crawl_jobs<br/>ORDER BY created_at DESC<br/>LIMIT 50
    DB-->>Storage: [job1, job2, ...]
    Storage-->>Manager: []*CrawlJob
    Manager-->>Handler: []*CrawlJob
    
    Note over Handler: Extract parent job IDs
    Handler->>Manager: GetJobChildStats([parent_ids])
    Manager->>Storage: GetJobChildStats([parent_ids])
    Storage->>DB: SELECT parent_id, COUNT(*),<br/>SUM(CASE WHEN status='completed'...)<br/>FROM crawl_jobs<br/>WHERE parent_id IN (?, ?)<br/>GROUP BY parent_id
    DB-->>Storage: {parent1: {count:10, completed:5}, ...}
    Storage-->>Manager: map[string]*JobChildStats
    Manager-->>Handler: map[string]*JobChildStats
    
    Note over Handler: Enrich jobs with statistics
    Handler->>Handler: For each job, add child_count,<br/>completed_children, failed_children
    Handler-->>UI: {jobs: [...enriched...], total_count: 150}

    Note over UI,DB: Scenario 2: Grouped Mode
    UI->>Handler: GET /api/jobs?grouped=true
    Handler->>Manager: ListJobs(opts)
    Manager->>Storage: ListJobs(opts)
    Storage->>DB: SELECT * FROM crawl_jobs
    DB-->>Storage: [parent1, child1, child2, parent2, ...]
    Storage-->>Manager: []*CrawlJob
    Manager-->>Handler: []*CrawlJob
    
    Handler->>Manager: GetJobChildStats([parent_ids])
    Manager->>Storage: GetJobChildStats([parent_ids])
    Storage->>DB: SELECT parent_id, COUNT(*), ...<br/>GROUP BY parent_id
    DB-->>Storage: {parent1: {count:2, ...}, ...}
    Storage-->>Manager: map[string]*JobChildStats
    Manager-->>Handler: map[string]*JobChildStats
    
    Note over Handler: Group jobs by parent-child
    Handler->>Handler: Build groups array:<br/>[{parent: parent1, children: [child1, child2]}, ...]
    Handler-->>UI: {groups: [...], total_count: 150}

    Note over UI,DB: Scenario 3: Filter by Parent
    UI->>Handler: GET /api/jobs?parent_id=parent-123
    Handler->>Manager: ListJobs(opts with ParentID)
    Manager->>Storage: ListJobs(opts)
    Storage->>DB: SELECT * FROM crawl_jobs<br/>WHERE parent_id = 'parent-123'
    DB-->>Storage: [child1, child2, ...]
    Storage-->>Manager: []*CrawlJob (only children)
    Manager-->>Handler: []*CrawlJob
    Handler-->>UI: {jobs: [children only], total_count: 100}

## Proposed File Changes

### internal\interfaces\document_service.go(MODIFY)

**Add `ParentID` and `Grouped` fields to `ListOptions` struct:**

In the `ListOptions` struct definition (lines 74-83), add two new fields after `EntityType`:

```go
ParentID string // Filter by parent job ID (empty = no filter, "root" = only root jobs, specific ID = children of that parent)
Grouped  bool   // Whether to group jobs by parent-child relationship (default: false for flat list)
```

**Field documentation:**
- `ParentID`: When empty, returns all jobs. When set to a specific job ID, returns only children of that parent. When set to "root", returns only root jobs (jobs with empty parent_id).
- `Grouped`: When true, the handler will return a grouped response structure with parents and their children nested. When false (default), returns a flat list with statistics fields added to each job.

**Backward compatibility:**
These fields are optional and default to empty/false, ensuring existing API calls continue to work without changes.

### internal\storage\sqlite\job_storage.go(MODIFY)

References: 

- internal\interfaces\document_service.go(MODIFY)

**Update `ListJobs()` to support `ParentID` filtering:**

In the `ListJobs()` method (lines 214-294), add filtering logic for `ParentID` after the existing filters (around line 260):

1. **Add ParentID filter condition:**
   ```go
   if opts.ParentID != "" {
       if opts.ParentID == "root" {
           query += " AND (parent_id IS NULL OR parent_id = '')"
       } else {
           query += " AND parent_id = ?"
           args = append(args, opts.ParentID)
       }
   }
   ```

2. **Position:** Add this after the `EntityType` filter (line 259) and before the ORDER BY clause (line 263).

3. **Logic explanation:**
   - When `ParentID == "root"`: Returns only parent jobs (jobs with no parent)
   - When `ParentID == <specific_id>`: Returns only children of that parent
   - When `ParentID == ""`: No filtering (returns all jobs)

**Add `GetJobChildStats()` helper method:**

Add a new method after `GetChildJobs()` (around line 343) to calculate aggregate child statistics:

```go
func (s *JobStorage) GetJobChildStats(ctx context.Context, parentIDs []string) (map[string]*JobChildStats, error)
```

**Method implementation:**

1. **Define return type** (add to file top or as inline struct):
   ```go
   type JobChildStats struct {
       ChildCount         int
       CompletedChildren  int
       FailedChildren     int
   }
   ```

2. **SQL query using aggregation:**
   ```sql
   SELECT 
       parent_id,
       COUNT(*) as child_count,
       SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed_children,
       SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_children
   FROM crawl_jobs
   WHERE parent_id IN (?, ?, ...)
   GROUP BY parent_id
   ```

3. **Build IN clause dynamically:**
   - Create placeholders for each parent ID: `(?, ?, ?)`
   - Build args slice with parent IDs
   - Handle empty parentIDs slice by returning empty map

4. **Scan results into map:**
   - Use `map[string]*JobChildStats` keyed by parent_id
   - Scan each row: `&parentID, &childCount, &completedChildren, &failedChildren`
   - Store in map: `stats[parentID] = &JobChildStats{...}`

5. **Return map:**
   - Jobs without children will not appear in map (caller should check existence)
   - Empty map if no children found

6. **Error handling:**
   - Return error if query fails
   - Log query execution with parent ID count

**Performance considerations:**
- Single query for all parent IDs (batch operation)
- Uses existing `idx_jobs_parent_id` index for efficient lookups
- Aggregation happens in database (faster than application-level loops)

**Update `CountJobsWithFilters()` to support `ParentID`:**

In the `CountJobsWithFilters()` method (lines 479-525), add the same `ParentID` filtering logic after the `EntityType` filter (around line 519):

```go
if opts.ParentID != "" {
    if opts.ParentID == "root" {
        query += " AND (parent_id IS NULL OR parent_id = '')"
    } else {
        query += " AND parent_id = ?"
        args = append(args, opts.ParentID)
    }
}
```

This ensures the count matches the filtered list results.

### internal\jobs\manager.go(MODIFY)

References: 

- internal\storage\sqlite\job_storage.go(MODIFY)

**Add `GetJobChildStats()` method to Manager:**

Add a new method after `CountJobs()` (around line 104) to expose child statistics calculation:

```go
func (m *Manager) GetJobChildStats(ctx context.Context, parentIDs []string) (map[string]*JobChildStats, error)
```

**Method implementation:**

1. **Type definition:**
   - Define `JobChildStats` struct at package level (after imports, around line 12):
   ```go
   // JobChildStats holds aggregate statistics for a parent job's children
   type JobChildStats struct {
       ChildCount        int `json:"child_count"`
       CompletedChildren int `json:"completed_children"`
       FailedChildren    int `json:"failed_children"`
   }
   ```

2. **Delegate to storage:**
   ```go
   stats, err := m.jobStorage.GetJobChildStats(ctx, parentIDs)
   if err != nil {
       return nil, fmt.Errorf("failed to get child stats: %w", err)
   }
   return stats, nil
   ```

3. **Add logging:**
   - Log the number of parent IDs queried
   - Log the number of parents with children found
   - Use debug level: `m.logger.Debug().Int("parent_count", len(parentIDs)).Int("stats_count", len(stats)).Msg("Retrieved child statistics")`

**Purpose:**
This method acts as a pass-through to the storage layer, maintaining the separation of concerns. The Manager layer can add business logic or caching in the future if needed.

**Note:**
No changes needed to `ListJobs()` method - it already delegates to storage, which now supports `ParentID` filtering via the updated `ListOptions`.

### internal\handlers\job_handler.go(MODIFY)

References: 

- internal\jobs\manager.go(MODIFY)
- internal\models\crawler_job.go

**Update `ListJobsHandler()` to support parent filtering, statistics, and grouped mode:**

In the `ListJobsHandler()` method (lines 50-127), make the following changes:

**1. Parse new query parameters (around line 63):**

After parsing `entityType` (line 61), add:
```go
parentID := r.URL.Query().Get("parent_id")
groupedStr := r.URL.Query().Get("grouped")
grouped := false
if groupedStr == "true" {
    grouped = true
}
```

**2. Add to ListOptions (around line 88-96):**

Add the new fields to the `opts` struct:
```go
opts := &interfaces.ListOptions{
    Limit:      limit,
    Offset:     offset,
    Status:     status,
    SourceType: sourceType,
    EntityType: entityType,
    ParentID:   parentID,  // NEW
    Grouped:    grouped,   // NEW
    OrderBy:    orderBy,
    OrderDir:   orderDir,
}
```

**3. Fetch jobs (line 98):**

No changes needed - existing call to `h.jobManager.ListJobs(ctx, opts)` will now use the new filters.

**4. Calculate child statistics (after line 103, before masking):**

Add logic to fetch child statistics for all jobs:
```go
// Extract parent job IDs for statistics calculation
parentJobIDs := make([]string, 0)
for _, job := range jobs {
    // Only calculate stats for parent jobs (jobs with no parent_id)
    if job.ParentID == "" {
        parentJobIDs = append(parentJobIDs, job.ID)
    }
}

// Fetch child statistics in batch
var childStatsMap map[string]*jobs.JobChildStats
if len(parentJobIDs) > 0 {
    var err error
    childStatsMap, err = h.jobManager.GetJobChildStats(ctx, parentJobIDs)
    if err != nil {
        h.logger.Warn().Err(err).Msg("Failed to get child statistics, continuing without stats")
        childStatsMap = make(map[string]*jobs.JobChildStats)
    }
} else {
    childStatsMap = make(map[string]*jobs.JobChildStats)
}
```

**5. Create enriched job response type (after line 106):**

Define a helper function to enrich jobs with statistics:
```go
// enrichJobWithStats adds child statistics to a job
enrichJobWithStats := func(job *models.CrawlJob) map[string]interface{} {
    enriched := map[string]interface{}{
        // Copy all job fields via JSON marshaling (preserves all fields)
    }
    
    // Add statistics fields
    if stats, exists := childStatsMap[job.ID]; exists {
        enriched["child_count"] = stats.ChildCount
        enriched["completed_children"] = stats.CompletedChildren
        enriched["failed_children"] = stats.FailedChildren
    } else {
        enriched["child_count"] = 0
        enriched["completed_children"] = 0
        enriched["failed_children"] = 0
    }
    
    return enriched
}
```

**Alternative approach (simpler):**
Instead of creating a map, add fields directly to the JSON response by creating a wrapper struct or using json.RawMessage. However, since `CrawlJob` is a defined struct, the cleanest approach is to create an anonymous struct or map for the response.

**6. Build response based on mode (replace lines 106-126):**

**For flat mode (default):**
```go
if !grouped {
    // Mask sensitive data and enrich with statistics
    enrichedJobs := make([]map[string]interface{}, 0, len(jobs))
    for _, job := range jobs {
        masked := job.MaskSensitiveData()
        
        // Convert to map and add statistics
        jobMap := convertJobToMap(masked) // Helper function to convert struct to map
        
        // Add child statistics
        if stats, exists := childStatsMap[masked.ID]; exists {
            jobMap["child_count"] = stats.ChildCount
            jobMap["completed_children"] = stats.CompletedChildren
            jobMap["failed_children"] = stats.FailedChildren
        } else {
            jobMap["child_count"] = 0
            jobMap["completed_children"] = 0
            jobMap["failed_children"] = 0
        }
        
        enrichedJobs = append(enrichedJobs, jobMap)
    }
    
    response := map[string]interface{}{
        "jobs":        enrichedJobs,
        "total_count": totalCount,
        "limit":       limit,
        "offset":      offset,
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
    return
}
```

**For grouped mode:**
```go
// Group jobs by parent
groupsMap := make(map[string]*JobGroup)
orphanJobs := make([]*models.CrawlJob, 0)

for _, job := range jobs {
    if job.ParentID == "" {
        // This is a parent job
        if _, exists := groupsMap[job.ID]; !exists {
            groupsMap[job.ID] = &JobGroup{
                Parent:   job,
                Children: make([]*models.CrawlJob, 0),
            }
        }
    } else {
        // This is a child job
        if group, exists := groupsMap[job.ParentID]; exists {
            group.Children = append(group.Children, job)
        } else {
            // Parent not in current page, treat as orphan
            orphanJobs = append(orphanJobs, job)
        }
    }
}

// Convert to array and enrich with statistics
groups := make([]map[string]interface{}, 0, len(groupsMap))
for parentID, group := range groupsMap {
    maskedParent := group.Parent.MaskSensitiveData()
    parentMap := convertJobToMap(maskedParent)
    
    // Add statistics
    if stats, exists := childStatsMap[parentID]; exists {
        parentMap["child_count"] = stats.ChildCount
        parentMap["completed_children"] = stats.CompletedChildren
        parentMap["failed_children"] = stats.FailedChildren
    } else {
        parentMap["child_count"] = 0
        parentMap["completed_children"] = 0
        parentMap["failed_children"] = 0
    }
    
    // Mask children
    maskedChildren := make([]*models.CrawlJob, 0, len(group.Children))
    for _, child := range group.Children {
        maskedChildren = append(maskedChildren, child.MaskSensitiveData())
    }
    
    groups = append(groups, map[string]interface{}{
        "parent":   parentMap,
        "children": maskedChildren,
    })
}

response := map[string]interface{}{
    "groups":      groups,
    "orphans":     orphanJobs, // Jobs whose parent is not in current page
    "total_count": totalCount,
    "limit":       limit,
    "offset":      offset,
}

w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(response)
```

**7. Add helper function `convertJobToMap()` (after the handler method, around line 128):**

```go
// convertJobToMap converts a CrawlJob struct to a map for JSON response enrichment
func convertJobToMap(job *models.CrawlJob) map[string]interface{} {
    // Marshal to JSON then unmarshal to map to preserve all fields and JSON tags
    data, err := json.Marshal(job)
    if err != nil {
        return map[string]interface{}{"id": job.ID, "error": "failed to serialize job"}
    }
    
    var jobMap map[string]interface{}
    if err := json.Unmarshal(data, &jobMap); err != nil {
        return map[string]interface{}{"id": job.ID, "error": "failed to deserialize job"}
    }
    
    return jobMap
}
```

**8. Add helper type `JobGroup` (at package level, around line 23):**

```go
// JobGroup represents a parent job with its children
type JobGroup struct {
    Parent   *models.CrawlJob
    Children []*models.CrawlJob
}
```

**Backward compatibility:**
- Default behavior (no `grouped` parameter) returns flat list with statistics fields added
- Existing clients continue to work without changes
- New fields (`child_count`, etc.) are added to all jobs (0 for child jobs)

**Performance considerations:**
- Single batch query for all child statistics (efficient)
- Statistics only calculated for parent jobs in current page
- Grouped mode may return orphan children if parent is not in current page