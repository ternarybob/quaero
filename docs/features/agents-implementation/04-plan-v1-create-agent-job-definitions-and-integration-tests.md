I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**Agent Framework Status:**
- ✅ `AgentExecutor` - Queue-based job executor (handles individual agent jobs)
- ✅ `AgentStepExecutor` - Job definition step executor (creates agent jobs for documents)
- ✅ `KeywordExtractor` - Keyword extraction agent using Google ADK
- ✅ `AgentService` - Service managing agent lifecycle and execution
- ✅ Configuration in `quaero.toml` with `[agent]` section
- ✅ Registration in `app.go` (lines 319-330 for AgentExecutor, lines 398-403 for AgentStepExecutor)

**Test Infrastructure:**
- ✅ `common.SetupTestEnvironment()` - Starts test server, builds binary, loads config
- ✅ `HTTPTestHelper` - Provides GET, POST, PUT, DELETE methods with assertions
- ✅ Test patterns from `job_definition_execution_test.go` - Job creation, polling, status verification
- ✅ Unit test pattern from `arbor_channel_test.go` - Simple table-driven tests

**Missing Components:**
- ❌ Job definition TOML for keyword extraction agent
- ❌ API tests for agent job execution
- ❌ Unit tests for keyword extractor with mock responses
- ❌ Error handling tests (missing API key, invalid document, API failures)

**Document Metadata Pattern:**
- Agent results stored in `document.Metadata[agentType]` (e.g., `metadata["keyword_extractor"]`)
- Contains `keywords` array and optional `confidence` map
- Updated via `documentStorage.UpdateDocument(doc)`

**Key Integration Points:**
- Job definitions loaded via `LoadTestJobDefinitions()` in test setup
- Jobs executed via `POST /api/job-definitions/{id}/execute`
- Job status polled via `GET /api/jobs/{id}`
- Documents queried via `GET /api/documents/{id}`

### Approach

## Implementation Strategy

**Create comprehensive testing and configuration for the agent framework** by following established patterns from the codebase. The approach focuses on three deliverables:

1. **Job Definition TOML** - Configuration file demonstrating keyword extraction with agent chaining example
2. **API Integration Tests** - End-to-end tests verifying agent job execution via HTTP API
3. **Unit Tests** - Isolated tests for keyword extraction logic with mocked ADK responses

**Why This Approach:**
- Follows existing patterns from `news-crawler.toml`, `nearby-restaurants-places.toml`, and `job_definition_execution_test.go`
- Reuses `common.SetupTestEnvironment()` and `HTTPTestHelper` infrastructure
- Tests both queue-based execution (AgentExecutor) and job definition execution (AgentStepExecutor)
- Validates error handling for missing API keys, invalid documents, and API failures
- Provides clear examples for future agent implementations

### Reasoning

I explored the repository structure, read example job definitions (`news-crawler.toml`, `nearby-restaurants-places.toml`), examined the test infrastructure (`job_definition_execution_test.go`, `setup.go`), reviewed the agent framework implementation (`agent_executor.go`, `keyword_extractor.go`, `agent_step_executor.go`), and understood the document metadata storage pattern. I identified all necessary patterns, helper functions, and integration points needed to create the tests and configuration.

## Mermaid Diagram

sequenceDiagram
    participant Test as API Test
    participant API as Quaero API
    participant JobExec as Job Executor
    participant AgentStep as Agent Step Executor
    participant Queue as Queue Manager
    participant AgentExec as Agent Executor
    participant AgentSvc as Agent Service
    participant KeywordExt as Keyword Extractor
    participant ADK as Google ADK
    participant DocStore as Document Storage

    Note over Test,DocStore: Test Flow: TestAgentJobExecution_KeywordExtraction

    Test->>API: POST /api/documents (create test doc)
    API-->>Test: 201 Created (doc_id)
    
    Test->>API: POST /api/job-definitions (agent job def)
    API-->>Test: 201 Created (job_def_id)
    
    Test->>API: POST /api/job-definitions/{id}/execute
    API->>JobExec: Execute job definition
    JobExec->>AgentStep: ExecuteStep(agent step)
    
    AgentStep->>DocStore: Query documents (source_type filter)
    DocStore-->>AgentStep: Return [doc1, doc2, ...]
    
    loop For each document
        AgentStep->>Queue: Enqueue agent job
    end
    
    Queue->>AgentExec: Process agent job
    AgentExec->>DocStore: Load document
    DocStore-->>AgentExec: Return document
    
    AgentExec->>AgentSvc: Execute(keyword_extractor, input)
    AgentSvc->>KeywordExt: Execute(model, input)
    KeywordExt->>KeywordExt: Validate input
    KeywordExt->>ADK: llmagent.New() + runner.Run()
    ADK-->>KeywordExt: Return keywords JSON
    KeywordExt->>KeywordExt: Parse & validate response
    KeywordExt-->>AgentSvc: Return keywords + confidence
    AgentSvc-->>AgentExec: Return agent output
    
    AgentExec->>DocStore: Update metadata[keyword_extractor]
    DocStore-->>AgentExec: Confirm update
    
    AgentExec-->>Queue: Job completed
    
    Test->>API: GET /api/jobs/{parent_id} (poll)
    API-->>Test: Job status: completed
    
    Test->>API: GET /api/documents/{doc_id}
    API-->>Test: Document with metadata.keyword_extractor
    
    Test->>Test: Assert keywords exist & valid

## Proposed File Changes

### deployments\local\job-definitions\keyword-extractor-agent.toml(MODIFY)

References: 

- deployments\local\job-definitions\news-crawler.toml
- deployments\local\job-definitions\nearby-restaurants-places.toml

Create a job definition TOML file for the keyword extraction agent following the structure from `news-crawler.toml` and `nearby-restaurants-places.toml`.

**File Structure:**

1. **Header Comments** (lines 1-4):
   - Title: "Keyword Extractor Agent Job Definition"
   - Description: "Demonstrates keyword extraction agent with document processing"
   - Note: "Place .toml files in job-definitions/ directory to auto-load at startup"

2. **Job Metadata** (lines 6-18):
   - `id = "keyword-extractor-agent"` - Unique job identifier
   - `name = "Keyword Extractor Agent"` - Display name
   - `type = "agent"` - Job type (must match AgentExecutor.GetJobType())
   - `job_type = "user"` - User-triggered job
   - `source_type = "agent"` - Source type for filtering
   - `description = "Extracts keywords from documents using Google Gemini ADK"`
   - `tags = ["agent", "keywords", "nlp"]` - Tags for categorization
   - `schedule = ""` - Empty for manual execution only
   - `timeout = "10m"` - Maximum execution time
   - `enabled = true` - Job is enabled
   - `auto_start = false` - Do not auto-start on scheduler init

3. **Step 1: Keyword Extraction** (lines 20-35):
   - `[[steps]]` - First step definition
   - `name = "extract_keywords"` - Step name
   - `action = "agent"` - Action type (routes to AgentStepExecutor)
   - `on_error = "fail"` - Fail job on error
   - `[steps.config]` section:
     - `agent_type = "keyword_extractor"` - Agent type (must match KeywordExtractor.GetType())
     - `document_filter = { source_type = "crawler" }` - Filter documents by source type
     - `max_keywords = 10` - Maximum keywords to extract (optional, default: 10)
   - Add comment explaining that this step processes all documents matching the filter
   - Add comment about document_filter options: `source_type`, `limit`, etc.

4. **Agent Chaining Example (Commented Out)** (lines 37-50):
   - Add commented-out second step showing agent chaining:
     ```toml
     # Example: Chain keyword extraction with summarization (future agent)
     # [[steps]]
     # name = "generate_summary"
     # action = "agent"
     # on_error = "fail"
     # 
     # [steps.config]
     # agent_type = "summarizer"  # Future agent type
     # document_filter = { source_type = "crawler" }
     # use_keywords = true  # Access keywords from previous step via document metadata
     ```
   - Add comment explaining how chaining works:
     - Each agent stores results in `document.Metadata[agentType]`
     - Next agent can access previous results via metadata
     - Example: Summarizer reads `metadata["keyword_extractor"]["keywords"]`

5. **Usage Notes** (lines 52-60):
   - Add comments explaining:
     - This job definition processes existing documents (requires documents in database)
     - To test: First run a crawler job to populate documents, then run this agent job
     - Agent requires Google API key configured in `quaero.toml` under `[agent]` section
     - Results stored in document metadata under `keyword_extractor` key
     - View results via `GET /api/documents/{id}` endpoint

**Follow the exact TOML structure and formatting from `news-crawler.toml` for consistency.**

### test\api\agent_job_test.go(NEW)

References: 

- test\api\job_definition_execution_test.go
- test\common\setup.go
- internal\jobs\processor\agent_executor.go
- internal\jobs\executor\agent_step_executor.go

Create comprehensive API integration tests for agent job execution following the pattern from `job_definition_execution_test.go`.

**Package and Imports:**
- Package: `api`
- Imports: `testing`, `time`, `net/http`, `github.com/ternarybob/quaero/test/common`

**Test 1: TestAgentJobExecution_KeywordExtraction** (lines 10-150):

Verifies end-to-end keyword extraction via agent job.

1. **Setup** (lines 12-18):
   - Call `common.SetupTestEnvironment("TestAgentJobExecution_KeywordExtraction")`
   - Defer `env.Cleanup()`
   - Create `HTTPTestHelper` via `env.NewHTTPTestHelper(t)`

2. **Create Test Document** (lines 20-45):
   - Create document via `POST /api/documents` with:
     - `id`: "test-doc-agent-1"
     - `source_type`: "test"
     - `title`: "Test Document for Keyword Extraction"
     - `content_markdown`: Sample text (200+ words about technology, AI, machine learning)
     - `url`: "https://test.example.com/doc1"
   - Assert `http.StatusCreated`
   - Parse response and extract document ID
   - Defer `DELETE /api/documents/{id}` for cleanup

3. **Create Agent Job Definition** (lines 47-75):
   - Create job definition via `POST /api/job-definitions` with:
     - `id`: "test-agent-job-def-1"
     - `name`: "Test Agent Job - Keyword Extraction"
     - `type`: "agent"
     - `description`: "Test keyword extraction"
     - `enabled`: true
     - `steps`: Single step with:
       - `name`: "extract_keywords"
       - `action`: "agent"
       - `config`: `{"agent_type": "keyword_extractor", "document_filter": {"source_type": "test"}, "max_keywords": 10}`
       - `on_error`: "fail"
   - Assert `http.StatusCreated`
   - Parse response and extract job definition ID
   - Defer `DELETE /api/job-definitions/{id}` for cleanup

4. **Execute Job Definition** (lines 77-85):
   - Execute via `POST /api/job-definitions/{id}/execute`
   - Assert `http.StatusAccepted`
   - Log execution started

5. **Poll for Parent Job Creation** (lines 87-115):
   - Poll `GET /api/jobs` every 500ms for up to 30 seconds
   - Look for parent job with `job_type = "agent"` and `source_type = "agent"`
   - Extract parent job ID
   - Defer `DELETE /api/jobs/{id}` for cleanup
   - Assert parent job found

6. **Poll for Job Completion** (lines 117-135):
   - Poll `GET /api/jobs/{parentJobID}` every 2 seconds for up to 5 minutes
   - Check job status transitions: pending → running → completed
   - Break when status is "completed" or "failed"
   - Assert status is "completed"
   - Log completion time

7. **Verify Document Metadata Updated** (lines 137-150):
   - Fetch document via `GET /api/documents/{documentID}`
   - Parse response and extract `metadata` field
   - Assert `metadata["keyword_extractor"]` exists
   - Assert `metadata["keyword_extractor"]["keywords"]` is array with 5-15 elements
   - Assert keywords are non-empty strings
   - Optionally verify `metadata["keyword_extractor"]["confidence"]` exists
   - Log extracted keywords

**Test 2: TestAgentJobExecution_InvalidDocumentID** (lines 152-220):

Verifies error handling when document ID is invalid.

1. **Setup** (lines 154-160):
   - Call `common.SetupTestEnvironment("TestAgentJobExecution_InvalidDocumentID")`
   - Create `HTTPTestHelper`

2. **Create Job Definition with Invalid Document Filter** (lines 162-185):
   - Create job definition with `document_filter: {"source_type": "nonexistent"}`
   - This will result in no documents found

3. **Execute and Verify Graceful Handling** (lines 187-220):
   - Execute job definition
   - Poll for parent job
   - Verify job completes with status "completed" (not "failed")
   - Verify no child jobs created (no documents to process)
   - Log that job handled empty document set gracefully

**Test 3: TestAgentJobExecution_MissingAPIKey** (lines 222-280):

Verifies error handling when Google API key is missing.

**NOTE:** This test requires modifying the test environment to temporarily remove the API key. Since we cannot modify the running service configuration, this test should:

1. **Setup** (lines 224-230):
   - Call `common.SetupTestEnvironment("TestAgentJobExecution_MissingAPIKey")`
   - Add comment: "This test verifies agent service initialization fails without API key"
   - Add comment: "In production, agent jobs will fail if API key is not configured"

2. **Skip Test if Agent Service is Available** (lines 232-240):
   - Check if agent service is initialized by attempting to execute a test job
   - If agent service is available, skip test with message: "Agent service is configured, skipping API key validation test"
   - This test is primarily for documentation purposes

3. **Document Expected Behavior** (lines 242-280):
   - Add comments documenting:
     - Agent service initialization fails in `app.go` if `config.Agent.GoogleAPIKey` is empty
     - Error message: "Google API key is required for agent service"
     - Agent executors are not registered if service initialization fails
     - Agent jobs will return 404 or validation errors if agent service is unavailable
   - Log expected behavior for documentation

**Test 4: TestAgentJobExecution_MultipleDocuments** (lines 282-380):

Verifies agent processes multiple documents correctly.

1. **Setup** (lines 284-290):
   - Call `common.SetupTestEnvironment("TestAgentJobExecution_MultipleDocuments")`
   - Create `HTTPTestHelper`

2. **Create Multiple Test Documents** (lines 292-330):
   - Create 3 documents via `POST /api/documents` with different content:
     - Document 1: Technology content
     - Document 2: Healthcare content
     - Document 3: Finance content
   - Store document IDs in array
   - Defer cleanup for all documents

3. **Create and Execute Job Definition** (lines 332-355):
   - Create job definition with `document_filter: {"source_type": "test"}`
   - Execute job definition
   - Poll for parent job completion

4. **Verify All Documents Processed** (lines 357-380):
   - Fetch each document via `GET /api/documents/{id}`
   - Verify `metadata["keyword_extractor"]` exists for each
   - Verify keywords are different for each document (content-specific)
   - Log keyword counts for each document
   - Assert all documents were processed successfully

**Follow the exact test structure, assertions, and logging patterns from `job_definition_execution_test.go` for consistency.**

### test\unit\keyword_extractor_test.go(NEW)

References: 

- test\unit\arbor_channel_test.go
- internal\services\agents\keyword_extractor.go

Create unit tests for keyword extraction logic with mocked ADK responses following the pattern from `arbor_channel_test.go`.

**Package and Imports:**
- Package: `unit`
- Imports: `testing`, `context`, `encoding/json`, `github.com/ternarybob/quaero/internal/services/agents`

**IMPORTANT NOTE ON MOCKING:**

Google ADK's `model.LLM` interface is difficult to mock directly because it requires complex setup with `llmagent.New()` and `runner.Run()`. Instead, these tests should focus on:

1. **Input Validation Tests** - Test the validation logic in `KeywordExtractor.Execute()` without calling the actual ADK
2. **Response Parsing Tests** - Test the `parseKeywordResponse()` and `cleanMarkdownFences()` helper functions directly
3. **Integration Test Note** - Document that full ADK integration is tested in `agent_job_test.go`

**Test 1: TestKeywordExtractor_InputValidation** (lines 10-80):

Verifies input validation logic.

1. **Test Cases** (lines 12-50):
   - Define test table with cases:
     - Missing `document_id`: `input: {"content": "test"}`, `expectError: true`, `errorContains: "document_id is required"`
     - Empty `document_id`: `input: {"document_id": "", "content": "test"}`, `expectError: true`
     - Missing `content`: `input: {"document_id": "doc1"}`, `expectError: true`, `errorContains: "content is required"`
     - Empty `content`: `input: {"document_id": "doc1", "content": ""}`, `expectError: true`
     - Valid input: `input: {"document_id": "doc1", "content": "test content"}`, `expectError: false`
     - Valid with max_keywords (int): `input: {"document_id": "doc1", "content": "test", "max_keywords": 15}`, `expectError: false`
     - Valid with max_keywords (float64): `input: {"document_id": "doc1", "content": "test", "max_keywords": 15.0}`, `expectError: false`
     - Valid with max_keywords (string): `input: {"document_id": "doc1", "content": "test", "max_keywords": "15"}`, `expectError: false`

2. **Test Execution** (lines 52-80):
   - Loop through test cases
   - Create `KeywordExtractor` instance
   - Call `Execute()` with nil context and nil model (will fail at ADK call, but validation runs first)
   - For error cases: Assert error is not nil and contains expected substring
   - For valid cases: Document that full execution requires ADK integration test
   - Log test results

**Test 2: TestKeywordExtractor_ParseKeywordResponse** (lines 82-180):

Tests the `parseKeywordResponse()` helper function directly.

**NOTE:** This requires making `parseKeywordResponse()` exported (rename to `ParseKeywordResponse()`) or creating a test helper that calls it. For this plan, assume we create a test helper in `keyword_extractor.go`:

```go
// TestParseKeywordResponse is a test helper that exposes parseKeywordResponse for testing
func TestParseKeywordResponse(response string, maxKeywords int) ([]string, map[string]float64, error) {
    return parseKeywordResponse(response, maxKeywords)
}
```

1. **Test Cases** (lines 84-140):
   - Define test table with cases:
     - Simple array: `response: '["keyword1", "keyword2", "keyword3"]'`, `maxKeywords: 10`, `expectKeywords: ["keyword1", "keyword2", "keyword3"]`, `expectConfidence: nil`
     - Object with confidence: `response: '{"keywords": ["kw1", "kw2"], "confidence": {"kw1": 0.95, "kw2": 0.87}}'`, `expectKeywords: ["kw1", "kw2"]`, `expectConfidence: {"kw1": 0.95, "kw2": 0.87}`
     - Array exceeding max: `response: '["kw1", "kw2", "kw3", "kw4", "kw5"]'`, `maxKeywords: 3`, `expectKeywords: ["kw1", "kw2", "kw3"]` (truncated)
     - Object exceeding max: `response: '{"keywords": ["kw1", "kw2", "kw3"], "confidence": {"kw1": 0.9, "kw2": 0.8, "kw3": 0.7}}'`, `maxKeywords: 2`, `expectKeywords: ["kw1", "kw2"]`, `expectConfidence: {"kw1": 0.9, "kw2": 0.8}` (truncated)
     - Invalid JSON: `response: 'not json'`, `expectError: true`
     - Empty array: `response: '[]'`, `expectKeywords: []`

2. **Test Execution** (lines 142-180):
   - Loop through test cases
   - Call `agents.TestParseKeywordResponse(tc.response, tc.maxKeywords)`
   - For success cases:
     - Assert keywords match expected
     - Assert confidence matches expected (if provided)
     - Assert truncation works correctly
   - For error cases:
     - Assert error is not nil
   - Log test results with keyword counts

**Test 3: TestKeywordExtractor_CleanMarkdownFences** (lines 182-250):

Tests the `cleanMarkdownFences()` helper function.

**NOTE:** Similar to above, requires exposing the function or creating a test helper:

```go
// TestCleanMarkdownFences is a test helper that exposes cleanMarkdownFences for testing
func TestCleanMarkdownFences(s string) string {
    return cleanMarkdownFences(s)
}
```

1. **Test Cases** (lines 184-220):
   - Define test table with cases:
     - No fences: `input: '{"keywords": ["kw1"]}'`, `expected: '{"keywords": ["kw1"]}'`
     - With json fence: `input: '```json\n{"keywords": ["kw1"]}\n```'`, `expected: '{"keywords": ["kw1"]}'`
     - With JSON fence (uppercase): `input: '```JSON\n{"keywords": ["kw1"]}\n```'`, `expected: '{"keywords": ["kw1"]}'`
     - With plain fence: `input: '```\n{"keywords": ["kw1"]}\n```'`, `expected: '{"keywords": ["kw1"]}'`
     - With whitespace: `input: '  ```json\n{"keywords": ["kw1"]}\n```  '`, `expected: '{"keywords": ["kw1"]}'`
     - Multiple fences (only outer removed): `input: '```json\n{"code": "```inner```"}\n```'`, `expected: '{"code": "```inner```"}'`

2. **Test Execution** (lines 222-250):
   - Loop through test cases
   - Call `agents.TestCleanMarkdownFences(tc.input)`
   - Assert result matches expected
   - Log test results

**Test 4: TestKeywordExtractor_MaxKeywordsClamp** (lines 252-310):

Verifies that max_keywords is clamped to [5, 15] range.

1. **Test Cases** (lines 254-280):
   - Define test table with cases:
     - Below minimum: `max_keywords: 2`, `expectedClamped: 5`
     - At minimum: `max_keywords: 5`, `expectedClamped: 5`
     - In range: `max_keywords: 10`, `expectedClamped: 10`
     - At maximum: `max_keywords: 15`, `expectedClamped: 15`
     - Above maximum: `max_keywords: 20`, `expectedClamped: 15`
     - Negative: `max_keywords: -5`, `expectedClamped: 5`

2. **Test Execution** (lines 282-310):
   - Loop through test cases
   - Create input with test max_keywords value
   - Call `Execute()` (will fail at ADK, but clamping happens first)
   - Document that clamping is verified by checking the prompt construction
   - Alternative: Create a test helper that exposes the clamping logic
   - Log test results

**Documentation Comments** (lines 1-8):

Add file header comments:
- "Unit tests for KeywordExtractor agent"
- "These tests focus on input validation and response parsing logic"
- "Full ADK integration is tested in test/api/agent_job_test.go"
- "Note: Some tests require test helpers in keyword_extractor.go to expose internal functions"

**Follow the simple test structure from `arbor_channel_test.go` with clear test names, table-driven tests, and descriptive logging.**