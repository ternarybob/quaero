I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current Documentation State

**AGENTS.md (970 lines):**
- Comprehensive technical reference for AI agents
- Covers architecture (layered, event-driven, queue-based jobs)
- Documents LLM service, storage schema, Chrome extension
- Has sections for: Architecture Overview, Service Initialization, Data Flow, Code Conventions, Testing Guidelines, Common Development Tasks, Troubleshooting
- **Missing:** Agent framework architecture, Google ADK integration, agent job definitions

**README.md (1429 lines):**
- User-facing documentation with quick start, features, API reference
- Covers: Overview, Technology Stack, Quick Start, MCP Server, LLM Setup, Architecture, Commands, API Endpoints, Development, Configuration, Troubleshooting
- **Missing:** Agent framework overview, agent quick start, agent API endpoints

**job-definitions/README.md (179 lines):**
- Documents job definition format (TOML/JSON)
- Covers: File Format, Field Reference, Examples (crawler jobs only)
- **Missing:** Agent job definition format, agent-specific fields, agent chaining examples

## Agent Framework Components

**From Implementation Review:**

1. **AgentService** (`internal/services/agents/service.go`):
   - Manages Google ADK model lifecycle
   - Registers agent executors (keyword extractor, future agents)
   - Routes execution requests to appropriate agent
   - Health check validates API key and model initialization
   - No offline fallback - requires valid Google API key

2. **KeywordExtractor** (`internal/services/agents/keyword_extractor.go`):
   - Implements `AgentExecutor` interface
   - Uses ADK's `llmagent.New()` with agent loop pattern
   - Input: `document_id`, `content`, `max_keywords` (5-15 range)
   - Output: `keywords` array, `confidence` map
   - Stores results in `document.Metadata["keyword_extractor"]`

3. **AgentExecutor** (`internal/jobs/processor/agent_executor.go`):
   - Queue-based job executor (handles individual agent jobs)
   - Job type: `"agent"`
   - Workflow: Load document → Execute agent → Update metadata → Publish event
   - Real-time logging via `publishAgentJobLog()`

4. **AgentStepExecutor** (`internal/jobs/executor/agent_step_executor.go`):
   - Job definition step executor (creates agent jobs for documents)
   - Step action: `"agent"`
   - Supports document filtering via `document_filter` config
   - Enables agent chaining via sequential steps

5. **Configuration** (`internal/common/config.go`):
   - `AgentConfig` struct with fields: `GoogleAPIKey`, `ModelName`, `MaxTurns`, `Timeout`
   - Environment variables: `QUAERO_AGENT_GOOGLE_API_KEY`, etc.
   - Defaults: `gemini-2.0-flash`, `max_turns: 10`, `timeout: 5m`

6. **Job Definition** (`keyword-extractor-agent.toml`):
   - Job type: `"agent"`
   - Step action: `"agent"`
   - Config fields: `agent_type`, `document_filter`, `max_keywords`
   - Supports agent chaining via multiple steps

### Approach

## Documentation Strategy

**Update three documentation files** to comprehensively document the new agent framework with Google ADK integration. The approach follows the existing documentation patterns in each file while adding new sections for agent-specific content.

**Key Principles:**
1. **AGENTS.md** - Technical reference for AI agents working on the codebase (architecture, patterns, implementation details)
2. **README.md** - User-facing documentation (quick start, features, API reference)
3. **job-definitions/README.md** - Job definition format reference (TOML structure, field descriptions, examples)

**Documentation Scope:**
- Agent framework architecture and components
- Google ADK integration (API key setup, model configuration, no offline fallback)
- Agent job definition format (TOML structure, agent-specific fields)
- Keyword extractor agent (input/output schema, configuration options)
- Agent chaining patterns and best practices
- Quick start guide for users

**Why This Approach:**
- Maintains consistency with existing documentation structure
- Separates technical (AGENTS.md) from user-facing (README.md) documentation
- Provides comprehensive reference for both developers and users
- Includes practical examples and troubleshooting guidance

### Reasoning

I explored the agent framework implementation by reading `service.go`, `keyword_extractor.go`, `agent_executor.go`, and the job definition TOML. I examined the existing documentation structure in AGENTS.md (architecture sections, patterns, troubleshooting), README.md (quick start, features, API reference), and job-definitions/README.md (TOML format, field reference, examples). I identified where agent framework documentation should be inserted to maintain consistency with existing patterns.

## Proposed File Changes

### AGENTS.md(MODIFY)

References: 

- internal\services\agents\service.go
- internal\services\agents\keyword_extractor.go
- internal\jobs\processor\agent_executor.go
- internal\jobs\executor\agent_step_executor.go
- deployments\local\quaero.toml
- internal\common\config.go

**Add Agent Framework Architecture Section** (after line 389, after Chrome Extension section, before Go Structure Standards section):

Insert new section titled `### Agent Framework Architecture` with subsections:

1. **Overview** (5-10 lines):
   - Brief description: "Agent framework provides AI-powered document processing using Google ADK with Gemini models"
   - Key capabilities: keyword extraction, summarization (future), classification (future)
   - Integration: Queue-based job execution, document metadata storage, event publishing
   - No offline fallback: Requires valid Google API key

2. **Core Components** (30-40 lines):
   - **AgentService** (`internal/services/agents/service.go`):
     - Manages Google ADK model lifecycle with `gemini.NewModel()`
     - Registers agent executors in internal registry
     - Routes execution requests by agent type
     - Health check validates API key and model initialization
     - Constructor: `NewService(config *common.AgentConfig, logger arbor.ILogger)`
   
   - **AgentExecutor Interface** (internal to service):
     - `Execute(ctx context.Context, model model.LLM, input map[string]interface{}) (map[string]interface{}, error)`
     - `GetType() string` - Returns agent type identifier
     - Implemented by: `KeywordExtractor`, future agent types
   
   - **AgentExecutor** (`internal/jobs/processor/agent_executor.go`):
     - Queue-based job executor for individual agent jobs
     - Job type: `"agent"`
     - Workflow: Load document → Execute agent → Update metadata → Publish event
     - Real-time logging via `publishAgentJobLog()`
   
   - **AgentStepExecutor** (`internal/jobs/executor/agent_step_executor.go`):
     - Job definition step executor
     - Step action: `"agent"`
     - Creates agent jobs for documents matching filter
     - Supports agent chaining via sequential steps

3. **Agent Execution Flow** (20-30 lines):
   - Add sequence diagram showing:
     ```
     User triggers job → JobDefinition → JobExecutor → AgentStepExecutor
       ↓
     Query documents (document_filter) → Create agent jobs → Enqueue
       ↓
     Queue → JobProcessor → AgentExecutor
       ↓
     Load document → AgentService.Execute() → KeywordExtractor
       ↓
     ADK llmagent.New() → Gemini API → Parse response
       ↓
     Update document.Metadata[agent_type] → Publish event
     ```
   - Explain each step briefly
   - Note: Both queue-based and job definition execution paths supported

4. **Google ADK Integration** (25-35 lines):
   - **Model Initialization**:
     - Uses `gemini.NewModel(ctx, modelName, clientConfig)` from `google.golang.org/adk/model/gemini`
     - Client config: `APIKey`, `Backend: genai.BackendGeminiAPI`
     - Default model: `gemini-2.0-flash` (fast, cost-effective)
   
   - **Agent Loop Pattern**:
     - Uses `llmagent.New(config)` from `google.golang.org/adk/agent/llmagent`
     - Config includes: `Model`, `Name`, `Instruction`, `GenerateContentConfig`
     - Execution via `runner.New(config)` and `runner.Run(ctx, ...)`
     - Event stream processing with `IsFinalResponse()` check
   
   - **No Offline Fallback**:
     - Service initialization fails if `GoogleAPIKey` is empty
     - Error message: "Google API key is required for agent service"
     - Agent features unavailable if service initialization fails
     - Graceful degradation: Service logs warning, continues without agents

5. **Agent Types** (20-30 lines):
   - **Keyword Extractor** (`internal/services/agents/keyword_extractor.go`):
     - Type identifier: `"keyword_extractor"`
     - Input: `document_id`, `content`, `max_keywords` (5-15 range, default: 10)
     - Output: `keywords` array, `confidence` map (optional)
     - Metadata storage: `document.Metadata["keyword_extractor"]`
     - Prompt engineering: Instructs model to extract domain-specific terms, avoid stop words
     - Response parsing: Supports both simple array and object with confidence scores
   
   - **Future Agent Types**:
     - Summarizer: Generate document summaries
     - Classifier: Categorize documents by topic
     - Entity Extractor: Extract named entities (people, places, organizations)
     - Follow same `AgentExecutor` interface pattern

6. **Agent Chaining** (15-25 lines):
   - **How It Works**:
     - Multiple agent steps in job definition execute sequentially
     - Each agent stores results in `document.Metadata[agentType]`
     - Next agent can access previous results via metadata
     - Example: Keyword extractor → Summarizer (uses keywords for context)
   
   - **Configuration Pattern**:
     ```toml
     [[steps]]
     name = "extract_keywords"
     action = "agent"
     [steps.config]
     agent_type = "keyword_extractor"
     document_filter = { source_type = "crawler" }
     
     [[steps]]
     name = "generate_summary"
     action = "agent"
     [steps.config]
     agent_type = "summarizer"
     document_filter = { source_type = "crawler" }
     use_keywords = true  # Access metadata["keyword_extractor"]["keywords"]
     ```
   
   - **Best Practices**:
     - Use same `document_filter` for chained steps to ensure consistency
     - Order steps by dependency (keywords before summarization)
     - Monitor job logs for each step's completion

7. **Configuration** (15-20 lines):
   - **Agent Config Section** (`quaero.toml`):
     ```toml
     [agent]
     google_api_key = "YOUR_GOOGLE_GEMINI_API_KEY"  # Required
     model_name = "gemini-2.0-flash"                # Default
     max_turns = 10                                  # Agent conversation turns
     timeout = "5m"                                  # Execution timeout
     ```
   
   - **Environment Variables**:
     - `QUAERO_AGENT_GOOGLE_API_KEY` - Overrides config file
     - `QUAERO_AGENT_MODEL_NAME` - Overrides model name
     - `QUAERO_AGENT_MAX_TURNS` - Overrides max turns
     - `QUAERO_AGENT_TIMEOUT` - Overrides timeout
   
   - **API Key Setup**:
     - Get API key from: https://aistudio.google.com/app/apikey
     - Free tier available with rate limits
     - Store in config file or environment variable

8. **Service Initialization** (10-15 lines):
   - Add to existing "Service Initialization Flow" section (line 248):
   - Insert after line 263 (after Scheduler Service):
     - **13. Agent Service** - Google ADK with Gemini models (optional, requires API key)
   - Update initialization order documentation:
     - Agent service initialized after EventService (line 256)
     - AgentExecutor registered with JobProcessor (line 319-330 in `app.go`)
     - AgentStepExecutor registered with JobExecutor (line 398-403 in `app.go`)
     - Conditional registration: Only if `AgentService != nil`

**Follow the existing documentation style**: Use code blocks for examples, bullet points for lists, clear section headers, and cross-references to relevant files.
**Update Troubleshooting Section** (after line 869, in existing Troubleshooting section):

Add new subsection titled `### Agent Service Issues` with troubleshooting guidance:

1. **Agent Service Not Initialized** (10-15 lines):
   - **Symptom**: Log message "Failed to initialize agent service - agent features will be unavailable"
   - **Cause**: Missing or invalid Google API key
   - **Solution**:
     - Check `quaero.toml` has `[agent]` section with `google_api_key` set
     - Or set environment variable: `QUAERO_AGENT_GOOGLE_API_KEY=your_key_here`
     - Get API key from: https://aistudio.google.com/app/apikey
     - Verify API key is valid (not expired, not revoked)
   - **Verification**: Look for log message "Agent service initialized with Google ADK"

2. **Agent Jobs Fail with "Unknown Agent Type"** (8-12 lines):
   - **Symptom**: Job fails with error "unknown agent type: {type}"
   - **Cause**: Agent type not registered or typo in job definition
   - **Solution**:
     - Check job definition `agent_type` matches registered agent (e.g., `"keyword_extractor"`)
     - Verify agent is registered in `service.go` `NewService()` function
     - Check service logs for "Agent registered" messages at startup
   - **Available Types**: `keyword_extractor` (more coming soon)

3. **Agent Execution Timeout** (8-12 lines):
   - **Symptom**: Job fails with "context deadline exceeded" error
   - **Cause**: Agent execution exceeds configured timeout (default: 5m)
   - **Solution**:
     - Increase timeout in `quaero.toml`: `[agent] timeout = "10m"`
     - Or set environment variable: `QUAERO_AGENT_TIMEOUT=10m`
     - Check document size (large documents take longer to process)
     - Monitor Gemini API rate limits (may cause delays)
   - **Note**: Timeout applies per agent execution, not per job

4. **Keywords Not Appearing in Document Metadata** (10-15 lines):
   - **Symptom**: Agent job completes but `metadata["keyword_extractor"]` is empty
   - **Cause**: Document not updated or agent returned no keywords
   - **Solution**:
     - Check job logs for "Document metadata updated successfully" message
     - Verify document has sufficient content (minimum ~100 words recommended)
     - Check agent response in logs for malformed JSON
     - Query document via `GET /api/documents/{id}` to verify metadata
   - **Metadata Structure**:
     ```json
     {
       "keyword_extractor": {
         "keywords": ["keyword1", "keyword2", ...],
         "confidence": {"keyword1": 0.95, ...}
       }
     }
     ```

5. **Gemini API Rate Limit Errors** (8-12 lines):
   - **Symptom**: Job fails with "429 Too Many Requests" or "quota exceeded" error
   - **Cause**: Exceeded Gemini API free tier rate limits
   - **Solution**:
     - Reduce job concurrency in `quaero.toml`: `[queue] concurrency = 2`
     - Add delays between agent jobs (not currently supported, future feature)
     - Upgrade to paid Gemini API tier for higher limits
     - Monitor API usage at: https://aistudio.google.com/app/apikey
   - **Free Tier Limits**: 15 requests per minute, 1500 requests per day (as of 2024)

**Follow the existing troubleshooting format**: Use bold for symptom/cause/solution, provide specific error messages, include verification steps.

### README.md(MODIFY)

References: 

- deployments\local\quaero.toml
- deployments\local\job-definitions\keyword-extractor-agent.toml

**Add Agent Framework to Key Features Section** (after line 21, in existing Key Features list):

Add new bullet point:
- 🤖 **AI Agents** - Google ADK-powered document processing (keyword extraction, summarization)

**Add Agent Framework Quick Start Section** (after line 208, after Chrome Extension installation, before LLM Setup section):

Insert new section titled `## Agent Framework (Google ADK)` with subsections:

1. **Overview** (5-8 lines):
   - Brief description: "Quaero includes an AI agent framework powered by Google ADK (Agent Development Kit) with Gemini models for intelligent document processing."
   - Current capabilities: Keyword extraction from documents
   - Future capabilities: Summarization, classification, entity extraction
   - Note: Requires Google Gemini API key (free tier available)

2. **Quick Setup** (15-20 lines):
   - **Step 1: Get Google API Key**:
     - Visit: https://aistudio.google.com/app/apikey
     - Create new API key (free tier available)
     - Copy API key for configuration
   
   - **Step 2: Configure Quaero**:
     - Add to `quaero.toml`:
       ```toml
       [agent]
       google_api_key = "YOUR_GOOGLE_GEMINI_API_KEY"
       model_name = "gemini-2.0-flash"  # Default, fast model
       timeout = "5m"
       ```
     - Or set environment variable:
       ```bash
       export QUAERO_AGENT_GOOGLE_API_KEY="your_key_here"
       ```
   
   - **Step 3: Verify Setup**:
     - Start Quaero and check logs for: "Agent service initialized with Google ADK"
     - If missing API key, logs show: "Failed to initialize agent service - agent features will be unavailable"

3. **Using Agents** (20-30 lines):
   - **Keyword Extraction Example**:
     - Create job definition in `job-definitions/` directory:
       ```toml
       id = "keyword-extractor"
       name = "Keyword Extractor"
       type = "agent"
       
       [[steps]]
       name = "extract_keywords"
       action = "agent"
       
       [steps.config]
       agent_type = "keyword_extractor"
       [steps.config.document_filter]
       source_type = "crawler"  # Process crawler documents
       max_keywords = 10
       ```
   
   - **Run Agent Job**:
     - Via Web UI: Navigate to Jobs page, click "Run" on keyword extractor job
     - Via API: `POST /api/job-definitions/keyword-extractor/execute`
     - Monitor progress in real-time via WebSocket logs
   
   - **View Results**:
     - Keywords stored in document metadata: `GET /api/documents/{id}`
     - Metadata structure:
       ```json
       {
         "keyword_extractor": {
           "keywords": ["keyword1", "keyword2", ...],
           "confidence": {"keyword1": 0.95, "keyword2": 0.87}
         }
       }
       ```

4. **Agent Chaining** (15-20 lines):
   - **Concept**: Chain multiple agents in sequence (e.g., keyword extraction → summarization)
   - **Example**:
     ```toml
     [[steps]]
     name = "extract_keywords"
     action = "agent"
     [steps.config]
     agent_type = "keyword_extractor"
     [steps.config.document_filter]
     source_type = "crawler"
     
     [[steps]]
     name = "generate_summary"
     action = "agent"
     [steps.config]
     agent_type = "summarizer"  # Future agent type
     [steps.config.document_filter]
     source_type = "crawler"
     use_keywords = true  # Access keywords from previous step
     ```
   - **How It Works**: Each agent stores results in `document.Metadata[agentType]`, next agent can access previous results
   - **Use Cases**: Keyword extraction before summarization, classification before entity extraction

5. **Available Agents** (10-15 lines):
   - **Keyword Extractor** (`keyword_extractor`):
     - Extracts 5-15 relevant keywords from document content
     - Input: Document content (markdown)
     - Output: Keywords array + confidence scores
     - Configuration: `max_keywords` (default: 10, range: 5-15)
   
   - **Coming Soon**:
     - Summarizer: Generate document summaries
     - Classifier: Categorize documents by topic
     - Entity Extractor: Extract named entities

6. **API Key & Pricing** (8-12 lines):
   - **Free Tier**: 15 requests/minute, 1500 requests/day (as of 2024)
   - **Paid Tiers**: Higher rate limits, more models available
   - **Privacy**: API calls send document content to Google servers (not suitable for sensitive data)
   - **Alternative**: Use offline LLM mode for embeddings/chat (agents require Google ADK)
   - **Monitoring**: Track usage at https://aistudio.google.com/app/apikey

**Follow the existing README style**: Use clear section headers, code blocks for examples, bullet points for lists, and practical examples.
**Update API Endpoints Section** (find the section documenting API endpoints, likely around line 900-1000):

Add new subsection titled `**Agent Jobs:**` with agent-specific endpoints:

1. **Agent Job Execution Endpoints** (15-20 lines):
   - `POST /api/job-definitions/{id}/execute` - Execute agent job definition
     - Request body: Empty (uses job definition configuration)
     - Response: `202 Accepted` with parent job ID
     - Example:
       ```bash
       curl -X POST http://localhost:8085/api/job-definitions/keyword-extractor-agent/execute
       ```
   
   - `GET /api/jobs/{id}` - Get agent job status
     - Response: Job model with status, progress, logs
     - Status values: `pending`, `running`, `completed`, `failed`
     - Example:
       ```bash
       curl http://localhost:8085/api/jobs/{job_id}
       ```
   
   - `GET /api/documents/{id}` - Get document with agent metadata
     - Response: Document model with `metadata` field containing agent results
     - Metadata structure: `metadata["keyword_extractor"]["keywords"]`
     - Example:
       ```bash
       curl http://localhost:8085/api/documents/{doc_id}
       ```

2. **Agent Service Health Check** (5-8 lines):
   - `GET /api/health` - Check agent service status
     - Response includes agent service health in `services` object
     - Example response:
       ```json
       {
         "status": "healthy",
         "services": {
           "agent": "healthy",
           "llm": "healthy",
           "storage": "healthy"
         }
       }
       ```

**Follow the existing API documentation format**: Use HTTP method + endpoint, describe request/response, provide curl examples.

### deployments\local\job-definitions\README.md(MODIFY)

References: 

- deployments\local\job-definitions\keyword-extractor-agent.toml
- deployments\local\job-definitions\news-crawler.toml

**Add Agent Job Definition Section** (after line 163, after existing examples, before Notes section):

Insert new section titled `## Agent Job Definitions` with subsections:

1. **Overview** (5-8 lines):
   - Brief description: "Agent jobs process existing documents using AI agents powered by Google ADK with Gemini models."
   - Use cases: Keyword extraction, summarization, classification, entity extraction
   - Requirements: Google Gemini API key configured in `quaero.toml`
   - Note: Agent jobs do NOT crawl new content - they process existing documents in the database

2. **Agent Job Format** (30-40 lines):
   - **Required Fields**:
     - `id` (string): Unique job identifier
     - `name` (string): Human-readable job name
     - `type` (string): Must be `"agent"` for agent jobs
     - `steps` (array): Array of agent steps to execute
   
   - **Agent Step Configuration**:
     - `name` (string): Step name for logging
     - `action` (string): Must be `"agent"` for agent steps
     - `on_error` (string): Error handling strategy (`"fail"`, `"continue"`, `"retry"`)
     - `config` (object): Agent-specific configuration
   
   - **Agent Step Config Fields**:
     - `agent_type` (string): Agent identifier (e.g., `"keyword_extractor"`)
     - `document_filter` (object): Filter documents to process
       - `source_type` (string): Filter by source type (e.g., `"crawler"`, `"places"`)
       - `limit` (integer): Maximum documents to process (optional)
       - `created_after` (string): Filter by creation date (optional)
       - `updated_after` (string): Filter by update date (optional)
     - Agent-specific parameters (e.g., `max_keywords` for keyword extractor)
   
   - **Optional Fields**:
     - `description` (string): Job description
     - `schedule` (string): Cron expression for scheduling (default: "" = manual only)
     - `timeout` (string): Maximum execution time (default: "10m")
     - `enabled` (boolean): Whether job is enabled (default: true)
     - `auto_start` (boolean): Auto-start on scheduler init (default: false)
     - `tags` (array): Tags for categorization

3. **Keyword Extractor Example** (25-35 lines):
   ```toml
   # Keyword Extractor Agent Job Definition
   id = "keyword-extractor-agent"
   name = "Keyword Extractor Agent"
   type = "agent"
   job_type = "user"
   source_type = "agent"
   description = "Extracts keywords from documents using Google Gemini ADK"
   
   # Tags for categorization
   tags = ["agent", "keywords", "nlp"]
   
   # Cron schedule (empty = manual execution only)
   schedule = ""
   
   # Maximum execution time
   timeout = "10m"
   
   # Whether this job is enabled
   enabled = true
   
   # Whether to auto-start when scheduler initializes
   auto_start = false
   
   # Job steps definition
   [[steps]]
   name = "extract_keywords"
   action = "agent"
   on_error = "fail"
   
   [steps.config]
   # Agent type (must match registered agent)
   agent_type = "keyword_extractor"
   
   # Filter documents to process
   [steps.config.document_filter]
   source_type = "crawler"  # Process documents from crawler jobs
   
   # Optional: Maximum keywords to extract (default: 10, range: 5-15)
   max_keywords = 10
   ```

4. **Agent Chaining Example** (30-40 lines):
   ```toml
   # Agent Chaining Example: Keyword Extraction → Summarization
   id = "keyword-and-summary-agent"
   name = "Keyword Extraction + Summarization"
   type = "agent"
   description = "Extracts keywords then generates summaries using those keywords"
   
   # Step 1: Extract keywords
   [[steps]]
   name = "extract_keywords"
   action = "agent"
   on_error = "fail"
   
   [steps.config]
   agent_type = "keyword_extractor"
   [steps.config.document_filter]
   source_type = "crawler"
   max_keywords = 10
   
   # Step 2: Generate summary (uses keywords from step 1)
   [[steps]]
   name = "generate_summary"
   action = "agent"
   on_error = "fail"
   
   [steps.config]
   agent_type = "summarizer"  # Future agent type
   [steps.config.document_filter]
   source_type = "crawler"  # Same filter as step 1
   use_keywords = true  # Access metadata["keyword_extractor"]["keywords"]
   ```
   
   **How Chaining Works**:
   - Each agent stores results in `document.Metadata[agentType]`
   - Next agent can access previous results via metadata
   - Example: Summarizer reads `metadata["keyword_extractor"]["keywords"]` to use keywords for context
   - Use same `document_filter` for chained steps to ensure consistency

5. **Available Agent Types** (15-20 lines):
   - **`keyword_extractor`**:
     - Extracts 5-15 relevant keywords from document content
     - Config parameters:
       - `max_keywords` (integer): Maximum keywords to extract (default: 10, range: 5-15)
     - Output stored in: `document.Metadata["keyword_extractor"]`
     - Output structure:
       ```json
       {
         "keywords": ["keyword1", "keyword2", ...],
         "confidence": {"keyword1": 0.95, "keyword2": 0.87}
       }
       ```
   
   - **Coming Soon**:
     - `summarizer`: Generate document summaries
     - `classifier`: Categorize documents by topic
     - `entity_extractor`: Extract named entities (people, places, organizations)

6. **Agent Job Best Practices** (15-20 lines):
   - **Document Filtering**:
     - Use specific `source_type` filters to target relevant documents
     - Add `limit` to test on small document sets first
     - Use `created_after` or `updated_after` for incremental processing
   
   - **Error Handling**:
     - Use `on_error = "fail"` for critical steps (default)
     - Use `on_error = "continue"` to skip failed documents and continue processing
     - Monitor job logs for errors and adjust configuration
   
   - **Performance**:
     - Agent jobs process documents sequentially (one at a time per job)
     - Reduce `document_filter` scope for faster execution
     - Monitor Gemini API rate limits (15 requests/minute on free tier)
     - Increase `timeout` for large document sets
   
   - **Agent Chaining**:
     - Order steps by dependency (keywords before summarization)
     - Use same `document_filter` for all chained steps
     - Test each step individually before chaining

**Follow the existing README format**: Use TOML code blocks, clear section headers, field descriptions with defaults, and practical examples.
**Update Troubleshooting Section** (after line 177, in existing Troubleshooting section):

Add new subsection for agent job troubleshooting:

**Agent job not executing:**
- Check Google API key is configured in `quaero.toml` under `[agent]` section
- Verify agent service initialized successfully (check service logs for "Agent service initialized with Google ADK")
- Ensure `agent_type` matches registered agent (e.g., `"keyword_extractor"`)
- Verify documents exist matching `document_filter` criteria
- Check job logs for detailed error messages

**Agent job completes but no metadata:**
- Verify document has sufficient content (minimum ~100 words recommended)
- Check job logs for "Document metadata updated successfully" message
- Query document via `GET /api/documents/{id}` to verify metadata structure
- Ensure agent returned valid response (check logs for malformed JSON errors)

**Gemini API rate limit errors:**
- Reduce job concurrency in `quaero.toml`: `[queue] concurrency = 2`
- Add `limit` to `document_filter` to process fewer documents per job
- Upgrade to paid Gemini API tier for higher rate limits
- Monitor API usage at: https://aistudio.google.com/app/apikey

**Follow the existing troubleshooting format**: Use bold for issue description, bullet points for solutions.