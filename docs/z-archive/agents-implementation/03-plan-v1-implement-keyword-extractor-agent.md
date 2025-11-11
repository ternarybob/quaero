I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**What Exists:**
1. ✅ `service.go` - Agent service with Gemini client initialization
2. ✅ `keyword_extractor.go` - Keyword extractor using **raw Gemini API** (not ADK)
3. ✅ `agent_executor.go` - Queue-based job executor (fully implemented)
4. ✅ `agent_step_executor.go` - Job definition step executor (fully implemented)
5. ✅ `AgentConfig` in `config.go` with Google API key, model name, max turns, timeout
6. ✅ `AgentService` interface in `interfaces/agent_service.go`
7. ❌ **Missing:** `google.golang.org/adk` dependency in `go.mod`

**Critical Issues:**
1. **Wrong Architecture:** Current implementation uses raw `genai.Client.Models.GenerateContent()` API calls instead of Google ADK's `llmagent` agent loop
2. **Wrong Interface:** `AgentExecutor` interface expects `*genai.Client` but should expect `model.LLM` from ADK
3. **Missing Dependency:** ADK package not in `go.mod`

**What Works:**
- Configuration loading and validation
- Job executor integration (both queue-based and step-based)
- Document metadata storage pattern
- Event publishing and logging
- Error handling and status updates

**What Needs Fixing:**
- Add ADK dependency to `go.mod`
- Refactor `service.go` to create `model.LLM` using `gemini.NewModel()` instead of raw client
- Update `AgentExecutor` interface to accept `model.LLM`
- Refactor `keyword_extractor.go` to use `llmagent.New()` with agent loop pattern
- Ensure proper JSON response parsing from ADK agent

### Approach

## Refactoring Strategy

**Phase 1: Add ADK Dependency**
Add `google.golang.org/adk` to `go.mod` and run `go mod tidy`.

**Phase 2: Fix Service Architecture**
Refactor `service.go` to use ADK's `gemini.NewModel()` instead of raw `genai.Client`. Update the `AgentExecutor` interface to accept `model.LLM`.

**Phase 3: Refactor Keyword Extractor**
Replace raw API calls with ADK's `llmagent.New()` and agent loop pattern. Use structured prompts and proper JSON parsing.

**Why This Approach:**
- Minimal changes to existing job executor integration
- Follows Google ADK best practices from official documentation
- Maintains backward compatibility with existing job definitions
- Preserves all error handling and logging patterns

### Reasoning

I explored the codebase by reading the agent service files (`service.go`, `keyword_extractor.go`), examining the configuration structure (`config.go`, `app.go`), understanding the job execution architecture (`agent_executor.go`, `agent_step_executor.go`), reviewing document models and storage patterns, and researching Google ADK integration via web search. I discovered that the current implementation bypasses ADK entirely and uses raw Gemini API calls, which needs to be corrected to use the proper `llmagent` agent loop pattern.

## Mermaid Diagram

sequenceDiagram
    participant Job as Job Executor
    participant AgentSvc as Agent Service
    participant ADK as Google ADK<br/>(llmagent)
    participant Gemini as Gemini API
    participant DocStore as Document Storage

    Note over Job,DocStore: Current Implementation (Raw API)
    Job->>AgentSvc: Execute("keyword_extractor", input)
    AgentSvc->>KeywordExt: Execute(genai.Client, input)
    KeywordExt->>Gemini: client.Models.GenerateContent()
    Gemini-->>KeywordExt: Raw JSON response
    KeywordExt->>KeywordExt: Parse JSON manually
    KeywordExt-->>AgentSvc: keywords + confidence
    AgentSvc-->>Job: Agent output
    Job->>DocStore: Update metadata

    Note over Job,DocStore: Refactored Implementation (ADK)
    Job->>AgentSvc: Execute("keyword_extractor", input)
    AgentSvc->>KeywordExt: Execute(model.LLM, input)
    KeywordExt->>ADK: llmagent.New(config)
    ADK-->>KeywordExt: Agent instance
    KeywordExt->>ADK: agent.Run(ctx, state)
    ADK->>Gemini: Managed API calls
    Gemini-->>ADK: Streaming events
    ADK-->>KeywordExt: Event stream
    KeywordExt->>KeywordExt: Collect & parse response
    KeywordExt-->>AgentSvc: keywords + confidence
    AgentSvc-->>Job: Agent output
    Job->>DocStore: Update metadata

## Proposed File Changes

### go.mod(MODIFY)

Add Google ADK dependency to the `require` section:

**Add line after line 20 (after `google.golang.org/genai v1.34.0`):**
- `google.golang.org/adk v0.1.0` - Agent Development Kit for building AI agents with Gemini

The ADK provides the `llmagent` package for creating agents with conversation loops, tool use, and streaming responses. It wraps the genai client and provides higher-level abstractions.

**After adding, run:** `go mod tidy` to download the dependency and update `go.sum`.

**Note:** Verify the latest stable version at implementation time. Use `go get google.golang.org/adk@latest` if needed.

### internal\services\agents\service.go(MODIFY)

References: 

- internal\services\agents\keyword_extractor.go(MODIFY)

**Critical Architecture Fix:** Replace raw `genai.Client` with ADK's `model.LLM` to enable proper agent loop functionality.

**Import Changes (lines 3-11):**
- Add: `"google.golang.org/adk/model/gemini"` - ADK's Gemini model adapter
- Keep: `"google.golang.org/genai"` - Still needed for ClientConfig
- Keep: All existing imports

**Update AgentExecutor Interface (lines 13-21):**
- Change line 18: `Execute(ctx context.Context, client *genai.Client, input map[string]interface{})` 
- To: `Execute(ctx context.Context, model model.LLM, input map[string]interface{})`
- Update doc comment: "Execute runs the agent with the given Gemini model and input"

**Update Service Struct (lines 23-31):**
- Change line 28: `client *genai.Client`
- To: `model model.LLM` - ADK model instance that wraps genai client
- Update doc comment: "model manages the Gemini model via ADK"

**Refactor NewService Constructor (lines 54-101):**

1. **Keep validation** (lines 56-68): API key, model name, timeout parsing - no changes

2. **Replace client initialization** (lines 70-78):
   - Remove: Direct `genai.NewClient()` call
   - Add: Create `genai.ClientConfig` with API key and backend
   - Add: Call `gemini.NewModel(ctx, config.ModelName, clientConfig)` to create ADK model
   - Error handling: "failed to initialize Gemini model via ADK: %w"

3. **Update service initialization** (lines 80-87):
   - Change: `client: client` to `model: model`
   - Keep: All other fields unchanged

4. **Update logging** (lines 93-98):
   - Change: "Agent service initialized with Gemini API" to "Agent service initialized with Google ADK"
   - Keep: All log fields (model, max_turns, timeout, registered_agents)

**Update Execute Method (line 155):**
- Change: `output, err := agent.Execute(timeoutCtx, s.client, input)`
- To: `output, err := agent.Execute(timeoutCtx, s.model, input)`

**Update HealthCheck Method (lines 190-201):**
- Change line 195: `if s.client == nil` to `if s.model == nil`
- Update error message: "agent service model is not initialized"
- Keep: All other logic unchanged

**Update Close Method (lines 209-217):**
- Change line 213: `s.client = nil` to `s.model = nil`
- Keep: All other cleanup logic

**Why These Changes:**
- ADK's `gemini.NewModel()` creates a `model.LLM` that wraps the genai client internally
- This enables proper agent loop functionality with `llmagent.New()`
- The model instance handles client lifecycle and configuration
- Follows official ADK patterns from Google documentation

### internal\services\agents\keyword_extractor.go(MODIFY)

References: 

- internal\services\agents\service.go(MODIFY)

**Complete Refactor:** Replace raw Gemini API calls with Google ADK's `llmagent` agent loop pattern.

**Import Changes (lines 3-9):**
- Add: `"strings"` - For response text cleaning
- Add: `"google.golang.org/adk/agent/llmagent"` - ADK agent loop
- Add: `"google.golang.org/adk/model"` - ADK model interface
- Remove: `"google.golang.org/genai"` - No longer using raw API

**Update Execute Method Signature (line 53):**
- Change: `Execute(ctx context.Context, client *genai.Client, input map[string]interface{})`
- To: `Execute(ctx context.Context, llmModel model.LLM, input map[string]interface{})`
- Update doc comment parameter: "llmModel: ADK model to use for extraction"

**Keep Validation Logic (lines 54-69):**
- No changes to document_id, content, max_keywords validation
- This logic is correct and follows best practices

**Refactor Prompt Building (lines 71-92):**

1. **Update prompt structure** to be more agent-friendly:
   - Keep: Core instructions about keyword extraction
   - Enhance: Add explicit JSON schema in prompt
   - Add: Clear instruction to return ONLY JSON (no markdown fences)
   - Format: Use clear sections (Role, Task, Rules, Output Format, Input)

2. **Example prompt structure:**
   ```
   You are a keyword extraction specialist.
   
   Task: Extract the {maxKeywords} most semantically relevant keywords from the document.
   
   Rules:
   - Single words or short phrases (2-3 words max)
   - Domain-specific terminology and technical concepts
   - No stop words (the, is, and, etc.)
   - Assign confidence scores (0.0-1.0)
   
   Output Format (JSON only, no markdown):
   {"keywords": ["keyword1", "keyword2"], "confidence": {"keyword1": 0.95, "keyword2": 0.87}}
   
   Document:
   {content}
   ```

**Replace API Call with ADK Agent Loop (lines 94-111):**

1. **Remove raw API call** (lines 94-104):
   - Delete: `client.Models.GenerateContent()` call
   - Delete: Manual part construction

2. **Add ADK agent creation:**
   - Create `llmagent.Config` with:
     - `Model: llmModel` - Use passed model
     - `Name: "keyword_extractor"` - Agent identifier
     - `Instruction: prompt` - Full prompt as instruction
     - `GenerateContentConfig: &genai.GenerateContentConfig{Temperature: 0.3}` - Low temp for consistency
   - Call `llmagent.New(config)` to create agent
   - Handle error: "failed to create keyword extraction agent: %w"

3. **Execute agent loop:**
   - Create empty initial state (no conversation history)
   - Call `agent.Run(ctx, state)` to execute
   - Iterate over event stream: `for event := range agent.Run(ctx, state)`
   - Collect response text from events
   - Handle errors in event stream

**Update Response Parsing (lines 113-136):**

1. **Add response cleaning** (before JSON parsing):
   - Trim whitespace: `response = strings.TrimSpace(response)`
   - Remove markdown code fences if present:
     - Check for ````json` prefix and ````` suffix
     - Strip them if found: `response = strings.TrimPrefix(response, "```json")`
     - `response = strings.TrimSuffix(response, "```")`
     - Trim again after stripping

2. **Keep JSON parsing logic** (lines 114-121):
   - No changes to struct definition
   - No changes to `json.Unmarshal()` call
   - Keep error message format

3. **Keep validation logic** (lines 123-130):
   - No changes to keyword count validation
   - No changes to confidence validation
   - These checks are correct

4. **Keep return statement** (lines 132-136):
   - No changes to output map structure
   - Maintains compatibility with existing code

**Keep GetType Method (lines 139-142):**
- No changes needed
- Returns "keyword_extractor" correctly

**Why These Changes:**
- ADK's `llmagent` provides proper agent loop with conversation management
- Handles streaming responses and event processing automatically
- More robust than raw API calls for complex agent interactions
- Follows official Google ADK patterns and best practices
- Maintains exact same input/output contract for backward compatibility