I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Key Findings:**

1. **`is_workflow` and `is_task` already removed** - No references found in any .go, .html, or .js files ✓

2. **`JobTypeParent` is still used and should be KEPT:**
   - Used in `crawler_job.go` line 26 as a constant
   - Used in `service.go` line 317 when creating crawler parent jobs
   - Used in `job_storage.go` lines 122-140 for normalization/validation
   - Used in test files for creating parent jobs
   - This is NOT the orchestration wrapper - it's for actual crawler parent jobs that spawn child URL jobs

3. **Valid JobDefinitionTypes:** `crawler`, `summarizer`, `custom` (from job_definition.go)

4. **Test files use invalid type:** `job_cascade_test.go` and `foreign_key_test.go` use `"type": "orchestration"` which is not a valid JobDefinitionType

5. **Terminology updates needed in comments:**
   - `crawler_job.go` line 26: "Root job that orchestrates workflow" → "Parent job that spawns child jobs"
   - `service.go` lines 51-91: Multiple "workflow" and "orchestration" references
   - `schema.go` line 106: "JobDefinition workflows" → "JobDefinition jobs"
   - `registry.go` lines 13-38: "workflows" → "multi-step jobs"
   - `executor.go` line 132: "workflow and task jobs" distinction should be removed
   - `crawler_actions.go` lines 236, 316: "workflow orchestration" → "job coordination"
   - `filters.go` line 12: References non-existent "orchestrator.go" file

6. **Historical references should be kept:**
   - `schema.go` migration comments (lines 398-400, 1844-1850) accurately describe the OLD orchestration pattern
   - `job_definition_handler.go` comments (lines 386, 397) explain what was removed
   - These provide valuable context for understanding the codebase evolution

7. **Legitimate "orchestrate" usage (keep as-is):**
   - `mcp/router.go` line 13: MCP tool routing (not job-related)
   - `chat/agent_loop.go` line 35: Agent conversation (not job-related)
   - `jobs/executor.go`: JobExecutor legitimately orchestrates multi-step workflows
   - `app/app.go`: Accurate description of JobExecutor's purpose

### Approach

Remove deprecated `is_workflow` and `is_task` terminology (already cleaned up), update job-related comments to replace "workflow" and "orchestration" with "job" terminology, keep `JobTypeParent` constant (used for actual crawler parent jobs), fix test files using invalid `"orchestration"` type, and verify no UI references exist. This is a documentation and terminology cleanup task with no functional changes.

### Reasoning

I explored the codebase using grep_search to find all references to `is_workflow`, `is_task`, `WORKFLOW`, `workflow`, `orchestration`, and `JobTypeParent`. I read the relevant files (job_handler.go, crawler_job.go, queue.html) and discovered that `is_workflow` and `is_task` were already removed in previous phases. I identified that `JobTypeParent` is still actively used for crawler parent jobs (not orchestration wrappers) and should be kept. I found terminology that needs updating in comments across multiple files, and test files using an invalid `"orchestration"` job definition type.

## Proposed File Changes

### internal\models\crawler_job.go(MODIFY)

References: 

- internal\services\crawler\service.go(MODIFY)
- internal\storage\sqlite\job_storage.go

**Update JobTypeParent comment to reflect current architecture:**

1. **Line 26** - Update comment:
   - Current: `JobTypeParent JobType = "parent" // Root job that orchestrates workflow`
   - New: `JobTypeParent JobType = "parent" // Parent job that spawns child jobs`

**Rationale:** The term "orchestrates workflow" is misleading - parent jobs don't orchestrate workflows, they spawn child URL jobs. The JobExecutor orchestrates workflows. This constant is used for crawler parent jobs that manage child URL crawling tasks.
**Verification: JobTypeParent constant should be kept.**

Confirm that `JobTypeParent` constant (line 26) is still used throughout the codebase for legitimate purposes:

1. **service.go line 317:** Used when creating crawler parent jobs
2. **job_storage.go lines 122-140:** Used for normalization and validation
3. **Test files:** Used for creating parent jobs in tests

**DO NOT remove this constant.** It's used for actual crawler parent jobs that spawn child URL jobs, not for the orchestration wrapper pattern that was removed.

**Only change:** Update the comment as specified in the first file change entry.

### internal\services\crawler\service.go(MODIFY)

**Update job-related terminology in architecture comments:**

1. **Line 51** - Update section header:
   - Current: `// 2. JOB DEFINITION SYSTEM (Workflow Orchestration)`
   - New: `// 2. JOB DEFINITION SYSTEM (Multi-Step Job Coordination)`

2. **Line 52** - Update description:
   - Current: `//    - Purpose: Orchestrate multi-step workflows and scheduled jobs`
   - New: `//    - Purpose: Coordinate multi-step jobs and scheduled jobs`

3. **Line 62** - Update use case:
   - Current: `//      * Defining scheduled workflows (cron jobs)`
   - New: `//      * Defining scheduled jobs (cron jobs)`

4. **Line 63** - Update use case:
   - Current: `//      * Orchestrating multi-step processes (crawl → summarize → cleanup)`
   - New: `//      * Coordinating multi-step processes (crawl → summarize → cleanup)`

5. **Line 65** - Update use case:
   - Current: `//      * Require workflow-level configuration and metadata`
   - New: `//      * Require job-level configuration and metadata`

6. **Line 69** - Update interaction description:
   - Current: `// JobExecutor (workflow) → QueueManager (task execution)`
   - New: `// JobExecutor (multi-step jobs) → QueueManager (task execution)`

7. **Line 75** - Update example header:
   - Current: `// EXAMPLE WORKFLOW:`
   - New: `// EXAMPLE JOB FLOW:`

8. **Line 83** - Update completion description:
   - Current: `// 7. Workflow completes, status persisted to database`
   - New: `// 7. Job completes, status persisted to database`

9. **Line 91** - Update design principle:
   - Current: `// - Separation of Concerns: Task execution vs. workflow orchestration`
   - New: `// - Separation of Concerns: Task execution vs. job coordination`

**Rationale:** Consistent terminology throughout the codebase. "Workflow" implies a business process orchestration system, but this is actually a job execution system. "Coordination" is more accurate than "orchestration" for describing how JobExecutor manages multi-step jobs.

### internal\storage\sqlite\schema.go(MODIFY)

**Update job-related terminology in schema comments:**

1. **Line 106** - Update table comment:
   - Current: `-- Used by both JobExecutor (for JobDefinition workflows) and queue-based jobs`
   - New: `-- Used by both JobExecutor (for JobDefinition jobs) and queue-based jobs`

**DO NOT modify migration comments (lines 398-400, 1844-1850):**
- These comments accurately describe the historical "orchestration jobs" pattern that was removed
- They provide valuable context for understanding the codebase evolution
- Changing them would make the migration history confusing

**Rationale:** The schema comment should use current terminology, but migration comments should preserve historical accuracy.

### internal\services\jobs\registry.go(MODIFY)

**Update job-related terminology in registry comments:**

1. **Line 13** - Update purpose description:
   - Current: `//   - Purpose: Orchestrates complex, multi-step workflows with configurable actions`
   - New: `//   - Purpose: Coordinates complex, multi-step jobs with configurable actions`

2. **Line 17** - Update use case:
   - Current: `//   - Used for: User-defined scheduled jobs, batch processing workflows`
   - New: `//   - Used for: User-defined scheduled jobs, batch processing jobs`

3. **Line 29** - Update description:
   - Current: `//     multi-step workflows (e.g., "scan all docs, then summarize, then extract keywords")`
   - New: `//     multi-step jobs (e.g., "scan all docs, then summarize, then extract keywords")`

4. **Line 38** - Update description:
   - Current: `// - JobTypeRegistry "summarizer": Multi-step workflow definition`
   - New: `// - JobTypeRegistry "summarizer": Multi-step job definition`

**Rationale:** Consistent terminology. The registry manages job definitions, not workflow definitions. "Coordinate" is more accurate than "orchestrate" for this context.

### internal\services\jobs\executor.go(MODIFY)

**Remove workflow vs task job distinction in logging:**

1. **Line 132** - Update log comment:
   - Current: `// Log execution start with distinction between workflow and task jobs`
   - New: `// Log execution start`

**Rationale:** The user explicitly stated "a job is a job, is a job" - there should be no distinction between workflow and task jobs. This comment perpetuates the old dual-job model that was removed.

### internal\services\jobs\actions\crawler_actions.go(MODIFY)

**Update job-related terminology in action comments:**

1. **Line 236** - Update transformAction comment:
   - Current: `// For wait-for-completion functionality, use a separate polling mechanism or workflow orchestration.`
   - New: `// For wait-for-completion functionality, use a separate polling mechanism or job coordination.`

2. **Line 316** - Update embedAction comment:
   - Current: `// For wait-for-completion functionality, use a separate polling mechanism or workflow orchestration.`
   - New: `// For wait-for-completion functionality, use a separate polling mechanism or job coordination.`

**Rationale:** Consistent terminology. "Job coordination" is more accurate than "workflow orchestration" in this context.

### internal\services\crawler\filters.go(MODIFY)

**Update outdated file reference in comment:**

1. **Line 12** - Update comment:
   - Current: `// Consolidates duplicate filtering from orchestrator.go and CrawlerJob.shouldEnqueueURL()`
   - New: `// Consolidates duplicate filtering from CrawlerJob.shouldEnqueueURL() and other components`

**Rationale:** The file `orchestrator.go` does not exist (verified via file_search). This comment references a file that was removed during previous refactorings. The comment should reflect the current codebase structure.

### test\api\job_cascade_test.go(MODIFY)

References: 

- internal\models\job_definition.go

**Fix invalid job definition type in test fixtures:**

1. **Line 19** - Update type field:
   - Current: `"type": "orchestration"`
   - New: `"type": "crawler"`

2. **Line 144** - Update type field:
   - Current: `"type": "orchestration"`
   - New: `"type": "crawler"`

3. **Line 301** - Update type field:
   - Current: `"type": "orchestration"`
   - New: `"type": "crawler"`

4. **Line 421** - Update type field:
   - Current: `"type": "orchestration"`
   - New: `"type": "crawler"`

**Rationale:** `"orchestration"` is not a valid JobDefinitionType. Valid types are: `"crawler"`, `"summarizer"`, `"custom"` (defined in `internal/models/job_definition.go`). Since these tests are creating job definitions that trigger crawl jobs, `"crawler"` is the appropriate type. This ensures tests use valid data that matches production code.

### test\api\foreign_key_test.go(MODIFY)

References: 

- internal\models\job_definition.go

**Fix invalid job definition type in test fixture:**

1. **Line 18** - Update type field:
   - Current: `"type": "orchestration"`
   - New: `"type": "crawler"`

**Rationale:** `"orchestration"` is not a valid JobDefinitionType. Valid types are: `"crawler"`, `"summarizer"`, `"custom"`. Since this test creates a job definition for testing foreign key constraints, `"crawler"` is appropriate. This ensures the test uses valid data that matches production code.

### internal\handlers\job_handler.go(MODIFY)

**Verification: No changes needed.**

Confirm that no references to `is_workflow` or `is_task` exist in this file. The grep_search results show these were already removed in previous phases.

**Verification steps:**
1. Search for `is_workflow` - should return no results
2. Search for `is_task` - should return no results
3. Confirm lines 172-174 (mentioned in user's task) no longer contain enrichment logic

**Expected state:** The handler should only enrich jobs with `child_count`, `completed_children`, `failed_children`, and `status_report` fields. No job type flags should be added.

### pages\queue.html(MODIFY)

**Verification: No changes needed.**

Confirm that no references to `is_workflow` or `is_task` exist in this file. The grep_search results show no matches in any .html files.

**Verification steps:**
1. Search the file for `is_workflow` - should return no results
2. Search the file for `is_task` - should return no results
3. Search for `workflow` or `orchestration` in UI text - should return no results
4. Confirm the UI is job-agnostic (no special handling for different job types)

**Expected state:** The UI should treat all jobs uniformly, using `status_report` from the backend for display. No client-side job type discrimination should exist.