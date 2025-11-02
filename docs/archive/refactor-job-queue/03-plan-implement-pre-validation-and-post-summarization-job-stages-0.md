I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Key Architectural Patterns:**
- Job types follow consistent structure: Deps struct → Job struct embedding BaseJob → Execute/Validate/GetType methods
- Queue messages use `Type` field for routing ("crawler_url", "summarizer", "cleanup", etc.)
- Config passed via `msg.Config` map with type assertions and defaults
- BaseJob provides `LogJobEvent()`, `EnqueueChildJob()`, `CreateChildJobRecord()` helpers
- Worker pool registers handlers via `RegisterHandler(type, handlerFunc)` in app.go
- StartCrawl creates parent job, then enqueues seed URLs as crawler_url messages (lines 575-636)
- ExecuteCompletionProbe marks job completed and publishes EventJobCompleted (lines 1136-1180)

**Document Tracking:**
- Documents don't store job_id - will query by source_type + timestamp window for post-summarization
- Documents created during crawl have CreatedAt timestamps matching job execution period

**Validation Utilities Available:**
- `common.ValidateBaseURL()` - validates URL format and detects test URLs
- `models.SourceConfig.Validate()` - validates source configuration
- `interfaces.AuthStorage.GetCredentialsByID()` - retrieves auth credentials
- HTTP HEAD requests can check URL accessibility

**Integration Points:**
1. StartCrawl (line 575): Enqueue pre-validation BEFORE seed URLs
2. ExecuteCompletionProbe (line 1173): Enqueue post-summarization AFTER marking completed
3. app.go (lines 435-498): Register new job handlers with worker pool

### Approach

Create two new job types (PreValidationJob and PostSummarizationJob) following established patterns. PreValidationJob validates auth/source/URLs before crawling starts. PostSummarizationJob generates corpus-level summaries after all URLs complete. Integrate into StartCrawl (pre-validation before seed URLs) and ExecuteCompletionProbe (post-summarization after completion). Register handlers in worker pool and app initialization.

### Reasoning

Explored existing job type patterns (SummarizerJob, CleanupJob, CrawlerJob), examined StartCrawl seed URL enqueueing logic, reviewed ExecuteCompletionProbe completion detection mechanism, analyzed queue message structure and handler registration in app.go, verified auth/source validation utilities in common/url_utils.go and models/source.go, confirmed document storage doesn't track job_id (will use timestamp-based queries), and identified all integration points for pre/post job enqueueing.

## Mermaid Diagram

sequenceDiagram
    participant UI as Queue Management UI
    participant Service as CrawlerService
    participant Queue as QueueManager
    participant Worker as WorkerPool
    participant PreVal as PreValidationJob
    participant Crawler as CrawlerJob
    participant Probe as CompletionProbe
    participant PostSum as PostSummarizationJob
    participant Storage as JobStorage
    participant LLM as LLMService

    Note over UI,LLM: Phase 1: Job Creation with Pre-Validation

    UI->>Service: StartCrawl(sourceType, seedURLs, config)
    Service->>Storage: SaveJob(parentJob with JobType='parent')
    Storage-->>Service: Parent job persisted
    
    Service->>Queue: Enqueue(pre_validation message)
    Service->>Storage: SaveJob(preValidationJob with JobType='pre_validation')
    
    Service->>Queue: Enqueue(seed URL messages as crawler_url)
    Service->>Storage: SaveJob(seedChildJobs with JobType='crawler_url')
    Service-->>UI: Return parentJobID

    Note over UI,LLM: Phase 2: Pre-Validation Execution

    Worker->>Queue: Receive(pre_validation message)
    Worker->>PreVal: Execute(ctx, msg)
    PreVal->>Storage: GetSource(sourceID) - validate config
    PreVal->>Storage: GetCredentialsByID(authID) - validate auth
    PreVal->>PreVal: HTTP HEAD requests to seed URLs
    PreVal->>Storage: LogJobEvent("Pre-validation completed")
    PreVal-->>Worker: Success/Failure
    Worker->>Queue: Delete(pre_validation message)

    Note over UI,LLM: Phase 3: URL Crawling (Existing Flow)

    Worker->>Queue: Receive(crawler_url message)
    Worker->>Crawler: Execute(ctx, msg)
    Crawler->>Crawler: Scrape URL, create document
    Crawler->>Storage: SaveDocument(doc)
    Crawler->>Storage: UpdateProgressCountersAtomic(parent)
    Crawler->>Crawler: Discover links, enqueue children
    Crawler->>Crawler: checkAndEnqueueCompletionProbe()
    Crawler-->>Worker: Success

    Note over UI,LLM: Phase 4: Completion Detection

    Worker->>Queue: Receive(completion_probe message after 5s delay)
    Worker->>Probe: ExecuteCompletionProbe(ctx, msg)
    Probe->>Storage: GetJob(parentID)
    Probe->>Probe: Check PendingURLs==0 && heartbeat stale
    Probe->>Storage: SaveJob(status='completed')
    
    Note over UI,LLM: Phase 5: Post-Summarization Trigger
    
    Probe->>Queue: Enqueue(post_summarization message)
    Probe->>Storage: SaveJob(postSummaryJob with JobType='post_summary')
    Probe-->>Worker: Completion verified

    Note over UI,LLM: Phase 6: Post-Summarization Execution

    Worker->>Queue: Receive(post_summarization message)
    Worker->>PostSum: Execute(ctx, msg)
    PostSum->>Storage: GetJob(parentID) - load parent job
    PostSum->>Storage: ListDocuments(timestamp filter)
    PostSum->>PostSum: Aggregate document content
    PostSum->>LLM: Chat(corpus summary prompt)
    LLM-->>PostSum: Generated summary
    PostSum->>PostSum: Extract keywords via frequency analysis
    PostSum->>Storage: SaveJob(parent with corpus_summary metadata)
    PostSum->>Storage: LogJobEvent("Post-summarization completed")
    PostSum-->>Worker: Success
    Worker->>Queue: Delete(post_summarization message)

    Note over UI,LLM: UI displays parent job with pre/post child jobs

## Proposed File Changes

### internal\jobs\types\pre_validation.go(NEW)

References: 

- internal\jobs\types\base.go
- internal\jobs\types\summarizer.go
- internal\common\url_utils.go
- internal\models\source.go

**Create PreValidationJob type for pre-flight validation:**

1. **Package declaration and imports:**
   - Package: `types`
   - Imports: context, fmt, net/http, time, internal/interfaces, internal/models, internal/queue, internal/common

2. **Define PreValidationJobDeps struct:**
   - `AuthStorage interfaces.AuthStorage` - for retrieving auth credentials
   - `SourceStorage interfaces.SourceStorage` - for retrieving source config
   - `HTTPClient *http.Client` - for URL accessibility checks (HEAD requests)

3. **Define PreValidationJob struct:**
   - Embed `*BaseJob`
   - Field: `deps *PreValidationJobDeps`

4. **Constructor NewPreValidationJob:**
   - Parameters: `base *BaseJob, deps *PreValidationJobDeps`
   - Returns: `*PreValidationJob`
   - Initialize struct with base and deps

5. **Execute method:**
   - Extract parent job ID from `msg.ParentID`
   - Extract source_id from `msg.Config["source_id"]` (string type assertion)
   - Extract auth_id from `msg.Config["auth_id"]` (string type assertion)
   - Extract seed_urls from `msg.Config["seed_urls"]` ([]interface{} type assertion, convert to []string)
   - Log validation start event via `LogJobEvent(ctx, parentID, "info", "Starting pre-validation")`

   **Validation Steps:**
   
   a. **Validate Source Config (if source_id provided):**
      - Call `deps.SourceStorage.GetSource(ctx, sourceID)`
      - If error: log error, return fmt.Errorf("source config not found: %w", err)
      - Call `sourceConfig.Validate()`
      - If error: log validation failure, return fmt.Errorf("source config validation failed: %w", err)
      - Log success: "Source config validated: base_url={sourceConfig.BaseURL}"
   
   b. **Validate Auth Credentials (if auth_id provided):**
      - Call `deps.AuthStorage.GetCredentialsByID(ctx, authID)`
      - If error: log error, return fmt.Errorf("auth credentials not found: %w", err)
      - Check auth.Cookies is not nil/empty (unmarshal JSON and check length)
      - If empty: return fmt.Errorf("auth credentials missing cookies")
      - Log success: "Auth credentials validated: {cookieCount} cookies available"
   
   c. **Validate Seed URLs Accessibility:**
      - For each seed URL in seed_urls:
        - Call `common.ValidateBaseURL(seedURL, c.logger)` to check format
        - If invalid: log warning, add to failed list, continue
        - Create HTTP HEAD request to seed URL with 5-second timeout
        - Execute request with `deps.HTTPClient.Do(req)`
        - If error or status >= 400: log warning, add to failed list, continue
        - If success: log debug "Seed URL accessible: {seedURL}"
      - If all URLs failed: return fmt.Errorf("all seed URLs failed validation")
      - If some failed: log warning "Some seed URLs failed validation: {failedCount}/{totalCount}"
   
   - Log validation completion event via `LogJobEvent(ctx, parentID, "info", "Pre-validation completed successfully")`
   - Return nil (success)

6. **Validate method:**
   - Check `msg.ParentID != ""` (required for logging)
   - Check `msg.Config != nil`
   - Return error if validation fails, nil otherwise

7. **GetType method:**
   - Return string: `"pre_validation"`

**Pattern Reference:** Follow exact structure of `internal/jobs/types/summarizer.go` (lines 14-99) for deps/job struct, constructor, and method signatures.

### internal\jobs\types\post_summarization.go(NEW)

References: 

- internal\jobs\types\base.go
- internal\jobs\types\summarizer.go
- internal\models\crawler_job.go(MODIFY)

**Create PostSummarizationJob type for corpus-level summarization:**

1. **Package declaration and imports:**
   - Package: `types`
   - Imports: context, fmt, strings, time, internal/interfaces, internal/models, internal/queue

2. **Define PostSummarizationJobDeps struct:**
   - `LLMService interfaces.LLMService` - for generating summaries
   - `DocumentStorage interfaces.DocumentStorage` - for querying documents
   - `JobStorage interfaces.JobStorage` - for updating parent job with summary

3. **Define PostSummarizationJob struct:**
   - Embed `*BaseJob`
   - Field: `deps *PostSummarizationJobDeps`

4. **Constructor NewPostSummarizationJob:**
   - Parameters: `base *BaseJob, deps *PostSummarizationJobDeps`
   - Returns: `*PostSummarizationJob`
   - Initialize struct with base and deps

5. **Execute method:**
   - Extract parent job ID from `msg.ParentID`
   - Extract source_type from `msg.Config["source_type"]` (string type assertion)
   - Extract entity_type from `msg.Config["entity_type"]` (string type assertion)
   - Log job start event via `LogJobEvent(ctx, parentID, "info", "Starting post-summarization")`

   **Load Parent Job:**
   - Call `deps.JobStorage.GetJob(ctx, parentID)`
   - Type assert to `*models.CrawlJob`
   - If error or wrong type: return fmt.Errorf("failed to load parent job: %w", err)
   - Extract job.CreatedAt and job.CompletedAt for timestamp window

   **Query Documents Created During Job:**
   - Build ListOptions with:
     - OrderBy: "created_at"
     - OrderDir: "desc"
     - Limit: 1000 (reasonable corpus size)
   - Call `deps.DocumentStorage.ListDocuments(opts)`
   - Filter documents by:
     - `doc.SourceType == source_type` (if provided)
     - `doc.CreatedAt >= job.CreatedAt && doc.CreatedAt <= job.CompletedAt`
   - If no documents found: log info "No documents found for summarization", return nil
   - Log info: "Found {docCount} documents for summarization"

   **Generate Corpus Summary:**
   - Aggregate document titles and content (limit to first 500 chars per doc to avoid token limits)
   - Build corpus text: concatenate titles with "\n---\n" separator
   - Truncate corpus to max 10,000 characters if needed
   - Create LLM messages:
     - System: "You are a helpful assistant that generates concise corpus-level summaries. Analyze the collection of documents and provide: 1) Overall theme/purpose, 2) Key topics covered, 3) Notable patterns or insights."
     - User: "Summarize this collection of {docCount} documents:\n\n{corpusText}"
   - Call `deps.LLMService.Chat(ctx, messages)`
   - If error: log error, use fallback summary "Summary generation failed: {error}"
   - Log info: "Generated corpus summary: {summaryLength} characters"

   **Extract Keywords:**
   - Aggregate all document titles and content
   - Simple frequency analysis: split into words, count occurrences
   - Filter: min length 4 chars, exclude common stop words ("the", "and", "for", etc.)
   - Sort by frequency descending
   - Take top 20 keywords
   - Log info: "Extracted {keywordCount} keywords"

   **Update Parent Job Metadata:**
   - Load parent job again (in case it was updated)
   - Add to job metadata (create if nil):
     - `corpus_summary`: generated summary string
     - `corpus_keywords`: keyword array
     - `corpus_document_count`: document count
     - `summarized_at`: current timestamp RFC3339
   - Call `deps.JobStorage.SaveJob(ctx, job)`
   - If error: log error but don't fail (summary generated successfully)
   - Log success: "Parent job updated with corpus summary"

   - Log completion event via `LogJobEvent(ctx, parentID, "info", fmt.Sprintf("Post-summarization completed: {docCount} documents, {keywordCount} keywords"))`
   - Return nil (success)

6. **Validate method:**
   - Check `msg.ParentID != ""` (required)
   - Check `msg.Config != nil`
   - Return error if validation fails, nil otherwise

7. **GetType method:**
   - Return string: `"post_summarization"`

**Pattern Reference:** Follow structure of `internal/jobs/types/summarizer.go` (lines 202-366) for document querying and LLM interaction patterns.

### internal\services\crawler\service.go(MODIFY)

References: 

- internal\queue\types.go(MODIFY)
- internal\models\crawler_job.go(MODIFY)

**Enqueue pre-validation job BEFORE seed URLs:**

**Location: After job persistence (line 525), BEFORE seed queue building (line 553)**

1. **Create pre-validation job message (insert after line 546):**
   - Create message ID: `fmt.Sprintf("%s-pre-validation", jobID)`
   - Create JobMessage:
     - Type: `"pre_validation"`
     - ParentID: `jobID`
     - Config map with:
       - `"source_id"`: sourceID (if provided)
       - `"auth_id"`: authSnapshot.ID (if authSnapshot != nil)
       - `"seed_urls"`: seedURLs array
       - `"source_type"`: sourceType
       - `"entity_type"`: entityType
   - Log debug: "Enqueueing pre-validation job: job_id={jobID}"

2. **Enqueue pre-validation message:**
   - Call `s.queueManager.Enqueue(s.ctx, preValidationMsg)`
   - If error: log warning "Failed to enqueue pre-validation job: {err}", continue (don't block crawl)
   - If success: log info "Pre-validation job enqueued: message_id={msg.ID}"

3. **Create pre-validation CrawlJob record:**
   - Create CrawlJob struct:
     - ID: message ID
     - ParentID: jobID
     - JobType: `models.JobTypePreValidation`
     - Name: "Pre-validation"
     - SourceType: sourceType
     - EntityType: entityType
     - Status: `models.JobStatusPending`
     - CreatedAt: time.Now()
   - Call `s.jobStorage.SaveJob(s.ctx, preValidationJob)`
   - If error: log warning, continue
   - If success: log debug "Pre-validation job persisted to database"

**Note:** Pre-validation runs asynchronously. Seed URLs are still enqueued immediately (existing behavior). Pre-validation failures will be logged but won't block the crawl (fail-open design for backward compatibility).

**Pattern Reference:** Follow seed URL enqueueing pattern at lines 575-636 for message creation and job persistence.

### internal\jobs\types\crawler.go(MODIFY)

References: 

- internal\queue\types.go(MODIFY)
- internal\models\crawler_job.go(MODIFY)

**Enqueue post-summarization job after completion:**

**Location: In ExecuteCompletionProbe method, after marking job completed (line 1180), BEFORE final log event (line 1182)**

1. **Create post-summarization job message (insert after line 1180):**
   - Extract source_type and entity_type from job struct
   - Create message ID: `fmt.Sprintf("%s-post-summary", msg.ParentID)`
   - Create JobMessage:
     - Type: `"post_summarization"`
     - ParentID: `msg.ParentID`
     - JobDefinitionID: `msg.JobDefinitionID`
     - Config map with:
       - `"source_type"`: job.SourceType
       - `"entity_type"`: job.EntityType
       - `"parent_job_id"`: msg.ParentID
   - Log info: "Enqueueing post-summarization job: parent_id={msg.ParentID}"

2. **Enqueue post-summarization message:**
   - Call `c.deps.QueueManager.Enqueue(ctx, postSummaryMsg)`
   - If error: log warning "Failed to enqueue post-summarization job: {err}" (don't fail completion)
   - If success: log info "Post-summarization job enqueued: message_id={msg.ID}"

3. **Create post-summarization CrawlJob record:**
   - Create CrawlJob struct:
     - ID: message ID
     - ParentID: msg.ParentID
     - JobType: `models.JobTypePostSummary`
     - Name: "Post-summarization"
     - SourceType: job.SourceType
     - EntityType: job.EntityType
     - Status: `models.JobStatusPending`
     - CreatedAt: time.Now()
   - Call `c.deps.JobStorage.SaveJob(ctx, postSummaryJob)`
   - If error: log warning, continue
   - If success: log debug "Post-summarization job persisted to database"

**Error Handling:** Post-summarization failures should NOT affect parent job completion status. Use warning-level logging and continue.

**Pattern Reference:** Follow completion probe enqueueing pattern at lines 1001-1022 for message creation with delay.

### internal\queue\types.go(MODIFY)

**Add helper constructors for new job message types:**

**Location: After NewCleanupMessage function (line 99)**

1. **Add NewPreValidationMessage constructor:**
   - Function signature: `func NewPreValidationMessage(parentID string, config map[string]interface{}) *JobMessage`
   - Create message via `NewJobMessage("pre_validation", parentID)`
   - Set `msg.Config = config`
   - Return message
   - Add comment: "// NewPreValidationMessage creates a pre-validation job message"

2. **Add NewPostSummarizationMessage constructor:**
   - Function signature: `func NewPostSummarizationMessage(parentID string, config map[string]interface{}) *JobMessage`
   - Create message via `NewJobMessage("post_summarization", parentID)`
   - Set `msg.Config = config`
   - Return message
   - Add comment: "// NewPostSummarizationMessage creates a post-summarization job message"

**Pattern Reference:** Follow exact pattern of `NewSummarizerMessage` (lines 88-92) and `NewCleanupMessage` (lines 95-99).

### internal\app\app.go(MODIFY)

References: 

- internal\jobs\types\pre_validation.go(NEW)
- internal\jobs\types\post_summarization.go(NEW)

**Register pre-validation and post-summarization job handlers:**

**Location: After reindex job handler registration (line 498), BEFORE parent job handler (line 500)**

1. **Register PreValidationJob handler (insert after line 498):**
   - Create deps struct:
     ```
     preValidationJobDeps := &jobtypes.PreValidationJobDeps{
         AuthStorage:   a.StorageManager.AuthStorage(),
         SourceStorage: a.StorageManager.SourceStorage(),
         HTTPClient:    &http.Client{Timeout: 10 * time.Second},
     }
     ```
   - Create handler function:
     ```
     preValidationJobHandler := func(ctx context.Context, msg *queue.JobMessage) error {
         baseJob := jobtypes.NewBaseJob(msg.ID, msg.JobDefinitionID, a.Logger, a.JobManager, a.QueueManager, a.StorageManager.JobLogStorage())
         job := jobtypes.NewPreValidationJob(baseJob, preValidationJobDeps)
         return job.Execute(ctx, msg)
     }
     ```
   - Register handler: `a.WorkerPool.RegisterHandler("pre_validation", preValidationJobHandler)`
   - Log: `a.Logger.Info().Msg("Pre-validation job handler registered")`

2. **Register PostSummarizationJob handler (insert after pre-validation):**
   - Create deps struct:
     ```
     postSummarizationJobDeps := &jobtypes.PostSummarizationJobDeps{
         LLMService:      a.LLMService,
         DocumentStorage: a.StorageManager.DocumentStorage(),
         JobStorage:      a.StorageManager.JobStorage(),
     }
     ```
   - Create handler function:
     ```
     postSummarizationJobHandler := func(ctx context.Context, msg *queue.JobMessage) error {
         baseJob := jobtypes.NewBaseJob(msg.ID, msg.JobDefinitionID, a.Logger, a.JobManager, a.QueueManager, a.StorageManager.JobLogStorage())
         job := jobtypes.NewPostSummarizationJob(baseJob, postSummarizationJobDeps)
         return job.Execute(ctx, msg)
     }
     ```
   - Register handler: `a.WorkerPool.RegisterHandler("post_summarization", postSummarizationJobHandler)`
   - Log: `a.Logger.Info().Msg("Post-summarization job handler registered")`

**Import Requirements:**
- Add `"net/http"` to imports if not already present
- Add `"time"` to imports if not already present

**Pattern Reference:** Follow exact pattern of summarizer handler registration (lines 462-473) and cleanup handler registration (lines 475-486).

### internal\models\crawler_job.go(MODIFY)

**Verify JobType constants include pre-validation and post-summary:**

**Location: JobType constants section (lines 22-27)**

**Verification Only - No Changes Needed:**
- Confirm `JobTypePreValidation JobType = "pre_validation"` exists (line 24)
- Confirm `JobTypePostSummary JobType = "post_summary"` exists (line 26)

These constants were already added in Phase 1. This file requires no modifications for Phase 3.

**Note:** If constants are missing, add them following the existing pattern.