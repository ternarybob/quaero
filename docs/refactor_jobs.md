I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Key Observations

**Current Architecture:**
1. **Scheduler Service**: Simple implementation using \`robfig/cron/v3\`, runs single cron job that publishes \`EventCollectionTriggered\`
2. **Job System**: Sophisticated crawler job system exists for on-demand web crawling (stored in \`crawl_jobs\` table)
3. **Storage**: Documents have both \`content\` and \`content_markdown\` fields; FTS5 indexes \`content\` field
4. **MCP Service**: Well-structured with document service, formatters, types, and router
5. **Config System**: Uses TOML with priority: CLI > Env > File > Defaults
6. **DI Pattern**: App initialization in \`internal/app/app.go\` with constructor-based dependency injection
7. **Static Files**: Served via \`PageHandler.StaticFileHandler\` from \`pages/static/\` directory
8. **Search Service**: FTS5-based, uses \`doc.Content\` field in several places
9. **LLM Service**: Has \`Chat()\` method for generating text, can be used for summarization

**Code References Using \`doc.Content\`:**
- \`internal/services/search/fts5_search_service.go\` (line 254)
- \`internal/services/metadata/extractor.go\` (lines 50, 55)
- \`internal/services/mcp/formatters.go\` (lines 35, 88)
- \`internal/services/identifiers/extractor.go\` (line 87)
- \`internal/services/documents/document_service.go\` (line 115)
- \`internal/storage/sqlite/document_storage.go\` (multiple locations)

**Design Decisions:**
1. **Job Types**: Distinguish between \"crawler jobs\" (on-demand, user-created) and \"default jobs\" (system-managed, scheduled)
2. **Mutex Strategy**: Global mutex across all default jobs to prevent concurrent execution
3. **Storage Migration**: Copy \`content\` → \`content_markdown\`, update FTS5, drop \`content\` column
4. **Image Handling**: Store in filesystem under \`./data/images/\` and \`./data/attachments/\`, reference in markdown
5. **Job Execution**: Default jobs won't be stored in \`crawl_jobs\` table; status tracked in scheduler memory

### Approach

## Implementation Approach

**Phase 1: Configuration & Infrastructure**
Add jobs configuration to TOML and Config struct, enhance scheduler to support multiple job types with individual schedules and mutex locking, add validation for minimum 5-minute intervals.

**Phase 2: Storage Refactor (Markdown-First)**
Migrate database schema to remove \`content\` field, update FTS5 triggers to index \`content_markdown\`, update all code references to use markdown field only.

**Phase 3: Default Job Implementation**
Create \`crawl_collect_job.go\` and \`scan_summarize_job.go\` services that implement the scheduled job logic, integrate with existing crawler and LLM services.

**Phase 4: MCP Enhancement**
Update MCP tools to use \`content_markdown\` field, add metadata query tools for keywords, summaries, and similar sources.

**Phase 5: UI & API**
Enhance jobs page to display default jobs separately from crawler jobs, add API endpoints for managing default job state (enable/disable, schedule updates).

### Reasoning

Explored the codebase by reading configuration files, service implementations, storage layer, models, and interfaces. Searched for references to \`doc.Content\` field to understand migration impact. Examined scheduler service, job storage, MCP services, and UI components to understand current architecture and integration points.

## Mermaid Diagram

sequenceDiagram
    participant Config as quaero.toml
    participant Scheduler as Scheduler Service
    participant CrawlJob as Crawl & Collect Job
    participant ScanJob as Scan & Summarize Job
    participant Crawler as Crawler Service
    participant LLM as LLM Service
    participant Storage as Document Storage
    participant MCP as MCP Service
    
    Config->>Scheduler: Load job configs (enabled, schedule)
    Scheduler->>Scheduler: Register default jobs with cron
    
    Note over Scheduler: Every 5 minutes (crawl_and_collect)
    Scheduler->>Scheduler: Acquire global mutex
    Scheduler->>CrawlJob: Execute job
    CrawlJob->>Crawler: Trigger crawler for sources
    Crawler->>Storage: Store as markdown with metadata
    CrawlJob->>Scheduler: Release mutex
    
    Note over Scheduler: Every 10 minutes (scan_and_summarize)
    Scheduler->>Scheduler: Acquire global mutex
    Scheduler->>ScanJob: Execute job
    ScanJob->>Storage: Query existing markdown docs
    ScanJob->>LLM: Generate summaries
    ScanJob->>Storage: Update metadata (keywords, word counts)
    ScanJob->>Scheduler: Release mutex
    
    Note over MCP: LLM queries documents
    MCP->>Storage: Search markdown content
    Storage->>MCP: Return documents with metadata

## Proposed File Changes

### Phase 1: Configuration & Infrastructure

- [x] **deployments\\local\\quaero.toml(MODIFY)**

Add \`[jobs]\` section with two default job configurations:

\`\`\`toml
[jobs.crawl_and_collect]
enabled = true
schedule = \"*/5 * * * *\"  # Every 5 minutes
description = \"Crawl and collect website data, store as markdown\"

[jobs.scan_and_summarize]
enabled = true
schedule = \"*/10 * * * *\"  # Every 10 minutes
description = \"Scan markdown documents and generate summaries with metadata\"
\`\`\`

Add documentation comments explaining:
- Jobs cannot be removed, only enabled/disabled
- Minimum interval is 5 minutes
- Schedule uses cron format (minute hour day month weekday)
- Jobs run independently but not concurrently

- [x] **internal\\common\\config.go(MODIFY)**

References: 

- internal\\services\\scheduler\\scheduler_service.go(MODIFY)

Add \`JobsConfig\` struct and integrate into main \`Config\` struct:

1. Add new structs after \`LoggingConfig\`:
- \`JobsConfig\` with fields: \`CrawlAndCollect\`, \`ScanAndSummarize\` (both \`JobConfig\` type)
- \`JobConfig\` with fields: \`Enabled\` (bool), \`Schedule\` (string), \`Description\` (string)

2. Add \`Jobs JobsConfig\` field to \`Config\` struct (line 21)

3. In \`NewDefaultConfig()\` function (line 138), add default job configuration with enabled=true, schedule=\"*/5 * * * *\" for crawl_and_collect and \"*/10 * * * *\" for scan_and_summarize

4. Add \`ValidateJobSchedule(schedule string)\` function that:
- Parses cron expression using \`cron.NewParser()\`
- Validates minimum 5-minute interval by checking minute field
- Returns error if invalid or interval < 5 minutes

5. Add import for \`github.com/robfig/cron/v3\`

- [x] **internal\\interfaces\\scheduler_service.go(MODIFY)**

Extend \`SchedulerService\` interface to support multiple job types:

1. Add \`JobStatus\` struct before interface with fields: \`Name\`, \`Enabled\`, \`Schedule\`, \`Description\`, \`LastRun\` (*time.Time), \`NextRun\` (*time.Time), \`IsRunning\` (bool), \`LastError\` (string)

2. Add new methods to interface:
- \`RegisterJob(name string, schedule string, handler func() error) error\`
- \`EnableJob(name string) error\`
- \`DisableJob(name string) error\`
- \`GetJobStatus(name string) (*JobStatus, error)\`
- \`GetAllJobStatuses() map[string]*JobStatus\`

3. Keep existing methods for backward compatibility

- [x] **internal\\services\\scheduler\\scheduler_service.go(MODIFY)**

References: 

- internal\\interfaces\\scheduler_service.go(MODIFY)
- internal\\common\\config.go(MODIFY)

Enhance scheduler service to support multiple jobs with mutex locking:

1. Update \`Service\` struct (line 14) to add:
- \`jobMu sync.Mutex\` (protects job map)
- \`globalMu sync.Mutex\` (prevents concurrent job execution)
- \`jobs map[string]*jobEntry\`

2. Add \`jobEntry\` struct with fields: \`name\`, \`schedule\`, \`description\`, \`handler\` (func() error), \`enabled\` (bool), \`cronID\` (cron.EntryID), \`lastRun\` (*time.Time), \`nextRun\` (*time.Time), \`isRunning\` (bool), \`lastError\` (string)

3. Update \`NewService()\` to initialize \`jobs: make(map[string]*jobEntry)\`

4. Implement new methods:
- \`RegisterJob(name, schedule, handler)\`: Validate schedule, add to jobs map, schedule with cron if enabled
- \`EnableJob(name)\`: Set enabled=true, add to cron scheduler
- \`DisableJob(name)\`: Set enabled=false, remove from cron scheduler
- \`GetJobStatus(name)\`: Return job status from jobs map
- \`GetAllJobStatuses()\`: Return all job statuses

5. Create \`executeJob(name string)\` wrapper that:
- Acquires \`globalMu\` to prevent concurrent execution
- Updates job status (lastRun, isRunning)
- Calls job handler from jobs map
- Updates lastError if handler fails
- Releases mutex
- Includes panic recovery

6. Update \`Start()\` method to accept job configurations and register them

7. Keep existing \`runScheduledTask()\` for backward compatibility

### Phase 2: Default Job Implementation

- [x] **internal\\services\\jobs\\crawl_collect_job.go(NEW)**

References: 

- internal\\services\\crawler\\service.go
- internal\\services\\sources\\service.go
- internal\\handlers\\job_handler.go(MODIFY)

Create new file implementing the crawl and collect default job:

1. Define \`CrawlCollectJob\` struct with fields: \`crawlerService\` (*crawler.Service), \`sourceService\` (*sources.Service), \`authStorage\` (interfaces.AuthStorage), \`logger\` (arbor.ILogger)

2. Implement constructor \`NewCrawlCollectJob()\` that accepts dependencies and returns *CrawlCollectJob

3. Implement \`Execute() error\` method that:
- Queries enabled sources using \`sourceService.GetEnabledSources()\`
- For each source:
  - Derives seed URLs based on source type (reuse logic from \`internal/handlers/job_handler.go\` deriveSeedURLs method)
  - Creates crawler config with \`DetailLevel: \"full\"\` and markdown storage
  - Calls \`crawlerService.StartCrawl()\` with refreshSource=true
  - Waits for job completion using \`crawlerService.WaitForJob()\`
- Handles images/attachments by storing in filesystem paths from config
- References images in markdown as relative paths
- Implements upsert via existing \`(source_type, source_id)\` unique constraint
- Logs progress and aggregates errors
- Returns error if any critical failure occurs

4. Add helper methods:
- \`deriveSeedURLs(source *models.SourceConfig) []string\`: Extract seed URLs
- \`deriveEntityType(source *models.SourceConfig) string\`: Map source type to entity type

5. Add package imports: context, fmt, time, arbor, interfaces, crawler, sources, models

- [x] **internal\\services\\jobs\\scan_summarize_job.go(NEW)**

References: 

- internal\\interfaces\\llm_service.go
- internal\\interfaces\\storage.go
- internal\\services\\summary\\summary_service.go

Create new file implementing the scan and summarize default job:

1. Define \`ScanSummarizeJob\` struct with fields: \`docStorage\` (interfaces.DocumentStorage), \`llmService\` (interfaces.LLMService), \`logger\` (arbor.ILogger)

2. Implement constructor \`NewScanSummarizeJob()\` that accepts dependencies and returns *ScanSummarizeJob

3. Implement \`Execute() error\` method that:
- Queries documents in batches (100 at a time) using \`docStorage.ListDocuments()\`
- For each document:
  - Checks if already has summary in metadata (skip if present)
  - Generates summary using \`llmService.Chat()\` with system prompt requesting 2-3 sentence summary
  - Handles LLM errors gracefully (log warning, use placeholder summary)
  - Extracts metadata:
    - Word count: count words in \`ContentMarkdown\` field
    - Keywords: frequency analysis of words (exclude common stop words)
    - Similar sources: group by metadata fields like project_key, space_key
  - Updates document metadata map with: summary, word_count, keywords, last_summarized timestamp
  - Saves document using \`docStorage.UpdateDocument()\`
- Logs progress every 10 documents
- Returns aggregated error if critical failures occur

4. Add helper methods:
- \`generateSummary(content string) (string, error)\`: Call LLM or return placeholder
- \`extractKeywords(content string, topN int) []string\`: Frequency analysis with stop word filtering
- \`calculateWordCount(content string) int\`: Count words in markdown
- \`findSimilarSources(doc *models.Document, allDocs []*models.Document) []string\`: Group by metadata

5. Add package imports: context, fmt, strings, time, arbor, interfaces, models

### Phase 3: Storage Refactor (Markdown-First) ✅ COMPLETED

- [x] **internal\\storage\\sqlite\\schema.go(MODIFY)**

Update database schema to remove \`content\` field and update FTS5 triggers:

1. In \`schemaSQL\` constant (line 3), update documents table definition:
- Remove \`content TEXT NOT NULL,\` line from table schema
- Keep \`content_markdown TEXT,\` as primary content field
- Update table comment to indicate markdown is primary

2. Update FTS5 virtual table definition (line 150) to index \`content_markdown\` instead of \`content\`

3. Update FTS5 triggers (lines 158-171):
- Change \`new.content\` to \`new.content_markdown\` in insert trigger
- Change \`new.content\` to \`new.content_markdown\` in update trigger

4. In \`runMigrations()\` function (line 191), add comprehensive migration:
- Check if \`content\` column exists using PRAGMA table_info
- If exists:
  - Copy \`content\` to \`content_markdown\` where markdown is empty
  - Drop and recreate FTS5 table with \`content_markdown\` field
  - Rebuild FTS5 index
  - Create new documents table without \`content\` column
  - Copy all data to new table
  - Drop old table and rename new table
  - Recreate indexes
- Log migration progress

- [x] **internal\\models\\document.go(MODIFY)**

Remove \`Content\` field from \`Document\` struct:

1. In \`Document\` struct (line 16), remove the \`Content string\` field (line 24)

2. Keep \`ContentMarkdown\` field as the primary content field

3. Update struct comment to indicate markdown is the primary content format

4. No changes needed to metadata structs or helper methods

- [x] **internal\\storage\\sqlite\\document_storage.go(MODIFY)**

References: 

- internal\\models\\document.go(MODIFY)

Update all document storage operations to remove \`content\` field references:

1. In \`SaveDocument()\` method (line 33): Remove \`doc.Content\` from INSERT and UPDATE statements, keep only \`content_markdown\`

2. In \`SaveDocuments()\` method (line 105): Remove \`doc.Content\` from prepared statement INSERT and UPDATE

3. In \`GetDocument()\` method (line 195): Remove \`content\` from SELECT statement

4. In \`GetDocumentBySource()\` method (line 209): Remove \`content\` from SELECT statement

5. In \`UpdateDocument()\` method (line 223): Remove \`content = ?,\` from UPDATE statement and remove \`doc.Content\` from Exec parameters

6. In \`FullTextSearch()\` method (line 262): Remove \`d.content\` from SELECT statement

7. In \`SearchByIdentifier()\` method (line 288): Remove \`content\` from SELECT and remove content search condition from WHERE clause

8. In \`ListDocuments()\` method (line 350): Remove \`content\` from SELECT statement

9. In \`GetDocumentsBySource()\` method (line 394): Remove \`content\` from SELECT statement

10. In \`scanDocument()\` helper (line 521): Remove \`&doc.Content\` from Scan parameters

11. In \`scanDocuments()\` helper (line 589): Remove \`&doc.Content\` from Scan parameters

12. Update all SQL queries to use \`content_markdown\` for searches

- [x] **internal\\services\\documents\\document_service.go(MODIFY)**

References: 

- internal\\models\\document.go(MODIFY)

Update document service to use markdown field only:

In \`UpdateDocument()\` method (line 107), change line 115 to compare \`ContentMarkdown\` instead of \`Content\`:

From: \`contentChanged := existing.Content != doc.Content || existing.Title != doc.Title\`

To: \`contentChanged := existing.ContentMarkdown != doc.ContentMarkdown || existing.Title != doc.Title\`

- [x] **internal\\services\\search\\fts5_search_service.go(MODIFY)**

References: 

- internal\\models\\document.go(MODIFY)

Update search service to use markdown field:

In \`containsReference()\` helper function (line 247), change line 254 to check \`ContentMarkdown\` instead of \`Content\`:

From: \`if strings.Contains(doc.Content, reference) {\`

To: \`if strings.Contains(doc.ContentMarkdown, reference) {\`

- [x] **internal\\services\\metadata\\extractor.go(MODIFY)**

References: 

- internal\\models\\document.go(MODIFY)

Update metadata extractor to use markdown field:

1. Change line 50 to use \`ContentMarkdown\`: \`if prRefs := e.extractUniqueMatches(e.prRefPattern, doc.ContentMarkdown); len(prRefs) > 0 {\`

2. Change line 55 to use \`ContentMarkdown\`: \`if pageRefs := e.extractUniqueMatches(e.confluencePagePattern, doc.ContentMarkdown); len(pageRefs) > 0 {\`

- [x] **internal\\services\\identifiers\\extractor.go(MODIFY)**

References: 

- internal\\models\\document.go(MODIFY)

Update identifier extractor to use markdown field:

Change line 87 to use \`ContentMarkdown\` instead of \`Content\`:

From: \`allIdentifiers = append(allIdentifiers, e.ExtractFromText(doc.Content)...)\`

To: \`allIdentifiers = append(allIdentifiers, e.ExtractFromText(doc.ContentMarkdown)...)\`

### Phase 4: MCP Enhancement

- [x] **internal\\services\\mcp\\formatters.go(MODIFY)**

References: 

- internal\\models\\document.go(MODIFY)

Update MCP formatters to use markdown only:

1. In \`formatDocument()\` function (line 12), replace lines 32-36 to only use \`ContentMarkdown\` (remove fallback to Content)

2. In \`formatDocumentList()\` function (line 42), change line 60 to use \`ContentMarkdown\` instead of \`Content\`

3. In \`formatDocumentJSON()\` function (line 81), change line 88 to use \`content_markdown\` key instead of \`content\`

- [ ] **internal\\services\\mcp\\document_service.go(MODIFY)**

References: 

- internal\\services\\mcp\\types.go
- internal\\services\\mcp\\formatters.go(MODIFY)

Add new MCP tools for metadata queries:

1. In \`ListTools()\` method (line 97), add three new tools to the Tools slice:
- \`search_by_keywords\`: Search documents by keywords in metadata (requires keywords array, optional limit)
- \`get_document_summary\`: Get summary from document metadata (requires id)
- \`find_similar_sources\`: Find documents from similar sources grouped by metadata key (requires source_type, metadata_key, optional limit)

2. In \`CallTool()\` method (line 191), add cases for: \`search_by_keywords\`, \`get_document_summary\`, \`find_similar_sources\`

3. Add implementation methods:
- \`searchByKeywords(ctx, args)\`: Query documents, filter by keywords in metadata.keywords field, return formatted list
- \`getDocumentSummary(ctx, args)\`: Get document by ID, extract summary/word_count/keywords from metadata, format as markdown
- \`findSimilarSources(ctx, args)\`: Get documents by source type, group by specified metadata key, return grouped results with counts

4. Use existing \`formatDocumentList()\` helper from \`internal/services/mcp/formatters.go\` for formatting results

### Phase 5: UI & API

- [ ] **internal\\app\\app.go(MODIFY)**

References: 

- internal\\services\\jobs\\crawl_collect_job.go(NEW)
- internal\\services\\jobs\\scan_summarize_job.go(NEW)
- internal\\services\\scheduler\\scheduler_service.go(MODIFY)
- internal\\handlers\\job_handler.go(MODIFY)

Update app initialization to register default jobs with scheduler:

1. In \`initServices()\` method, replace scheduler initialization section (lines 274-284) with:
- Create scheduler service
- If \`Config.Jobs.CrawlAndCollect.Enabled\`, create \`CrawlCollectJob\` and register with scheduler
- If \`Config.Jobs.ScanAndSummarize.Enabled\`, create \`ScanSummarizeJob\` and register with scheduler
- Start scheduler with legacy cron expression for backward compatibility
- Log registered jobs

2. Add import for \`github.com/ternarybob/quaero/internal/services/jobs\` package

3. In \`initHandlers()\` method (line 290), update JobHandler initialization (line 328) to pass \`a.SchedulerService\` as additional parameter

- [ ] **pages\\jobs.html(MODIFY)**

Enhance jobs page to display default jobs separately:

1. After \"Job Statistics\" card (line 65), add new \"Default Jobs\" card with:
- Table showing: job name, description, schedule (cron), status (enabled/disabled), last run, next run, actions
- Actions: Enable/Disable toggle button, Edit Schedule button
- Refresh button in header

2. Update \"Jobs Table\" card header (line 68) to clarify it shows \"Crawler Jobs\" instead of just \"Jobs\"

3. In \`<script>\` section, add new functions:
- \`loadDefaultJobs()\`: Fetch from \`/api/jobs/default\`, render table with job status
- \`toggleDefaultJob(jobName, enable)\`: POST to \`/api/jobs/default/{name}/enable\` or \`/disable\`
- \`editJobSchedule(jobName, currentSchedule)\`: Prompt for new schedule, validate format
- \`updateJobSchedule(jobName, schedule)\`: PUT to \`/api/jobs/default/{name}/schedule\`

4. Update \`DOMContentLoaded\` event listener (line 623) to call \`loadDefaultJobs()\`

5. Update \`startAutoRefresh()\` function (line 606) to also refresh default jobs when running jobs exist

- [ ] **internal\\handlers\\job_handler.go(MODIFY)**

References: 

- internal\\interfaces\\scheduler_service.go(MODIFY)
- internal\\common\\config.go(MODIFY)

Add API endpoints for default job management:

1. Add \`schedulerService\` field to \`JobHandler\` struct (line 25)

2. Update \`NewJobHandler()\` constructor (line 34) to accept \`schedulerService interfaces.SchedulerService\` parameter and assign to struct field

3. Add new handler methods:
- \`GetDefaultJobsHandler(w, r)\`: GET /api/jobs/default - Returns all default job statuses from \`schedulerService.GetAllJobStatuses()\`, formats as JSON array
- \`EnableDefaultJobHandler(w, r)\`: POST /api/jobs/default/{name}/enable - Calls \`schedulerService.EnableJob(name)\`
- \`DisableDefaultJobHandler(w, r)\`: POST /api/jobs/default/{name}/disable - Calls \`schedulerService.DisableJob(name)\`
- \`UpdateDefaultJobScheduleHandler(w, r)\`: PUT /api/jobs/default/{name}/schedule - Validates schedule using \`common.ValidateJobSchedule()\`, updates job (note: requires scheduler service enhancement or config update)

4. Add import for \`common\` package to access \`ValidateJobSchedule()\` function

5. Extract job name from URL path using string splitting (similar to existing handlers)

- [ ] **internal\\server\\routes.go(MODIFY)**

References: 

- internal\\handlers\\job_handler.go(MODIFY)

Add routes for default job management:

In \`handleJobRoutes()\` function (line 88), add new route handling at the beginning:

1. GET /api/jobs/default - Route to \`GetDefaultJobsHandler\`
2. POST /api/jobs/default/{name}/enable - Route to \`EnableDefaultJobHandler\`
3. POST /api/jobs/default/{name}/disable - Route to \`DisableDefaultJobHandler\`
4. PUT /api/jobs/default/{name}/schedule - Route to \`UpdateDefaultJobScheduleHandler\`

Use path suffix checking and \`strings.Contains()\` to detect \`/default/\` in path, similar to existing route handling patterns in the function.

Add import for \`strings\` package if not already present.

- [ ] **README.md(MODIFY)**

Update README to document default jobs feature:

1. In \"Key Features\" section (line 9), add bullet point for scheduled jobs

2. In \"Configuration File\" section (line 637), add jobs configuration example showing \`[jobs.crawl_and_collect]\` and \`[jobs.scan_and_summarize]\` with enabled, schedule, and description fields

3. Add new \"Default Jobs\" subsection under \"API Endpoints\" (around line 530) documenting:
- GET /api/jobs/default
- POST /api/jobs/default/{name}/enable
- POST /api/jobs/default/{name}/disable
- PUT /api/jobs/default/{name}/schedule

4. In \"Current Status\" section (line 721), update working features to include:
- Default scheduled jobs (crawl_and_collect, scan_and_summarize)
- Markdown-first document storage with FTS5 search
- Job management UI with enable/disable controls