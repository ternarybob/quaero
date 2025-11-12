I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Key Findings:**

1. **No existing LLM service** - Clean slate for Google ADK implementation
2. **ChatService exists but unused** - Not initialized in `app.go`, will be wired in subsequent phase
3. **Agent service provides pattern** - Lines 56-103 in `internal/services/agents/service.go` show Google ADK initialization with `gemini.NewModel()` and graceful degradation
4. **Interface well-defined** - `internal/interfaces/llm_service.go` specifies `Embed()`, `Chat()`, `HealthCheck()`, `GetMode()`, `Close()`
5. **Database expects 768-d embeddings** - `config.go` line 173 sets `EmbeddingDimension: 768`
6. **Configuration pattern established** - `AgentConfig` (lines 142-148) shows structure for Google API key and model settings
7. **Environment override pattern** - Lines 514-528 show `QUAERO_AGENT_*` environment variable handling

**Model Selection:**
- **Embedding**: `gemini-embedding-001` with `outputDimensionality: 768` (maintains compatibility with existing schema)
- **Chat**: `gemini-2.0-flash` (same as agent service, fast and cost-effective)
- **Legacy models**: `text-embedding-004` is deprecated per Google's 2025 guidance

### Approach

Create a new Google ADK-based LLM service implementing the `LLMService` interface. Follow the proven pattern from `internal/services/agents/service.go` for Google ADK integration. Add LLM configuration section to support API key and model names. The implementation will use `gemini-embedding-001` (768-d) for embeddings and `gemini-2.0-flash` for chat, ensuring compatibility with existing database schema and agent service.

### Reasoning

Explored the codebase structure, read the LLMService interface definition, examined the agent service implementation pattern, reviewed configuration system, checked app initialization flow, and searched web for current Google Gemini model recommendations. Confirmed no existing LLM implementations exist and ChatService is defined but not currently wired.

## Mermaid Diagram

sequenceDiagram
    participant Config as config.go
    participant GeminiSvc as gemini_service.go
    participant ADK as Google ADK
    participant Interface as LLMService Interface

    Note over Config: Add LLMConfig struct<br/>with API key, model names,<br/>timeout, embed dimension

    Note over Config: Add environment<br/>variable overrides<br/>QUAERO_LLM_*

    Note over GeminiSvc: Create GeminiService<br/>implementing LLMService

    Config->>GeminiSvc: NewGeminiService(config, logger)
    GeminiSvc->>GeminiSvc: Validate API key
    GeminiSvc->>GeminiSvc: Set default model names
    GeminiSvc->>ADK: gemini.NewModel(embedModelName)
    ADK-->>GeminiSvc: embedModel instance
    GeminiSvc->>ADK: gemini.NewModel(chatModelName)
    ADK-->>GeminiSvc: chatModel instance
    GeminiSvc-->>Config: Return *GeminiService

    Note over GeminiSvc,Interface: Implement Interface Methods

    Interface->>GeminiSvc: Embed(ctx, text)
    GeminiSvc->>ADK: GenerateContent(text, dim=768)
    ADK-->>GeminiSvc: []float32 embedding
    GeminiSvc-->>Interface: Return embedding

    Interface->>GeminiSvc: Chat(ctx, messages)
    GeminiSvc->>ADK: GenerateContent(messages)
    ADK-->>GeminiSvc: Response text
    GeminiSvc-->>Interface: Return response

    Interface->>GeminiSvc: HealthCheck(ctx)
    GeminiSvc->>GeminiSvc: Verify models initialized
    GeminiSvc-->>Interface: Return nil or error

    Interface->>GeminiSvc: GetMode()
    GeminiSvc-->>Interface: Return LLMModeCloud

    Interface->>GeminiSvc: Close()
    GeminiSvc->>GeminiSvc: Set models to nil
    GeminiSvc-->>Interface: Return nil

## Proposed File Changes

### internal\services\llm\gemini_service.go(NEW)

References: 

- internal\interfaces\llm_service.go
- internal\services\agents\service.go
- internal\common\config.go(MODIFY)

Create new Gemini LLM service implementing `interfaces.LLMService` interface.

**Package and Imports:**
- Package: `llm`
- Import Google ADK packages: `google.golang.org/adk/model`, `google.golang.org/adk/model/gemini`, `google.golang.org/genai`
- Import arbor logger, context, fmt, time
- Import `internal/common` for config, `internal/interfaces` for interface

**Service Struct:**
- Define `GeminiService` struct with fields:
  - `config *common.LLMConfig` - Configuration
  - `logger arbor.ILogger` - Structured logger
  - `embedModel model.LLM` - Gemini embedding model instance
  - `chatModel model.LLM` - Gemini chat model instance
  - `timeout time.Duration` - Operation timeout

**Constructor: NewGeminiService(config, logger)**
- Validate `config.GoogleAPIKey` is not empty (return error if missing)
- Set default model names if empty:
  - `EmbedModelName`: `"gemini-embedding-001"`
  - `ChatModelName`: `"gemini-2.0-flash"`
- Parse timeout duration from config (default: `"5m"`)
- Initialize embedding model using `gemini.NewModel(ctx, embedModelName, clientConfig)`
  - ClientConfig: `APIKey: config.GoogleAPIKey`, `Backend: genai.BackendGeminiAPI`
- Initialize chat model using `gemini.NewModel(ctx, chatModelName, clientConfig)`
- Log successful initialization with model names and timeout
- Return `*GeminiService` and nil error
- Follow exact pattern from `internal/services/agents/service.go` lines 56-103

**Embed(ctx, text) Method:**
- Create timeout context using service timeout
- Call embedding model's `GenerateContent()` or appropriate ADK embedding method
- **Important**: Specify `outputDimensionality: 768` to match database schema (see `config.go` line 173)
- Parse response and extract embedding vector as `[]float32`
- Validate vector length is exactly 768 dimensions
- Log embedding generation with text length and duration
- Return embedding vector or error
- Handle context cancellation and timeout errors

**Chat(ctx, messages) Method:**
- Convert `[]interfaces.Message` to ADK message format
- Create timeout context using service timeout
- Call chat model's `GenerateContent()` with converted messages
- Extract text response from ADK response object
- Log chat completion with message count and duration
- Return response string or error
- Handle context cancellation and timeout errors

**HealthCheck(ctx) Method:**
- Verify both `embedModel` and `chatModel` are not nil
- Verify model names are set correctly using `model.Name()` method
- Optionally: Make lightweight test call to verify API connectivity
- Log health check result
- Return nil if healthy, error with details if unhealthy
- Follow pattern from `internal/services/agents/service.go` lines 191-209

**GetMode() Method:**
- Return `interfaces.LLMModeCloud` constant
- Simple one-liner method

**Close() Method:**
- Log service closure
- Set `embedModel` and `chatModel` to nil (ADK models don't require explicit cleanup)
- Return nil
- Follow pattern from `internal/services/agents/service.go` lines 217-225

**Error Handling:**
- Wrap all errors with context using `fmt.Errorf()`
- Use structured logging for all operations (Info, Debug, Error levels)
- Handle timeout errors gracefully with clear messages

### internal\common\config.go(MODIFY)

References: 

- internal\services\agents\service.go
- internal\interfaces\llm_service.go

Add LLM configuration section to support Google ADK LLM service.

**Add LLMConfig Struct (after AgentConfig, around line 148):**
- Define `LLMConfig` struct with fields:
  - `GoogleAPIKey string` with toml tag `"google_api_key"` - Google Gemini API key for LLM operations
  - `EmbedModelName string` with toml tag `"embed_model_name"` - Gemini embedding model identifier (default: `"gemini-embedding-001"`)
  - `ChatModelName string` with toml tag `"chat_model_name"` - Gemini chat model identifier (default: `"gemini-2.0-flash"`)
  - `Timeout string` with toml tag `"timeout"` - LLM operation timeout as duration string (default: `"5m"`)
  - `EmbedDimension int` with toml tag `"embed_dimension"` - Embedding vector dimension (default: 768, must match SQLite config)
- Add documentation comments explaining each field
- Follow exact pattern from `AgentConfig` struct (lines 142-148)

**Add LLM Field to Config Struct (line 27, after Agent field):**
- Add `LLM LLMConfig` with toml tag `"llm"`
- Maintains alphabetical ordering of config sections

**Update NewDefaultConfig() Function (around line 254, after Agent section):**
- Add LLM configuration with defaults:
  - `GoogleAPIKey: ""` - User must provide API key (no fallback)
  - `EmbedModelName: "gemini-embedding-001"` - Current recommended embedding model
  - `ChatModelName: "gemini-2.0-flash"` - Fast, cost-effective chat model (same as agent service)
  - `Timeout: "5m"` - 5 minutes for LLM operations
  - `EmbedDimension: 768` - Matches SQLite embedding dimension (line 173)
- Add comment explaining model choices and compatibility requirements

**Add Environment Variable Overrides (after Agent section, around line 528):**
- Add environment variable handling in `applyEnvOverrides()` function:
  - `QUAERO_LLM_GOOGLE_API_KEY` → `config.LLM.GoogleAPIKey`
  - `QUAERO_LLM_EMBED_MODEL_NAME` → `config.LLM.EmbedModelName`
  - `QUAERO_LLM_CHAT_MODEL_NAME` → `config.LLM.ChatModelName`
  - `QUAERO_LLM_TIMEOUT` → `config.LLM.Timeout`
  - `QUAERO_LLM_EMBED_DIMENSION` → `config.LLM.EmbedDimension` (parse as int)
- Follow exact pattern from agent environment overrides (lines 514-528)
- Use `strconv.Atoi()` for integer parsing with error checking

**Validation Notes:**
- Ensure `EmbedDimension` matches `Storage.SQLite.EmbeddingDimension` (both should be 768)
- API key validation happens in service constructor, not config loading
- Timeout parsing happens in service constructor using `time.ParseDuration()`