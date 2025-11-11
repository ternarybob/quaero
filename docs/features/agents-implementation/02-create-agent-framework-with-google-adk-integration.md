I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current Architecture

**Job Execution Flow:**
- JobDefinition (TOML) → JobExecutor → StepExecutor → Service → Storage
- Queue-based: JobProcessor polls queue, routes to registered executors by job type
- Executors implement `interfaces.JobExecutor` with `Execute()`, `GetJobType()`, `Validate()`

**Document Metadata Pattern:**
- `Document.Metadata` is a `map[string]interface{}` stored as JSON in SQLite
- No dedicated methods for partial metadata updates - use `UpdateDocument()` or `SaveDocument()`
- Metadata is flexible and schema-less, perfect for agent-generated data

**Configuration Pattern:**
- Config structs in `internal/common/config.go` with TOML tags
- Environment variable overrides in `applyEnvOverrides()` function
- Validation happens in service constructors (fail fast on missing required config)

**Service Initialization:**
- Services created in `app.initServices()` with dependency injection
- Executors registered with `JobProcessor.RegisterExecutor()`
- Step executors registered with `JobExecutor.RegisterStepExecutor()`

**Google ADK Requirements:**
- Module: `google.golang.org/adk` (v0.1.0+)
- Packages: `google.golang.org/adk/agent/llmagent`, `google.golang.org/adk/model/gemini`, `google.golang.org/genai`
- API Key: Can be passed via `genai.ClientConfig.APIKey` or `GOOGLE_API_KEY` env var
- Model initialization: `gemini.NewModel(ctx, modelName, clientConfig)` → `llmagent.New(config)`

### Approach

## Agent Framework with Google ADK Integration

**Core Strategy:** Create a new agent service layer that uses Google's ADK (Agent Development Kit) with Gemini models to process documents. The framework follows Quaero's established patterns: interface-based design, dependency injection, queue-based job execution, and TOML configuration.

**Key Design Decisions:**

1. **No Fallback Policy**: If Google API key is missing or invalid, agent jobs fail immediately. No offline/mock mode fallback.

2. **Metadata Storage**: Keywords extracted by agents are stored in the existing `Document.Metadata` map (e.g., `metadata["keywords"] = ["keyword1", "keyword2"]`), avoiding new database tables.

3. **Job Execution Path**: Agent jobs follow the standard queue-based execution: JobDefinition → Queue → JobProcessor → AgentExecutor → AgentService → Google ADK.

4. **Agent Chaining Support**: Job definitions can chain multiple agent steps (e.g., keyword extraction → summarization), with output from one agent passed as input to the next.

5. **Reuse Existing Patterns**: Follow the same patterns as `CrawlerExecutor` and `PlacesSearchStepExecutor` for consistency.

### Reasoning

I explored the codebase structure by reading configuration files (`config.go`, `quaero.toml`), examining existing job executors (`crawler_executor.go`, `places_search_step_executor.go`), reviewing interfaces (`job_executor.go`, `storage.go`, `llm_service.go`), understanding the document model and metadata storage (`document.go`, `document_storage.go`), and researching Google ADK integration patterns via web search. I also examined the job processor architecture and how services are initialized in `app.go`.

## Mermaid Diagram

sequenceDiagram
    participant User
    participant JobDef as Job Definition<br/>(TOML)
    participant Queue as Queue Manager<br/>(goqite)
    participant Processor as Job Processor
    participant AgentExec as Agent Executor
    participant AgentSvc as Agent Service
    participant ADK as Google ADK<br/>(llmagent)
    participant Gemini as Gemini API
    participant DocStore as Document Storage

    User->>JobDef: Create agent job definition<br/>(keyword-extractor-agent.toml)
    JobDef->>Queue: Enqueue agent job<br/>(type: "agent")
    Queue->>Processor: Poll for jobs
    Processor->>AgentExec: Route to AgentExecutor<br/>(job type: "agent")
    
    AgentExec->>DocStore: Load document by ID
    DocStore-->>AgentExec: Return document<br/>(content, metadata)
    
    AgentExec->>AgentSvc: Execute agent<br/>(type: "keyword_extractor")
    AgentSvc->>ADK: Create llmagent<br/>(instruction + content)
    ADK->>Gemini: Send prompt<br/>(extract keywords)
    Gemini-->>ADK: Return keywords JSON<br/>{"keywords": [...], "confidence": {...}}
    ADK-->>AgentSvc: Parse response
    AgentSvc-->>AgentExec: Return keywords + confidence
    
    AgentExec->>DocStore: Update document metadata<br/>(keywords, confidence, timestamp)
    DocStore-->>AgentExec: Confirm save
    
    AgentExec->>Processor: Mark job completed
    Processor->>Queue: Delete message
    Queue-->>User: Job completed<br/>(keywords in metadata)

## Proposed File Changes

### go.mod(MODIFY)

Add Google ADK dependencies to the `require` section:

- `google.golang.org/adk v0.1.0` - Agent Development Kit core module
- `google.golang.org/genai v0.19.0` - Gemini API client library

These dependencies provide the `llmagent` package for agent creation and the `gemini` model integration. The ADK handles agent loops, tool use, and streaming responses. The genai package provides the underlying Gemini API client.

After adding these lines, run `go mod tidy` to download dependencies and update `go.sum`.

Note: Version numbers should be verified against the latest stable releases at the time of implementation. Use `go get google.golang.org/adk@latest` to fetch the most recent version.

### internal\common\config.go(MODIFY)

References: 

- deployments\local\quaero.toml(MODIFY)

**Add AgentConfig struct** (after line 139, before `NewDefaultConfig()`):

Create a new `AgentConfig` struct with fields:
- `GoogleAPIKey string` (TOML tag: `google_api_key`) - Gemini API key for agent operations
- `ModelName string` (TOML tag: `model_name`) - Gemini model identifier (default: "gemini-2.0-flash")
- `MaxTurns int` (TOML tag: `max_turns`) - Maximum agent conversation turns (default: 10)
- `Timeout string` (TOML tag: `timeout`) - Agent execution timeout as duration string (default: "5m")

**Add Agent field to Config struct** (line 27, after `PlacesAPI`):

Add `Agent AgentConfig` with TOML tag `agent` to the main `Config` struct.

**Add default values in NewDefaultConfig()** (line 239, after PlacesAPI defaults):

Initialize `Agent` field with:
- `GoogleAPIKey: ""` (empty, must be provided by user)
- `ModelName: "gemini-2.0-flash"` (fast, cost-effective model)
- `MaxTurns: 10` (reasonable limit for agent loops)
- `Timeout: "5m"` (5 minutes for agent execution)

**Add environment variable overrides in applyEnvOverrides()** (line 483, after Places API overrides):

Add environment variable support:
- `QUAERO_AGENT_GOOGLE_API_KEY` → `config.Agent.GoogleAPIKey`
- `QUAERO_AGENT_MODEL_NAME` → `config.Agent.ModelName`
- `QUAERO_AGENT_MAX_TURNS` → `config.Agent.MaxTurns` (parse as int)
- `QUAERO_AGENT_TIMEOUT` → `config.Agent.Timeout`

Follow the same pattern as existing environment variable overrides (lines 479-482 for Places API).

### deployments\local\quaero.toml(MODIFY)

References: 

- internal\common\config.go(MODIFY)

**Add Agent Configuration Section** (after line 64, after Places API section):

Add a new `[agent]` section with:

```toml
# =============================================================================
# Agent Configuration (Google ADK with Gemini)
# =============================================================================
# Configure AI agents for document processing (keyword extraction, summarization, etc.)
# Requires Google Gemini API key from: https://aistudio.google.com/app/apikey
#
# IMPORTANT: Agents require a valid API key. No offline fallback is available.
# If the API key is missing or invalid, agent jobs will fail immediately.
#
# Env vars: QUAERO_AGENT_GOOGLE_API_KEY, QUAERO_AGENT_MODEL_NAME, QUAERO_AGENT_MAX_TURNS, QUAERO_AGENT_TIMEOUT

[agent]
# google_api_key = "YOUR_GOOGLE_GEMINI_API_KEY"  # Required for agent operations
# model_name = "gemini-2.0-flash"                 # Gemini model (default: gemini-2.0-flash)
# max_turns = 10                                   # Maximum agent conversation turns (default: 10)
# timeout = "5m"                                   # Agent execution timeout (default: 5m)
```

This section documents the agent configuration options and provides clear guidance that the API key is required. The commented-out values show the defaults from `config.go`.

### internal\interfaces\agent_service.go(NEW)

References: 

- internal\interfaces\llm_service.go
- internal\interfaces\places_service.go

Create the `AgentService` interface defining the contract for agent operations.

**Package declaration:** `package interfaces`

**Imports:** `context`

**Interface definition:**

Define `AgentService` interface with methods:

1. `Execute(ctx context.Context, agentType string, input map[string]interface{}) (map[string]interface{}, error)`
   - Executes an agent of the specified type with the given input
   - `agentType`: Agent identifier (e.g., "keyword_extractor", "summarizer")
   - `input`: Agent-specific input data (e.g., `{"document_id": "doc_123", "content": "..."}`)
   - Returns: Agent output as a map (e.g., `{"keywords": ["keyword1", "keyword2"], "confidence": {...}}`)
   - Errors: Returns error if agent type is unknown, API call fails, or response is malformed

2. `HealthCheck(ctx context.Context) error`
   - Verifies the agent service is operational and can communicate with Google ADK
   - Returns: nil if healthy, error with details if unhealthy

3. `Close() error`
   - Releases resources and performs cleanup
   - Returns: error if cleanup fails

**Documentation:**

Add comprehensive doc comments explaining:
- The interface provides a unified API for executing different agent types
- Agents are powered by Google ADK with Gemini models
- No offline fallback - requires valid Google API key
- Agent types are registered at service initialization
- Input/output formats are agent-specific and documented per agent type

### internal\services\agents(NEW)

Create the `agents` service directory to house the agent service implementation and agent type implementations.

This directory will contain:
- `service.go` - Main agent service implementation
- `keyword_extractor.go` - Keyword extraction agent (first agent type)
- Future agent implementations (summarizer, classifier, etc.)

The directory structure follows the existing pattern used by other services like `crawler/`, `places/`, `documents/`.

### internal\services\agents\service.go(NEW)

References: 

- internal\services\places\service.go
- internal\interfaces\agent_service.go(NEW)
- internal\common\config.go(MODIFY)

Implement the main `AgentService` that manages Google ADK agent lifecycle and execution.

**Package declaration:** `package agents`

**Imports:**
- Standard: `context`, `fmt`, `time`
- Google ADK: `google.golang.org/adk/agent/llmagent`, `google.golang.org/adk/model/gemini`, `google.golang.org/genai`
- Internal: `github.com/ternarybob/arbor`, `github.com/ternarybob/quaero/internal/common`, `github.com/ternarybob/quaero/internal/interfaces`

**Service struct:**

Define `Service` struct with fields:
- `config *common.AgentConfig` - Agent configuration
- `logger arbor.ILogger` - Structured logger
- `model model.LLM` - Gemini model instance from ADK
- `agents map[string]AgentExecutor` - Registered agent executors by type
- `timeout time.Duration` - Parsed timeout duration

**AgentExecutor interface (internal):**

Define internal interface for agent type implementations:
- `Execute(ctx context.Context, model model.LLM, input map[string]interface{}) (map[string]interface{}, error)`
- `GetType() string`

This allows different agent types (keyword extractor, summarizer, etc.) to be registered and executed uniformly.

**Constructor: NewService(config *common.AgentConfig, logger arbor.ILogger) (*Service, error)**

1. **Validate configuration:**
   - Check `config.GoogleAPIKey` is not empty - return error with message "Google API key is required for agent service (set QUAERO_AGENT_GOOGLE_API_KEY or agent.google_api_key in config)"
   - Check `config.ModelName` is not empty - default to "gemini-2.0-flash" if missing
   - Parse `config.Timeout` as duration - return error if invalid

2. **Initialize Gemini model:**
   - Create `genai.ClientConfig` with `APIKey: config.GoogleAPIKey` and `Backend: genai.BackendGeminiAPI`
   - Call `gemini.NewModel(context.Background(), config.ModelName, clientConfig)`
   - Return error if model creation fails (e.g., invalid API key, network error)

3. **Initialize service:**
   - Create `Service` instance with initialized model and empty agents map
   - Log successful initialization with model name

4. **Register built-in agents:**
   - Create `KeywordExtractor` instance (from `keyword_extractor.go`)
   - Register it with `service.RegisterAgent(keywordExtractor)`
   - Log registered agent types

**Method: RegisterAgent(agent AgentExecutor)**

Add agent to the `agents` map keyed by `agent.GetType()`. Log registration.

**Method: Execute(ctx context.Context, agentType string, input map[string]interface{}) (map[string]interface{}, error)**

1. Look up agent executor in `agents` map by `agentType`
2. Return error if agent type not found: "unknown agent type: {agentType}"
3. Create timeout context from `service.timeout`
4. Call `agent.Execute(ctx, service.model, input)`
5. Return result or error
6. Log execution with agent type, duration, and success/failure

**Method: HealthCheck(ctx context.Context) error**

1. Create a simple test prompt: "Respond with 'OK' if you can read this."
2. Create `llmagent.Config` with `Model: service.model`, `Name: "health_check"`, `Instruction: testPrompt`
3. Call `llmagent.New(config)` to create a test agent
4. Return error if agent creation fails
5. Log successful health check
6. Return nil

**Method: Close() error**

1. Log service shutdown
2. Set `service.model` to nil (ADK handles cleanup internally)
3. Return nil

Follow the pattern from `internal/services/places/service.go` for service structure and error handling.

### internal\services\agents\keyword_extractor.go(NEW)

References: 

- internal\services\agents\service.go(NEW)

Implement the keyword extraction agent using Google ADK's llmagent.

**Package declaration:** `package agents`

**Imports:**
- Standard: `context`, `encoding/json`, `fmt`, `strings`
- Google ADK: `google.golang.org/adk/agent/llmagent`, `google.golang.org/adk/model`

**KeywordExtractor struct:**

Define `KeywordExtractor` struct (empty, stateless).

**Method: GetType() string**

Return `"keyword_extractor"`.

**Method: Execute(ctx context.Context, model model.LLM, input map[string]interface{}) (map[string]interface{}, error)**

1. **Extract and validate input:**
   - Extract `content` field from input map as string
   - Return error if `content` is missing or empty: "content is required for keyword extraction"
   - Optionally extract `max_keywords` (default: 10) and `min_keywords` (default: 5)

2. **Build agent instruction:**
   - Create instruction string:
     ```
     You are a keyword extraction specialist. Analyze the provided document content and extract 5-15 relevant keywords that best represent the document's main topics and themes.
     
     Requirements:
     - Extract between {min_keywords} and {max_keywords} keywords
     - Keywords should be single words or short phrases (2-3 words max)
     - Focus on nouns, technical terms, and domain-specific concepts
     - Avoid generic words like "the", "and", "is"
     - Return keywords in order of relevance (most relevant first)
     
     Return your response as a JSON object with this exact structure:
     {
       "keywords": ["keyword1", "keyword2", ...],
       "confidence": {"keyword1": 0.95, "keyword2": 0.87, ...}
     }
     
     Document content:
     {content}
     ```
   - Truncate content to first 4000 characters if longer (to avoid token limits)

3. **Create and configure agent:**
   - Create `llmagent.Config` with:
     - `Model: model`
     - `Name: "keyword_extractor"`
     - `Instruction: instructionString`
     - `GenerateContentConfig: &genai.GenerateContentConfig{Temperature: 0.3}` (low temperature for consistent extraction)
   - Call `llmagent.New(config)` to create agent
   - Return error if agent creation fails

4. **Execute agent:**
   - Create agent session with empty initial state
   - Send empty user message (instruction contains the content)
   - Receive agent response
   - Extract text from response

5. **Parse agent response:**
   - Trim whitespace and remove markdown code fences if present (```json ... ```)
   - Parse JSON response into a map
   - Validate response structure:
     - Check `keywords` field exists and is a string array
     - Check `confidence` field exists and is a map (optional)
   - Return error if response is malformed: "agent returned invalid JSON response"

6. **Validate keywords:**
   - Check keyword count is between min and max
   - Filter out empty strings
   - Trim whitespace from each keyword
   - Convert to lowercase for consistency

7. **Return result:**
   - Return map with `keywords` and `confidence` fields
   - If confidence scores are missing, generate default scores (1.0 for all)

**Error handling:**
- Wrap all errors with context: "keyword extraction failed: {error}"
- Log errors with document content length and error details

Follow the pattern from `internal/services/places/service.go` for API interaction and error handling.

### internal\jobs\processor\agent_executor.go(NEW)

References: 

- internal\jobs\processor\crawler_executor.go
- internal\interfaces\agent_service.go(NEW)
- internal\interfaces\storage.go

Implement the `AgentExecutor` for queue-based agent job processing.

**Package declaration:** `package processor`

**Imports:**
- Standard: `context`, `encoding/json`, `fmt`, `time`
- Internal: `github.com/ternarybob/arbor`, `github.com/ternarybob/quaero/internal/interfaces`, `github.com/ternarybob/quaero/internal/models`, `github.com/ternarybob/quaero/internal/jobs`

**AgentExecutor struct:**

Define `AgentExecutor` struct with fields:
- `agentService interfaces.AgentService` - Agent service for executing agents
- `documentStorage interfaces.DocumentStorage` - Document storage for loading/saving documents
- `jobMgr *jobs.Manager` - Job manager for status updates
- `eventService interfaces.EventService` - Event service for publishing events
- `logger arbor.ILogger` - Structured logger

**Constructor: NewAgentExecutor(...) *AgentExecutor**

Create constructor accepting all dependencies via dependency injection. Follow the pattern from `NewCrawlerExecutor` in `crawler_executor.go` (lines 42-64).

**Method: GetJobType() string**

Return `"agent"` as the job type this executor handles.

**Method: Validate(job *models.JobModel) error**

Validate job configuration:
1. Check `agent_type` field exists in `job.Config` and is a non-empty string
2. Check `document_id` field exists in `job.Config` and is a non-empty string
3. Return descriptive error if validation fails

Follow validation pattern from `crawler_executor.go` lines 72-96.

**Method: Execute(ctx context.Context, job *models.JobModel) error**

1. **Extract configuration:**
   - Extract `agent_type` from `job.Config` using `job.GetConfigString("agent_type")`
   - Extract `document_id` from `job.Config` using `job.GetConfigString("document_id")`
   - Return error if extraction fails

2. **Load document:**
   - Call `documentStorage.GetDocument(document_id)`
   - Return error if document not found: "document not found: {document_id}"
   - Log document title and content length

3. **Prepare agent input:**
   - Create input map with:
     - `document_id`: document ID
     - `content`: document content markdown
     - `title`: document title
     - `metadata`: document metadata (for context)
   - Add any additional config fields from `job.Config` (e.g., `max_keywords`, `min_keywords`)

4. **Execute agent:**
   - Create timeout context (5 minutes default)
   - Call `agentService.Execute(ctx, agent_type, input)`
   - Log execution start with agent type and document ID
   - Handle errors: log and return with context

5. **Update document metadata:**
   - Merge agent output into `document.Metadata`
   - For keyword extractor: `document.Metadata["keywords"] = output["keywords"]`
   - For keyword extractor: `document.Metadata["keyword_confidence"] = output["confidence"]`
   - Add agent execution metadata:
     - `document.Metadata["agent_processed_at"] = time.Now().Format(time.RFC3339)`
     - `document.Metadata["agent_type"] = agent_type`
   - Set `document.UpdatedAt = time.Now()`

6. **Save updated document:**
   - Call `documentStorage.UpdateDocument(document)`
   - Return error if save fails
   - Log successful update with keyword count (if applicable)

7. **Publish events:**
   - Publish `EventDocumentUpdated` event with payload:
     - `job_id`: job ID
     - `document_id`: document ID
     - `agent_type`: agent type
     - `timestamp`: current time
   - Log event publication

8. **Update job status:**
   - Call `jobMgr.UpdateJobStatus(ctx, job.ID, "completed")`
   - Call `jobMgr.SetJobFinished(ctx, job.ID)`
   - Return nil (success)

**Error handling:**
- All errors should be logged with context (job ID, document ID, agent type)
- Errors should be set on the job via `jobMgr.SetJobError(ctx, job.ID, errorMsg)`
- Return errors to trigger job failure in processor

Follow the execution pattern from `crawler_executor.go` lines 98-478, especially error handling and event publishing.

### internal\jobs\executor\agent_step_executor.go(NEW)

References: 

- internal\jobs\executor\places_search_step_executor.go
- internal\jobs\executor\interfaces.go
- internal\interfaces\agent_service.go(NEW)

Implement the `AgentStepExecutor` for job definition-based agent execution.

**Package declaration:** `package executor`

**Imports:**
- Standard: `context`, `encoding/json`, `fmt`, `time`
- Internal: `github.com/ternarybob/arbor`, `github.com/ternarybob/quaero/internal/interfaces`, `github.com/ternarybob/quaero/internal/models`, `github.com/ternarybob/quaero/internal/jobs`

**AgentStepExecutor struct:**

Define `AgentStepExecutor` struct with fields:
- `agentService interfaces.AgentService` - Agent service for executing agents
- `documentStorage interfaces.DocumentStorage` - Document storage for loading/saving documents
- `jobMgr *jobs.Manager` - Job manager for creating agent jobs
- `queueMgr interfaces.QueueService` - Queue manager for enqueueing jobs
- `logger arbor.ILogger` - Structured logger

**Constructor: NewAgentStepExecutor(...) *AgentStepExecutor**

Create constructor accepting all dependencies. Follow pattern from `NewPlacesSearchStepExecutor` in `places_search_step_executor.go` (lines 23-36).

**Method: GetStepType() string**

Return `"agent"` as the step type this executor handles.

**Method: ExecuteStep(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (string, error)**

1. **Extract and validate step configuration:**
   - Extract `agent_type` from `step.Config` (required)
   - Extract `document_id` from `step.Config` (required)
   - Return error if either is missing
   - Extract optional config: `max_keywords`, `min_keywords`, etc.
   - Log step execution start with agent type and document ID

2. **Create agent job:**
   - Create job config map with:
     - `agent_type`: from step config
     - `document_id`: from step config
     - Any additional config fields from step config
   - Create job metadata map with:
     - `job_definition_id`: jobDef.ID
     - `parent_job_id`: parentJobID
     - `step_name`: step.Name
   - Create `JobModel` using `models.NewChildJobModel(parentJobID, "agent", step.Name, config, metadata, 1)`

3. **Enqueue agent job:**
   - Serialize job model to JSON using `job.ToJSON()`
   - Create queue message with job ID and payload
   - Call `queueMgr.Enqueue(ctx, message)`
   - Return error if enqueue fails
   - Log job enqueued with job ID

4. **Wait for job completion:**
   - Poll job status every 2 seconds using `jobMgr.GetJob(ctx, jobID)`
   - Check if job status is terminal (completed, failed, cancelled)
   - Return error if job failed with error message from job
   - Timeout after 10 minutes (configurable)
   - Log polling progress

5. **Load updated document:**
   - Call `documentStorage.GetDocument(document_id)` to get updated document with agent results
   - Extract agent output from document metadata (e.g., `keywords`, `keyword_confidence`)
   - Marshal output to JSON string for return value

6. **Return result:**
   - Return JSON string containing agent output
   - This allows chaining: next step can access previous step's output
   - Log successful completion with output summary

**Agent Chaining Support:**

The step executor supports chaining by:
1. Storing agent output in document metadata (persistent)
2. Returning agent output as JSON string (for immediate use by next step)
3. Next agent step can access previous results via document metadata

Example chain:
- Step 1: Keyword extraction → stores `keywords` in metadata
- Step 2: Summarization → reads `keywords` from metadata, generates summary

Follow the pattern from `places_search_step_executor.go` lines 38-190, especially job creation, polling, and result handling.

### internal\app\app.go(MODIFY)

References: 

- internal\services\agents\service.go(NEW)
- internal\jobs\processor\agent_executor.go(NEW)
- internal\jobs\executor\agent_step_executor.go(NEW)

**Add AgentService field to App struct** (line 96, after PlacesService):

Add field: `AgentService interfaces.AgentService`

**Initialize AgentService in initServices()** (line 337, after PlacesService initialization):

1. **Create agent service:**
   - Call `agents.NewService(&a.Config.Agent, a.Logger)`
   - Store in `a.AgentService`
   - Handle error: return with context "failed to initialize agent service"
   - Log successful initialization

2. **Register AgentExecutor with JobProcessor** (line 325, after database maintenance executor):
   - Create `agentExecutor` using `processor.NewAgentExecutor(a.AgentService, a.StorageManager.DocumentStorage(), jobMgr, a.EventService, a.Logger)`
   - Call `jobProcessor.RegisterExecutor(agentExecutor)`
   - Log registration: "Agent executor registered for job type: agent"

3. **Register AgentStepExecutor with JobExecutor** (line 362, after places search step executor):
   - Create `agentStepExecutor` using `executor.NewAgentStepExecutor(a.AgentService, a.StorageManager.DocumentStorage(), jobMgr, queueMgr, a.Logger)`
   - Call `a.JobExecutor.RegisterStepExecutor(agentStepExecutor)`
   - Log registration: "Agent step executor registered"

**Add cleanup in Close()** (line 678, after EventService close):

Add agent service cleanup:
- Check if `a.AgentService != nil`
- Call `a.AgentService.Close()`
- Log any errors: "Failed to close agent service"
- Log success: "Agent service closed"

Follow the initialization pattern from PlacesService (lines 331-337) and executor registration pattern from CrawlerExecutor (lines 291-303).

### deployments\local\job-definitions\keyword-extractor-agent.toml(NEW)

References: 

- deployments\local\job-definitions\news-crawler.toml
- deployments\local\job-definitions\nearby-restaurants-places.toml

Create an example job definition for keyword extraction agent.

**Job Definition Structure:**

```toml
# Keyword Extractor Agent Job Definition
# This job demonstrates how to use the keyword extraction agent to process documents
# and extract relevant keywords for improved search and categorization.

id = "keyword-extractor-agent"
name = "Keyword Extractor Agent"
type = "agent"
job_type = "user"
source_type = "agent"
description = "Agent job that extracts keywords from documents using Google Gemini"

# Tags to apply to processed documents (optional)
tags = ["agent-processed", "keywords"]

# Cron schedule (empty = manual execution only)
# Examples:
#   "0 */6 * * *"  = Every 6 hours
#   "0 0 * * *"    = Daily at midnight
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
# Agent type to execute
agent_type = "keyword_extractor"

# Document ID to process (required)
# In a real workflow, this would be dynamically set or iterated over multiple documents
document_id = "doc_example_123"

# Optional: Keyword extraction parameters
max_keywords = 15
min_keywords = 5
```

**Documentation Comments:**

Add comments explaining:
- This is an example showing single document processing
- In production, you'd typically chain this with a crawler job or iterate over documents
- The `document_id` must exist in the database before running this job
- Agent chaining example: Add a second step with `action = "agent"` and `agent_type = "summarizer"` (future)

**Agent Chaining Example (commented out):**

Add a commented-out second step showing how to chain agents:

```toml
# Example: Chain keyword extraction with summarization (future agent type)
# [[steps]]
# name = "generate_summary"
# action = "agent"
# on_error = "fail"
# 
# [steps.config]
# agent_type = "summarizer"
# document_id = "doc_example_123"
# use_keywords = true  # Use keywords from previous step
```

Follow the structure from `news-crawler.toml` for consistency.