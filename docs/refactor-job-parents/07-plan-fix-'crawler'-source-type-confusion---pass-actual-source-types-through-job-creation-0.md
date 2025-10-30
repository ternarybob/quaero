I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Problem Re-Analysis

After the user's clarification, I now understand the real issue:

**The \"crawler\" jobs ARE valid** - they represent **Job Definition workflows** (orchestration layer), not individual crawling tasks. The confusion arises because:

1. **Two Job Systems Coexist**:
   - **Job Definitions** (type=\"crawler\") - Workflow orchestration with pre/post steps
   - **CrawlJobs** (source_type=\"jira/confluence/github\") - Actual URL crawling tasks

2. **The Real Problem**: Parent CrawlJobs are incorrectly getting \`source_type=\"crawler\"\` instead of inheriting the actual source type (\"jira\", \"confluence\", \"github\") from their source configuration.

3. **User's Vision**: Jobs should be structured as synchronous ordered steps:
   - **Pre-check** (validation job)
   - **Crawler** (spawns multiple child URL jobs)
   - **Post-review** (status check job)

4. **Visibility Issue**: The current code structure doesn't clearly separate:
   - Workflow orchestration (Job Definitions)
   - Task execution (Queue-based jobs)
   - Pre/Post job hooks

## Root Cause

Looking at \`internal/services/jobs/job_helper.go\` line 308:
\`\`\`go
jobID, err = crawlerService.StartCrawl(
    source.Type,  // ← This SHOULD pass \"jira\", \"confluence\", or \"github\"
    entityType,
    seedURLs,
    ...
)
\`\`\`

The code is correct, but somewhere parent jobs are being created with \`source_type=\"crawler\"\` instead of the actual source type. This suggests:
- Either \`source.Type\` is being set to \"crawler\" incorrectly
- Or there's a separate code path creating jobs without proper source configuration
- Or Job Definition type is bleeding into CrawlJob source_type field

## User's Requirements

1. **Parent jobs should inherit source type** from their source configuration (jira/confluence/github)
2. **Pre/Post jobs should be explicit** in the workflow structure
3. **Code structure should be visible** - separate folders for workflows vs tasks
4. **UI should show hierarchy** - Job Definition → Pre → Crawler (with children) → Post

### Approach

## Solution Strategy

### Phase 1: Fix Source Type Inheritance in Parent Jobs

**Goal**: Ensure all parent CrawlJobs inherit the correct source type from their source configuration, not the job definition type.

**Approach**:
1. **Trace source type flow** from Job Definition → Crawler Action → StartCrawl → CrawlJob
2. **Add validation** to prevent \"crawler\" from being used as a source_type
3. **Fix any code paths** that incorrectly set source_type to the job definition type
4. **Add defensive checks** to ensure source.Type is always one of the valid source types

### Phase 2: Add Pre/Post Job Support to Job Definitions

**Goal**: Make pre-check and post-review jobs explicit in the workflow structure.

**Approach**:
1. **Extend JobDefinition model** to support pre_jobs and post_jobs arrays
2. **Update JobExecutor** to execute pre-jobs before main steps, post-jobs after
3. **Create pre-check job type** for validation (auth, connectivity, config)
4. **Create post-review job type** for status aggregation and reporting

### Phase 3: Restructure Code for Visibility

**Goal**: Separate workflow orchestration from task execution in the folder structure.

**Approach**:
1. **Create internal/workflows/** for Job Definition orchestration
2. **Keep internal/jobs/** for queue-based task execution
3. **Move executor and actions** to workflows folder
4. **Update imports** throughout the codebase

### Phase 4: Update UI to Show Workflow Hierarchy

**Goal**: Display the complete workflow hierarchy in the Queue Management UI.

**Approach**:
1. **Show Job Definition** as the top-level item
2. **Show Pre/Crawler/Post steps** as expandable sections
3. **Show child URL jobs** under the Crawler step
4. **Add visual indicators** for workflow vs task jobs

### Reasoning

I explored the codebase by:
1. Reading the user's clarification about pre/post job structure
2. Examining \`internal/jobs/types/crawler.go\` to understand queue-based jobs
3. Reviewing \`internal/services/jobs/executor.go\` for Job Definition orchestration
4. Tracing the source type flow from Job Definitions through to CrawlJobs
5. Understanding the distinction between workflow orchestration and task execution

## Mermaid Diagram

sequenceDiagram
    participant UI as Queue UI
    participant Executor as JobExecutor
    participant PreJob as Pre-Check Job
    participant CrawlAction as Crawl Action
    participant CrawlerSvc as CrawlerService
    participant PostJob as Post-Review Job
    participant DB as JobStorage

    Note over UI,DB: Job Definition Workflow Execution

    UI->>Executor: Execute Job Definition<br/>(type=\"crawler\")
    
    Note over Executor: Phase 1: Pre-Jobs
    Executor->>PreJob: Execute Pre-Check
    PreJob->>PreJob: Validate source config
    PreJob->>PreJob: Check authentication
    PreJob->>PreJob: Verify connectivity
    PreJob-->>Executor: Pre-check passed ✓

    Note over Executor: Phase 2: Main Steps
    Executor->>CrawlAction: Execute \"crawl\" action
    CrawlAction->>CrawlAction: Validate source.Type<br/>(must be jira/confluence/github)
    
    alt Invalid source type (e.g., \"crawler\")
        CrawlAction-->>Executor: Error: Invalid source type
    else Valid source type (e.g., \"jira\")
        CrawlAction->>CrawlerSvc: StartCrawl(source.Type=\"jira\", ...)
        CrawlerSvc->>CrawlerSvc: Validate sourceType != \"crawler\"
        CrawlerSvc->>DB: Create Parent CrawlJob<br/>(source_type=\"jira\")
        CrawlerSvc->>DB: Enqueue Child URL Jobs<br/>(source_type=\"jira\")
        CrawlerSvc-->>CrawlAction: Job ID
        CrawlAction-->>Executor: Crawl started ✓
    end

    Note over Executor: Phase 3: Post-Jobs
    Executor->>PostJob: Execute Post-Review
    PostJob->>DB: Get job results
    PostJob->>PostJob: Aggregate statistics
    PostJob->>PostJob: Generate summary
    PostJob-->>Executor: Post-review complete ✓

    Executor-->>UI: Workflow completed

    Note over UI: Display Hierarchy:<br/>Job Definition → Pre → Crawler (with children) → Post

## Proposed File Changes

### internal\\services\\jobs\\actions\\crawler_actions.go(MODIFY)

References: 

- internal\\models\\source.go
- internal\\services\\jobs\\job_helper.go

**Fix source type inheritance in crawlAction (around line 113)**

Before calling \`startCrawlJobFunc()\`, add validation to ensure the source has a valid source type:
- Check that \`source.Type\` is not empty and is one of: \`models.SourceTypeJira\`, \`models.SourceTypeConfluence\`, or \`models.SourceTypeGithub\`
- If invalid, log an error and skip this source: \`deps.Logger.Error().Str(\"source_id\", source.ID).Str(\"source_type\", source.Type).Msg(\"Invalid source type - skipping source\")\`
- Add a defensive check to ensure we're not accidentally passing the job definition type as the source type

**Add logging to track source type flow (around line 140)**

After the job is started successfully, log the source type that was passed:
- \`deps.Logger.Info().Str(\"job_id\", jobID).Str(\"source_type\", string(source.Type)).Msg(\"Crawl job started with source type\")\`

This helps trace where source_type=\"crawler\" might be coming from.

**Reference**: Valid source types are defined in \`internal/models/source.go\` (lines 9-13).

### internal\\services\\crawler\\service.go(MODIFY)

References: 

- internal\\models\\source.go

**Add source type validation in StartCrawl() (around line 263)**

At the beginning of the \`StartCrawl()\` method, before creating the job (before line 295), add validation:
- Check if \`sourceType\` is one of the valid source types: \"jira\", \"confluence\", or \"github\"
- Reject \"crawler\" explicitly: \`if sourceType == \"crawler\" { return \"\", fmt.Errorf(\"invalid source type 'crawler': this is a job definition type, not a source type. Expected: jira, confluence, or github\") }\`
- Log the validation failure using \`contextLogger.Error()\` before returning
- This prevents the root cause of parent jobs having source_type=\"crawler\"

**Add defensive logging (around line 297)**

After creating the job struct, log the source type being used:
- \`contextLogger.Info().Str(\"source_type\", sourceType).Str(\"entity_type\", entityType).Msg(\"Creating crawl job with source type\")\`

This provides an audit trail for debugging source type issues.

**Reference**: The valid source types are defined as constants in \`internal/models/source.go\` (lines 9-13): \`SourceTypeJira\`, \`SourceTypeConfluence\`, \`SourceTypeGithub\`.

### internal\\models\\job_definition.go(MODIFY)

**Add PreJobs and PostJobs fields to JobDefinition struct (around line 94)**

Add two new fields to the \`JobDefinition\` struct:
- \`PreJobs []string \\\`json:\"pre_jobs\"\\\`\` - Array of job definition IDs to execute before main steps (validation, pre-checks)
- \`PostJobs []string \\\`json:\"post_jobs\"\\\`\` - Array of job definition IDs to execute after main steps (already exists at line 94)

Note: \`PostJobs\` already exists, but we need to add \`PreJobs\` for symmetry.

**Add validation for PreJobs (around line 145)**

In the \`Validate()\` method, add validation for the new PreJobs field:
- Ensure PreJobs array doesn't contain the job's own ID (prevent circular dependencies)
- Validate that PreJobs IDs are not empty strings

**Add MarshalPreJobs and UnmarshalPreJobs methods (after line 243)**

Add serialization methods for the PreJobs field (similar to PostJobs):
- \`MarshalPreJobs()\` - Serializes PreJobs array to JSON string for database storage
- \`UnmarshalPreJobs()\` - Deserializes PreJobs JSON string from database

These methods follow the same pattern as \`MarshalPostJobs()\` and \`UnmarshalPostJobs()\` at lines 228-243.

### internal\\services\\jobs\\executor.go(MODIFY)

References: 

- internal\\models\\job_definition.go(MODIFY)

**Add pre-job execution in ExecuteJobDefinition (around line 100)**

Before executing the main job steps, add pre-job execution logic:
- Check if \`jobDef.PreJobs\` is not empty
- For each pre-job ID in the array:
  - Load the pre-job definition from storage
  - Execute it synchronously using \`ExecuteJobDefinition()\` (recursive call)
  - If pre-job fails and error strategy is \"fail\", stop execution and return error
  - Log pre-job execution: \`e.logger.Info().Str(\"pre_job_id\", preJobID).Msg(\"Executing pre-job\")\`

**Add logging to distinguish workflow vs task jobs (around line 150)**

When executing job steps, add logging to clarify the job type:
- \`e.logger.Info().Str(\"job_def_id\", jobDef.ID).Str(\"job_def_type\", string(jobDef.Type)).Msg(\"Executing job definition workflow\")\`
- This helps distinguish Job Definition workflows (type=\"crawler\") from CrawlJobs (source_type=\"jira\")

**Update post-job execution to use the existing PostJobs field (around line 200)**

The post-job execution logic should already exist (using \`jobDef.PostJobs\`), but ensure:
- Post-jobs are executed AFTER all main steps complete successfully
- Post-jobs receive the results/status of the main job
- Post-job failures are logged but don't fail the main job (configurable)

### internal\\storage\\sqlite\\job_definition_storage.go(MODIFY)

References: 

- internal\\models\\job_definition.go(MODIFY)
- internal\\storage\\sqlite\\schema.go

**Add pre_jobs column to job_definitions table schema**

Update the table schema (in the CREATE TABLE statement or migration) to include:
- \`pre_jobs TEXT\` - JSON array of pre-job definition IDs

**Update SaveJobDefinition to persist PreJobs (around INSERT/UPDATE statement)**

When saving a job definition:
- Call \`jobDef.MarshalPreJobs()\` to serialize the PreJobs array
- Include the serialized pre_jobs in the INSERT/UPDATE statement
- Handle errors from marshaling

**Update GetJobDefinition to load PreJobs (around SELECT statement)**

When loading a job definition:
- Read the \`pre_jobs\` column from the database
- Call \`jobDef.UnmarshalPreJobs(preJobsJSON)\` to deserialize
- Handle errors from unmarshaling

**Add database migration for pre_jobs column**

Create a migration in \`internal/storage/sqlite/schema.go\` to add the \`pre_jobs\` column to existing databases:
- \`ALTER TABLE job_definitions ADD COLUMN pre_jobs TEXT DEFAULT '[]'\`
- Set default to empty JSON array for backward compatibility

### pages\\queue.html(MODIFY)

**Update job card to show source type clearly (around line 215)**

In the job card subtitle that displays source type:
- Add validation to check if \`source_type\` is valid (jira/confluence/github)
- If \`source_type\` is \"crawler\" or empty, display a warning badge: \`<span class=\"label label-warning\">WORKFLOW</span>\`
- If \`source_type\` is valid, display it with proper capitalization: \"Jira\", \"Confluence\", \"GitHub\"
- Add a tooltip explaining the difference: \"Workflow orchestration job\" vs \"Source crawling job\"

**Add visual distinction for Job Definition workflows (around line 182)**

For jobs that represent Job Definition workflows (not individual crawl tasks):
- Add a CSS class: \`job-card-workflow\`
- Use a different icon: \`<i class=\"fas fa-project-diagram\"></i>\` instead of folder/file icons
- Add a badge: \`<span class=\"label label-info\">WORKFLOW</span>\`

**Update filter modal to include workflow toggle (around line 377)**

Add a new filter section for job types:
- Checkbox for \"Show Workflow Jobs\" (Job Definitions)
- Checkbox for \"Show Task Jobs\" (Individual CrawlJobs)
- Default: Show both types

This helps users filter between orchestration workflows and actual crawling tasks.

**Reference**: The image shows jobs with source_type=\"crawler\" which should be displayed as workflow jobs, not source crawling jobs.

### internal\\handlers\\job_handler.go(MODIFY)

**Add job type enrichment in ListJobsHandler (around line 169)**

When enriching jobs with statistics, add a new field to distinguish job types:
- \`jobMap[\"is_workflow\"] = (masked.SourceType == \"\" || masked.SourceType == \"crawler\")\` - Indicates if this is a Job Definition workflow
- \`jobMap[\"is_task\"] = (masked.SourceType == \"jira\" || masked.SourceType == \"confluence\" || masked.SourceType == \"github\")\` - Indicates if this is a task job

This allows the UI to render different visual styles for workflows vs tasks.

**Add source type validation logging (around line 114)**

After fetching jobs from \`h.jobManager.ListJobs()\`, add logging to track source type distribution:
- Count jobs by source_type
- Log a warning if any jobs have source_type=\"crawler\": \`h.logger.Warn().Int(\"count\", crawlerCount).Msg(\"Found jobs with source_type='crawler' - these should have actual source types\")\`

This helps identify the root cause of the source type issue.

**Do NOT filter out jobs** - The previous plan suggested filtering, but per the user's clarification, \"crawler\" jobs are valid workflow jobs and should be displayed (just with proper labeling).