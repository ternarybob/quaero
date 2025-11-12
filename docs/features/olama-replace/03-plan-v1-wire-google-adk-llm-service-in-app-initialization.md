I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current State Analysis:**

1. **GeminiService Implementation Complete** - `internal/services/llm/gemini_service.go` implements `LLMService` interface with Google ADK
   - Constructor: `NewGeminiService(config *common.Config, logger arbor.ILogger) (*GeminiService, error)`
   - Returns error if API key missing (graceful degradation pattern)
   - Implements all required methods: `Embed()`, `Chat()`, `HealthCheck()`, `GetMode()`, `Close()`

2. **ChatService Implementation Complete** - `internal/services/chat/chat_service.go` ready for wiring
   - Constructor: `NewChatService(llmService, documentStorage, searchService, logger)`
   - Already uses `LLMService` interface (no offline-specific code)
   - Methods: `Chat()`, `GetMode()`, `HealthCheck()`, `GetServiceStatus()`

3. **Configuration Ready** - `internal/common/config.go` has complete LLM configuration
   - `LLMConfig` struct with Google API key, model names, timeout, embed dimension (lines 151-158)
   - Environment variable overrides implemented (lines 547-564)
   - Default values set (lines 265-271)

4. **App Structure Missing Services** - `internal/app/app.go` needs updates
   - App struct (lines 44-102) lacks `LLMService` and `ChatService` fields
   - `initServices()` (lines 213-459) needs LLM and Chat service initialization
   - `Close()` (lines 660-735) needs cleanup for both services
   - Agent service pattern (lines 354-370) provides exact template

5. **Initialization Order Critical** - Services must follow dependency chain
   - LLM service depends on: Config, Logger (no other services)
   - Chat service depends on: LLM service, DocumentStorage, SearchService, Logger
   - Current order: DocumentService (line 234) → SearchService (line 240) → EventService (line 119)
   - **Optimal placement**: LLM service after SearchService, Chat service after LLM service

6. **Handler Integration Not Required** - No chat handler exists in current codebase
   - `initHandlers()` (lines 462-633) has no chat-related handlers
   - Chat service will be available for future handler implementation
   - This phase focuses only on service initialization and wiring

### Approach

Wire the Google ADK-based LLM service and ChatService into the application initialization flow following the established agent service pattern. Add both services to the App struct, initialize them in dependency order within `initServices()`, perform health checks during startup, and ensure proper cleanup in `Close()`. The implementation follows graceful degradation - if the Google API key is missing, LLM service initialization fails with a warning but the application continues without chat features.

### Reasoning

Read `internal/app/app.go` to understand current service initialization patterns, examined the agent service initialization (lines 354-370) as the template to follow, reviewed `internal/services/llm/gemini_service.go` to understand the constructor signature and error handling, checked `internal/services/chat/chat_service.go` to understand dependencies, and verified `internal/common/config.go` has complete LLM configuration with environment variable support.

## Mermaid Diagram

sequenceDiagram
    participant Main as cmd/quaero/main.go
    participant App as app.New()
    participant Config as Config
    participant LLMSvc as LLM Service
    participant ChatSvc as Chat Service
    participant Logger as Logger

    Main->>Config: LoadFromFile(configPath)
    Config-->>Main: *Config
    Main->>Logger: InitLogger(config)
    Logger-->>Main: arbor.ILogger
    Main->>App: New(config, logger)
    
    Note over App: initDatabase()
    Note over App: Initialize WebSocket & LogService
    
    App->>App: initServices()
    
    Note over App: Initialize DocumentService
    Note over App: Initialize SearchService
    
    App->>LLMSvc: llm.NewGeminiService(config, logger)
    alt API Key Present
        LLMSvc->>LLMSvc: Validate config
        LLMSvc->>LLMSvc: Initialize embed & chat models
        LLMSvc-->>App: *GeminiService
        App->>LLMSvc: HealthCheck(ctx)
        alt Health Check Pass
            LLMSvc-->>App: nil
            App->>Logger: Info("LLM service initialized")
        else Health Check Fail
            LLMSvc-->>App: error
            App->>Logger: Warn("Health check failed")
        end
    else API Key Missing
        LLMSvc-->>App: error
        App->>App: Set LLMService = nil
        App->>Logger: Warn("LLM service unavailable")
    end
    
    alt LLMService != nil
        App->>ChatSvc: chat.NewChatService(llmService, docStorage, searchService, logger)
        ChatSvc->>ChatSvc: Initialize agent loop & tool router
        ChatSvc-->>App: *ChatService
        App->>ChatSvc: HealthCheck(ctx)
        alt Health Check Pass
            ChatSvc-->>App: nil
            App->>Logger: Info("Chat service initialized")
        else Health Check Fail
            ChatSvc-->>App: error
            App->>Logger: Warn("Health check failed")
        end
    else LLMService == nil
        App->>App: Set ChatService = nil
        App->>Logger: Info("Chat service not initialized")
    end
    
    Note over App: Continue with other services...
    Note over App: Initialize AgentService
    Note over App: Initialize handlers
    
    App-->>Main: *App
    
    Note over Main: Application runs...
    
    Main->>App: Close()
    App->>LLMSvc: Close()
    LLMSvc->>LLMSvc: Clear model references
    LLMSvc-->>App: nil
    App->>App: Set ChatService = nil
    App->>Logger: Info("Services closed")

## Proposed File Changes

### internal\app\app.go(MODIFY)

References: 

- internal\services\llm\gemini_service.go
- internal\services\chat\chat_service.go
- internal\interfaces\llm_service.go
- internal\interfaces\chat_service.go
- internal\common\config.go

**Add LLM and Chat service fields to App struct (after AgentService field, around line 87):**

- Add `LLMService interfaces.LLMService` field with comment `// LLM service (Google ADK)`
- Add `ChatService interfaces.ChatService` field with comment `// Chat service (agent-based)`
- Maintains alphabetical ordering and consistency with other service fields

**Add import for LLM service package (in import block, around line 25):**

- Add `"github.com/ternarybob/quaero/internal/services/llm"` import
- Add `"github.com/ternarybob/quaero/internal/services/chat"` import
- Place after agents import, before auth import (alphabetical order)

**Initialize LLM service in initServices() method (after SearchService initialization, around line 248):**

- Call `llm.NewGeminiService(a.Config, a.Logger)` to create LLM service
- Check for error - if error occurs:
  - Set `a.LLMService = nil` explicitly
  - Log warning with `a.Logger.Warn().Err(err).Msg("Failed to initialize LLM service - chat features will be unavailable")`
  - Log info message: `"To enable LLM features, set QUAERO_LLM_GOOGLE_API_KEY or llm.google_api_key in config"`
- If successful:
  - Assign to `a.LLMService`
  - Perform health check with `a.LLMService.HealthCheck(context.Background())`
  - If health check fails, log warning: `"LLM service health check failed - API key may be invalid"`
  - If health check passes, log info: `"LLM service initialized and health check passed"`
- **Follow exact pattern from agent service initialization (lines 354-370)**
- **Critical**: This must happen AFTER SearchService initialization (line 240) and BEFORE ChatService initialization

**Initialize Chat service in initServices() method (after LLM service initialization):**

- Check if `a.LLMService != nil` before initializing chat service
- If LLM service is available:
  - Call `chat.NewChatService(a.LLMService, a.StorageManager.DocumentStorage(), a.SearchService, a.Logger)`
  - Assign to `a.ChatService`
  - Perform health check with `a.ChatService.HealthCheck(context.Background())`
  - If health check fails, log warning with error details
  - If health check passes, log info: `"Chat service initialized and health check passed"`
- If LLM service is nil:
  - Set `a.ChatService = nil`
  - Log info: `"Chat service not initialized (LLM service unavailable)"`
- **Dependencies**: Requires `a.LLMService`, `a.StorageManager.DocumentStorage()`, `a.SearchService`, `a.Logger`
- **Placement**: After LLM service initialization, before AgentService initialization (line 354)

**Add LLM service cleanup in Close() method (after AgentService cleanup, around line 718):**

- Add conditional check: `if a.LLMService != nil`
- Call `a.LLMService.Close()` and check for error
- If error occurs, log warning: `a.Logger.Warn().Err(err).Msg("Failed to close LLM service")`
- If successful, log info: `a.Logger.Info().Msg("LLM service closed")`
- **Placement**: After agent service cleanup (line 713-718), before event service cleanup (line 720-725)

**Add Chat service cleanup in Close() method (after LLM service cleanup):**

- Add comment: `// Close chat service (no explicit Close method, just nil reference)`
- Set `a.ChatService = nil` (ChatService has no Close method, just clear reference)
- **Note**: ChatService doesn't implement a Close method, so only nil the reference
- **Placement**: Immediately after LLM service cleanup, before event service cleanup

**Error Handling Pattern:**
- Follow agent service pattern exactly (lines 354-370)
- Graceful degradation: Missing API key logs warning but doesn't fail app startup
- Health check failures log warnings but don't prevent service initialization
- All errors wrapped with context using `fmt.Errorf()`

**Logging Standards:**
- Use structured logging with arbor logger
- Info level for successful initialization and health checks
- Warn level for initialization failures and health check failures
- Include relevant context (model names, error details) in log messages