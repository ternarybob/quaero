I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Problem Analysis

The user reports three issues with the crawler job system:

1. **"Crawler" parent jobs are redundant and confusing** - Jobs with `source_type="crawler"` appear in the UI but don't represent actual work
2. **Need visibility into job failure reasons** - Jobs can fail in multiple ways but failure reasons aren't surfaced in the UI
3. **Failure reasons must be available in the UI** - Logs are stored by job_id but not easily accessible

### Root Cause Investigation

After exploring the codebase, I found:

**Valid Source Types** (from `internal/models/source.go`):
- `SourceTypeJira = "jira"`
- `SourceTypeConfluence = "confluence"`  
- `SourceTypeGithub = "github"`

**Job Definition Types** (from `internal/models/job_definition.go`):
- `JobTypeCrawler = "crawler"` (workflow type)
- `JobTypeSummarizer = "summarizer"`
- `JobTypeCustom = "custom"`

**The Confusion**: Jobs are appearing with `source_type="crawler"` (the job definition type) instead of the actual source type (jira/confluence/github). This is incorrect - the `source_type` field in `CrawlJob` should always be one of the valid source types.

**Current Flow** (from `internal/services/jobs/job_helper.go` line 308):
```
StartCrawl(source.Type, entityType, seedURLs, ...)
```
This correctly passes `source.Type` which should be "jira", "confluence", or "github".

**UI Filtering** (from `pages/queue.html` lines 376-387):
The filter modal only shows checkboxes for "jira" and "confluence" - there's no "crawler" option, confirming it's not a valid source type.

### Likely Causes

1. **Legacy data** - Old jobs created before proper source type validation
2. **Parent jobs without sources** - Jobs created for orchestration that don't have an associated source
3. **Test/manual jobs** - Jobs created directly without going through proper source configuration

### Approach

## Solution Strategy

### Phase 1: Fix Source Type Confusion (Current Phase)

**Goal**: Ensure all jobs have meaningful source types and hide/filter parent orchestration jobs

**Approach**:
1. **Backend validation** - Add validation in `StartCrawl()` to reject invalid source types
2. **UI filtering** - Filter out jobs with `source_type="crawler"` or empty source types from the main job list
3. **Parent job identification** - Use `parent_id` field (empty = parent job) rather than source_type to identify parent jobs
4. **Display logic** - Show parent jobs only when they have valid source types OR when explicitly viewing job hierarchy

**Key Insight**: Parent jobs should inherit the source type from their associated source configuration. If a job has no source, it shouldn't appear in the main queue view unless explicitly requested.

### Phases 2 & 3: Failure Reason Visibility

These are handled by other engineers and will:
- Populate the `Error` field in `CrawlJob` model when failures occur
- Surface error messages in the UI with filtering by log level

### Reasoning

I explored the codebase structure by:
1. Reading the three files mentioned by the user: `service.go`, `job_handler.go`, and `queue.html`
2. Tracing the `StartCrawl()` method to understand how source types are passed
3. Examining the models to understand valid source types vs job definition types
4. Reviewing the UI filter modal to see what source types are expected
5. Searching for references to "crawler" as a source type to find the root cause

## Mermaid Diagram

sequenceDiagram
    participant UI as Queue UI
    participant Handler as JobHandler
    participant Helper as job_helper.go
    participant Crawler as CrawlerService
    participant DB as JobStorage

    Note over UI,DB: Current Flow (with bug)
    UI->>Handler: GET /api/jobs
    Handler->>DB: ListJobs()
    DB-->>Handler: Jobs (including source_type="crawler")
    Handler-->>UI: Jobs with invalid source types
    UI->>UI: Display confusing "crawler" jobs

    Note over UI,DB: Fixed Flow
    UI->>Handler: GET /api/jobs
    Handler->>DB: ListJobs()
    DB-->>Handler: All jobs
    Handler->>Handler: Filter out invalid source types
    Note right of Handler: Exclude: empty, "crawler",<br/>or non-standard types
    Handler-->>UI: Only valid source types
    UI->>UI: Display meaningful jobs only

    Note over Helper,Crawler: Validation at Job Creation
    Helper->>Helper: Validate source.Type
    alt Invalid source type
        Helper-->>Helper: Return error
    else Valid source type
        Helper->>Crawler: StartCrawl(source.Type, ...)
        Crawler->>Crawler: Validate sourceType again
        alt Invalid
            Crawler-->>Helper: Error
        else Valid
            Crawler->>DB: SaveJob()
        end
    end

## Proposed File Changes

### internal\services\crawler\service.go(MODIFY)

References: 

- internal\models\source.go

**Add source type validation in `StartCrawl()` method (around line 263)**

Before creating the job (before line 295), add validation to ensure `sourceType` is one of the valid source types defined in `internal/models/source.go`:
- Check if `sourceType` is one of: `models.SourceTypeJira`, `models.SourceTypeConfluence`, or `models.SourceTypeGithub`
- If invalid, return an error: `fmt.Errorf("invalid source type: %s (must be one of: jira, confluence, github)", sourceType)`
- Log the validation failure using `contextLogger.Error()` before returning

This prevents jobs from being created with invalid source types like "crawler" at the source.

### internal\handlers\job_handler.go(MODIFY)

References: 

- internal\models\source.go

**Update `ListJobsHandler()` to filter out invalid source types (around line 112)**

After fetching jobs from `h.jobManager.ListJobs(ctx, opts)` but before processing them:
- Filter the jobs slice to exclude jobs with invalid source types
- Invalid source types are: empty string, "crawler", or any value not in `["jira", "confluence", "github"]`
- For parent jobs (where `parent_id` is empty), only show them if they have a valid source type
- Log filtered jobs at DEBUG level: `h.logger.Debug().Int("filtered_count", count).Msg("Filtered jobs with invalid source types")`

This ensures the API doesn't return confusing jobs to the UI.

**Alternative approach**: Add a query parameter `include_invalid_sources=true` to allow viewing these jobs for debugging purposes, but default to filtering them out.

### pages\queue.html(MODIFY)

**Update client-side filtering to exclude invalid source types (around line 548)**

In the `matchesActiveFilters()` function:
- Add a check before the existing source filter logic
- If `job.source_type` is empty, "crawler", or not in `["jira", "confluence", "github"]`, return `false` to filter it out
- This provides defense-in-depth filtering even if backend filtering is bypassed

**Update the source type display (around line 215)**

In the job card subtitle that displays source type:
- Change from: `'Source: ' + (item.job.source_type || 'N/A')`
- To: Display a user-friendly label based on source type:
  - "jira" → "Jira"
  - "confluence" → "Confluence"
  - "github" → "GitHub"
  - Invalid/empty → "Unknown Source" (with a warning icon)

**Add a visual indicator for parent jobs (around line 209)**

For parent jobs (where `item.type === 'parent'`):
- If the parent has a valid source type, show it normally
- If the parent has an invalid source type, add a warning badge: `<span class="label label-warning">ORCHESTRATION</span>`
- This helps users understand that some parent jobs are for workflow orchestration rather than actual crawling

### internal\services\jobs\job_helper.go(MODIFY)

References: 

- internal\models\source.go
- internal\services\crawler\service.go(MODIFY)

**Add defensive validation in `StartCrawlJob()` (around line 74)**

Before calling `crawlerService.StartCrawl()` at line 307:
- Validate that `source.Type` is not empty and is one of the valid source types from `internal/models/source.go`
- If invalid, return an error: `fmt.Errorf("invalid source type '%s' for source %s: must be one of: jira, confluence, github", source.Type, source.ID)`
- Log the validation failure: `logger.Error().Str("source_id", source.ID).Str("source_type", source.Type).Msg("Invalid source type detected")`

This provides an additional validation layer before jobs are created, catching configuration errors early.

**Reference**: The valid source types are defined as constants in `internal/models/source.go` (lines 9-13): `SourceTypeJira`, `SourceTypeConfluence`, `SourceTypeGithub`.