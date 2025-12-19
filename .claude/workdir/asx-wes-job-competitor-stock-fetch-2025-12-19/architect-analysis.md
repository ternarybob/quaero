# ARCHITECT ANALYSIS - Dynamic Competitor Stock Data Spawning

## Task Overview (REVISED)
1. Create a new job definition for ASX:WES (Wesfarmers)
2. Optimize summary prompts for signal/noise focus and data accuracy
3. **Add DYNAMIC competitor stock data fetching via child job spawning**

## Requirement Clarification
The competitor analysis step should:
1. Use LLM to analyze the target company and identify competitors
2. **Spawn child jobs** to fetch stock data for each identified competitor
3. Wait for child jobs to complete
4. Use fetched competitor data in the final summary

This is NOT a static pattern - competitors are discovered dynamically.

## Existing Spawn Pattern Analysis

### Workers That Spawn Child Jobs
1. **CrawlerWorker** (`crawler_worker.go:1687`)
   - `spawnChildJob()` method
   - Uses `w.jobMgr.CreateChildJob(ctx, parentID, jobType, phase, payload)`
   - Sets `ReturnsChildJobs() = true`

2. **GitHubGitWorker** (`github_git_worker.go:702`)
   - Returns `ReturnsChildJobs() = true`
   - Creates batched child jobs for repository files

3. **TestJobGeneratorWorker** (`test_job_generator_worker.go:163`)
   - `spawnChildJob()` method for testing spawn behavior

### Key Spawn Requirements
1. Worker must return `ReturnsChildJobs() = true`
2. Use `w.jobMgr.CreateChildJob(ctx, parentID, jobType, phase, payload)` to spawn
3. Orchestrator monitors children via `StepMonitor` if `ReturnsChildJobs() = true`
4. Children complete before step is marked complete

## Design Decision

### Option A: New WorkerType for Competitor Analysis (SELECTED)
Create `WorkerTypeCompetitorAnalysis` that:
1. Takes a target ASX code and prompt
2. Uses LLM to identify competitor codes
3. Spawns `asx_stock_data` child jobs for each competitor
4. Returns `ReturnsChildJobs() = true` so orchestrator waits

**Justification for CREATE (not EXTEND)**:
- Summary worker is synchronous (`ReturnsChildJobs() = false`)
- Changing summary to spawn would break existing jobs
- This is a distinct workflow: analyze → identify → spawn → collect
- Follows existing spawn patterns (CrawlerWorker, GitHubGitWorker)

### Option B: Extend Summary Worker (REJECTED)
- Would require major changes to summary worker
- Risk breaking existing summary steps
- Summary is designed to be synchronous

## Files to Create/Modify

### CREATE (with justification)
1. **`internal/queue/workers/competitor_analysis_worker.go`**
   - New worker that identifies competitors and spawns stock data jobs
   - Follows CrawlerWorker spawn pattern
   - Implements `DefinitionWorker` interface
   - Returns `ReturnsChildJobs() = true`

### MODIFY
1. **`internal/models/worker_type.go`**
   - Add `WorkerTypeCompetitorAnalysis WorkerType = "competitor_analysis"`
   - Update `IsValid()` switch
   - Update `AllWorkerTypes()` slice

2. **`internal/app/workers.go`** (or equivalent)
   - Register new worker with step manager

3. **`bin/job-definitions/web-search-asx-wes.toml`**
   - Replace static competitor steps with single `competitor_analysis` step
   - Update summary prompt to reference spawned competitor data

## Implementation Pattern

```go
// CompetitorAnalysisWorker spawns asx_stock_data jobs for competitors
type CompetitorAnalysisWorker struct {
    jobMgr          *queue.Manager
    documentStorage interfaces.DocumentStorage
    kvStorage       interfaces.KeyValueStorage
    logger          arbor.ILogger
}

func (w *CompetitorAnalysisWorker) ReturnsChildJobs() bool {
    return true // Orchestrator will wait for child jobs
}

func (w *CompetitorAnalysisWorker) CreateJobs(ctx, step, jobDef, stepID, initResult) (string, error) {
    // 1. Use LLM to identify competitors from step config
    competitors := w.identifyCompetitors(ctx, targetASXCode, prompt)

    // 2. Spawn asx_stock_data jobs for each competitor
    for _, code := range competitors {
        w.jobMgr.CreateChildJob(ctx, stepID, "asx_stock_data", "fetch", map[string]interface{}{
            "asx_code": code,
            "period": "Y1",
            "output_tags": []string{targetTag, "competitors"},
        })
    }

    return stepID, nil // Orchestrator waits for children
}
```

## Job Definition Structure

```toml
# Step: Analyze competitors and spawn stock data fetches
[step.analyze_competitors]
type = "competitor_analysis"
description = "Identify retail sector competitors and fetch their stock data"
depends = "search_competitors"
on_error = "continue"
asx_code = "WES"
api_key = "{google_gemini_api_key}"
prompt = "Identify the top 3-5 ASX-listed competitors for WES in the retail sector"
output_tags = ["wes", "asx-wes-competitors"]

# Step: Generate final summary (waits for competitor data)
[step.summarize_results]
type = "summary"
depends = "analyze_competitors"  # Will have competitor stock data available
filter_tags = ["wes"]
...
```

## Build Verification
Build script required after Go code changes:
- `./scripts/build.sh`

## Anti-Creation Compliance

**CREATE Justification for `competitor_analysis_worker.go`**:
1. No existing worker combines LLM analysis + child job spawning
2. Summary worker cannot be extended (synchronous by design)
3. Follows established spawn patterns from CrawlerWorker
4. Minimum viable implementation - only what's needed
5. Uses existing `CreateChildJob` infrastructure

This follows the refactoring skill: CREATE is acceptable when EXTEND is not possible.
