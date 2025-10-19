I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Critical Insight from User Feedback:**

The crawler is intentionally generic - it doesn't have Jira-specific or Confluence-specific logic. It just fetches URLs and stores responses. The transformation layer should follow the same philosophy:

1. **Generic content extraction** - Look for common JSON fields (title, body, content, description, text) rather than parsing Jira's \`fields.summary\` or Confluence's \`body.storage.value\`
2. **Minimal metadata** - Just track: source_type, source_id, url, timestamps, links
3. **No domain knowledge** - Don't parse issue types, space keys, assignees, etc.
4. **Raw preservation** - Store original JSON for future processing

This approach is simpler, more maintainable, and aligns with the generic crawler architecture. The search/RAG layer can handle content understanding without the transformer needing domain-specific knowledge.

### Approach

**Architecture Decision: Generic Document Transformation Pipeline**

Create a single generic transformer service that processes all crawler results uniformly, regardless of source type. The transformer will:
1. Subscribe to \`EventCollectionTriggered\` event
2. Query for completed crawler jobs
3. Extract content from JSON responses using generic patterns (title, body, content, text fields)
4. Create normalized documents with minimal source metadata (type, ID, URL, timestamps)
5. Store raw JSON in metadata for future processing

**Key Design Principles:**
- **Source-agnostic:** No Jira/Confluence-specific parsing logic
- **Content-first:** Extract all text content from JSON responses
- **Minimal metadata:** Track source, links, timestamps only
- **Extensible:** Raw JSON preserved for future enhancement
- **Idempotent:** Upsert semantics allow safe re-processing

### Reasoning

Explored the codebase and understood: (1) Event system with pub/sub pattern, \`EventCollectionTriggered\` has no subscribers; (2) Crawler is already generic - fetches any URL, stores results with metadata; (3) Document model supports generic content with ContentMarkdown field; (4) Storage layer provides upsert via (source_type, source_id) constraint; (5) User feedback indicates preference for generic transformation over source-specific parsing.

## Mermaid Diagram

sequenceDiagram
    participant Scheduler as Scheduler Service
    participant EventSvc as Event Service
    participant Transformer as Document Transformer
    participant JobStore as Job Storage
    participant DocStore as Document Storage
    participant DB as SQLite Database
    
    Note over Scheduler,EventSvc: Every 5 minutes (cron)
    Scheduler->>EventSvc: PublishSync(EventCollectionTriggered)
    EventSvc->>Transformer: handleCollectionEvent(ctx, event)
    
    rect rgb(240, 248, 255)
        Note over Transformer,DB: Generic Transformation Pipeline
        Transformer->>JobStore: GetJobsByStatus(ctx, \"completed\")
        JobStore->>DB: SELECT * FROM crawl_jobs WHERE status='completed'
        DB-->>JobStore: All completed jobs
        JobStore-->>Transformer: Jobs (any source type)
        
        loop For each completed job
            Transformer->>Transformer: Parse job.SourceConfigSnapshot
            Transformer->>Transformer: Extract source_type, base_url
            
            Note over Transformer: Process crawler results
            Transformer->>Transformer: Extract response_body from metadata
            Transformer->>Transformer: Unmarshal JSON → map[string]interface{}
            
            Transformer->>Transformer: findContentFields(json)<br/>Search for: title, body, content, etc.
            Transformer->>Transformer: extractAllText(json)<br/>Recursively collect all text
            Transformer->>Transformer: extractSourceID(json)<br/>Find: id, key, or use URL
            
            Note over Transformer: Build generic document
            Transformer->>Transformer: Create Document:<br/>- SourceType from job<br/>- SourceID from JSON<br/>- Title from findContentFields<br/>- ContentMarkdown from extractAllText<br/>- Metadata: {source, url, raw_json}
            
            Transformer->>DocStore: SaveDocument(doc)
            DocStore->>DB: INSERT/UPDATE documents<br/>(source_type, source_id)
            DB-->>DocStore: Upsert complete
        end
        
        Transformer->>Transformer: Log: \"Transformed X documents from Y jobs\"
    end
    
    Note over Scheduler,DB: Documents ready for search/RAG<br/>(no source-specific knowledge required)

## Proposed File Changes

### internal\\services\\transformer(NEW)

Create a new directory to house the generic document transformer service.

### internal\\services\\transformer\\service.go(NEW)

References: 

- internal\\interfaces\\storage.go(MODIFY)
- internal\\interfaces\\event_service.go
- internal\\models\\document.go
- internal\\services\\crawler\\types.go

Create a generic document transformer service that processes all crawler results uniformly.

**Service Structure:**
- Struct: \`Service\` with fields: \`jobStorage\` (interfaces.JobStorage), \`documentStorage\` (interfaces.DocumentStorage), \`eventService\` (interfaces.EventService), \`logger\` (arbor.ILogger)
- Constructor: \`NewService(jobStorage, documentStorage, eventService, logger)\` that subscribes to \`EventCollectionTriggered\` during initialization

**Event Handler Method:**
- \`handleCollectionEvent(ctx context.Context, event interfaces.Event) error\`
- Query \`jobStorage.GetJobsByStatus(ctx, \"completed\")\` to find all recently completed jobs
- Filter for jobs that haven't been processed yet (track via job metadata or separate flag)
- For each job, call \`transformJob(ctx, job)\`
- Log summary: \"Transformed X documents from Y jobs\"

**Job Transformation:**
- \`transformJob(ctx context.Context, job *crawler.CrawlJob) error\`
- Parse \`job.SourceConfigSnapshot\` to extract base URL and source ID
- Note: Job results may not be in memory after restart - handle gracefully
- For each URL in the job, attempt to reconstruct or skip if unavailable
- Call \`extractDocument()\` for each result

**Generic Content Extraction:**
- \`extractDocument(result *crawler.CrawlResult, sourceType, sourceID, baseURL string) (*models.Document, error)\`
- Extract \`response_body\` from \`result.Metadata\`
- Unmarshal JSON into \`map[string]interface{}\`
- Call \`findContentFields()\` to locate title and body content
- Call \`extractAllText()\` to concatenate all text content
- Build minimal metadata map with: source_type, source_id, url, raw_json (stringified)
- Create \`Document\` with:
  - \`ID\`: UUID with \"doc_\" prefix
  - \`SourceType\`: from job (\"jira\", \"confluence\", \"github\")
  - \`SourceID\`: extracted from JSON (\"id\", \"key\", or URL path)
  - \`Title\`: from \`findContentFields()\`
  - \`ContentMarkdown\`: from \`extractAllText()\`
  - \`Metadata\`: minimal map
  - \`URL\`: from result or constructed from base URL
  - \`DetailLevel\`: \"full\"
  - Timestamps: now

**Generic Field Discovery:**
- \`findContentFields(data map[string]interface{}) (title, body string)\`
- Search for title in common field names: \"title\", \"name\", \"summary\", \"subject\"
- Search for body in: \"body\", \"content\", \"description\", \"text\", \"value\"
- Recursively search nested objects (e.g., \"body.storage.value\", \"fields.description\")
- Return first matches found

**Text Extraction:**
- \`extractAllText(data interface{}) string\`
- Recursively traverse JSON structure
- Concatenate all string values found
- Skip keys like: \"id\", \"key\", \"self\", \"_links\", \"_expandable\"
- Handle arrays by processing each element
- Strip HTML tags if detected
- Return concatenated text with paragraph breaks

**Document Persistence:**
- Call \`documentStorage.SaveDocument(doc)\` for each document
- Handle errors with logging, continue processing
- Use upsert semantics (source_type + source_id unique constraint)

**Error Handling:**
- Log warnings for malformed JSON, skip and continue
- Aggregate errors and return summary
- Use \`logger.Warn()\` for skipped items, \`logger.Error()\` for critical failures

### internal\\services\\transformer\\helpers.go(NEW)

Create helper functions for generic content extraction and processing.

**UUID Generation:**
- \`generateDocumentID() string\` - Generate UUID with \"doc_\" prefix using \`github.com/google/uuid\`

**Source ID Extraction:**
- \`extractSourceID(data map[string]interface{}, fallbackURL string) string\`
- Look for ID in common fields: \"id\", \"key\", \"number\"
- If not found, extract from URL path (last segment)
- Return string representation

**URL Construction:**
- \`constructURL(baseURL, path string) string\`
- Handle relative and absolute URLs
- Use \`net/url\` package for proper URL joining

**HTML Stripping:**
- \`stripHTML(html string) string\`
- Remove HTML tags using regex: \`<[^>]*>\`
- Decode HTML entities: \`&amp;\` → \`&\`, \`&lt;\` → \`<\`, etc.
- Return plain text

**JSON Flattening:**
- \`flattenJSON(data interface{}, prefix string) map[string]string\`
- Recursively flatten nested JSON to dot-notation keys
- Example: \`{\"a\": {\"b\": \"c\"}}\` → \`{\"a.b\": \"c\"}\`
- Used for metadata storage

**Text Cleaning:**
- \`cleanText(text string) string\`
- Trim whitespace
- Remove excessive newlines (more than 2 consecutive)
- Normalize unicode characters

**Error Aggregation:**
- \`aggregateErrors(errs []error) error\`
- Combine multiple errors with count
- Return nil if no errors

These helpers provide reusable utilities for generic content processing without source-specific knowledge.

### internal\\app\\app.go(MODIFY)

References: 

- internal\\services\\transformer\\service.go(NEW)

Initialize the generic document transformer service in the service initialization sequence.

**Location:** In the \`initServices()\` method, after \`EventService\` initialization (line 208) and before scheduler service initialization (line 277).

**Add Import:**
- Add import: \`\"github.com/ternarybob/quaero/internal/services/transformer\"\`

**Add Field to App Struct (lines 37-81):**
- Add new field after \`EventService\` (line 53):
  - \`TransformerService *transformer.Service\`

**Initialize Transformer (after line 222, before line 233):**
- Recommended placement: After \`SourceService\` initialization (line 222), before crawler service (line 233)
- Add comment: \`// 6.7. Initialize document transformer (subscribes to collection events)\`
- Create transformer:
  \`\`\`
  a.TransformerService = transformer.NewService(
      a.StorageManager.JobStorage(),
      a.StorageManager.DocumentStorage(),
      a.EventService,
      a.Logger,
  )
  a.Logger.Info().Msg(\"Document transformer initialized and subscribed to collection events\")
  \`\`\`

**Placement Rationale:**
- Must be after \`EventService\` (line 208) so transformer can subscribe
- Must be after \`DocumentService\` and storage initialization (lines 188-197)
- Should be before scheduler (line 277) so transformer is ready when events are published
- No dependency on crawler service, so can be initialized before or after it

**No Close() Method Needed:**
- Transformer is stateless, no cleanup required
- Event subscription is managed by EventService

### internal\\interfaces\\storage.go(MODIFY)

**No changes required.** The existing interfaces already provide all necessary methods:

**JobStorage interface:**
- \`GetJobsByStatus(ctx context.Context, status string) ([]interface{}, error)\` - Query completed jobs
- \`GetJob(ctx context.Context, jobID string) (interface{}, error)\` - Retrieve job details

**DocumentStorage interface:**
- \`SaveDocument(doc *models.Document) error\` - Persist transformed documents
- \`SaveDocuments(docs []*models.Document) error\` - Batch operations

Both interfaces are implemented in \`internal/storage/sqlite/\` and available via \`StorageManager\`. No modifications needed.