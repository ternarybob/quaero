I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current State:**
- `GetServiceStatus()` contains offline mode checks for ports 8086 and 8087 (embed/chat servers)
- `checkServerHealth()` function performs TCP connection checks to verify server availability
- `GetMode()`, `Chat()`, and `HealthCheck()` already delegate to `llmService` interface correctly
- `agent_loop.go` uses `llmService` through the interface with no offline-specific logic
- No test files exist in `internal/services/chat/` directory
- The new Google ADK LLM service returns `LLMModeCloud` from `GetMode()`

**Design Decision:**
Since the new LLM service is cloud-based (Google ADK), the status response should reflect cloud service characteristics rather than local server ports. The simplified status will include mode, health check result, and timestamp.

### Approach

Remove offline mode infrastructure from ChatService by eliminating port-based health checks (8086, 8087) and simplifying `GetServiceStatus()` to return cloud-mode-appropriate status. The `GetMode()`, `Chat()`, and `HealthCheck()` methods already delegate correctly to the `LLMService` interface and require no changes. The `agent_loop.go` file works through the interface and needs no modifications.

### Reasoning

Read the three relevant files (`chat_service.go`, `agent_loop.go`, `chat_service.go` interface), searched for test files (none found), examined the `LLMService` interface to understand mode constants (`LLMModeCloud`, `LLMModeOffline`, `LLMModeMock`), and identified that offline mode checks exist only in `GetServiceStatus()` method (lines 92-110) and the `checkServerHealth()` helper function (lines 116-134).

## Mermaid Diagram

sequenceDiagram
    participant Client as API Client
    participant ChatSvc as ChatService
    participant LLMSvc as LLMService<br/>(Google ADK)
    participant AgentLoop as AgentLoop

    Note over ChatSvc: Remove offline mode checks

    Client->>ChatSvc: GetServiceStatus(ctx)
    ChatSvc->>LLMSvc: GetMode()
    LLMSvc-->>ChatSvc: "cloud"
    ChatSvc->>LLMSvc: HealthCheck(ctx)
    LLMSvc-->>ChatSvc: nil (healthy)
    ChatSvc-->>Client: {mode: "cloud", healthy: true, service_type: "google_adk"}

    Note over ChatSvc,AgentLoop: No changes - already interface-based

    Client->>ChatSvc: Chat(ctx, request)
    ChatSvc->>AgentLoop: Execute(ctx, message, streamFunc)
    AgentLoop->>LLMSvc: Chat(ctx, messages)
    LLMSvc-->>AgentLoop: response text
    AgentLoop-->>ChatSvc: final answer
    ChatSvc-->>Client: ChatResponse

    Client->>ChatSvc: HealthCheck(ctx)
    ChatSvc->>LLMSvc: HealthCheck(ctx)
    LLMSvc-->>ChatSvc: nil or error
    ChatSvc-->>Client: nil or error

## Proposed File Changes

### internal\services\chat\chat_service.go(MODIFY)

References: 

- internal\interfaces\llm_service.go
- internal\services\llm\gemini_service.go

**Remove offline mode infrastructure and simplify GetServiceStatus() for cloud-based LLM service.**

**Remove checkServerHealth() Function (lines 116-134):**
- Delete the entire `checkServerHealth()` function
- This function performs TCP connection checks to ports 8086 and 8087
- No longer needed since Google ADK LLM service doesn't use local ports
- Function is only called from `GetServiceStatus()` which we're refactoring

**Refactor GetServiceStatus() Method (lines 78-113):**
- Keep method signature: `func (s *ChatService) GetServiceStatus(ctx context.Context) map[string]interface{}`
- Remove offline mode conditional logic (lines 92-110)
- Simplify to return cloud-mode-appropriate status information:
  - `mode`: Get from `s.llmService.GetMode()` (will return "cloud")
  - `healthy`: Call `s.llmService.HealthCheck(ctx)` and return boolean (true if nil error, false otherwise)
  - `service_type`: Return "google_adk" to indicate the LLM provider
  - `last_check_time`: Return current timestamp in RFC3339 format
- Remove all references to `embed_server`, `chat_server`, `model_loaded` fields
- Remove port checking logic and TCP connection attempts
- Log health check result at Debug level with status details

**Remove Unused Imports:**
- Remove `net` import (line 6) - no longer needed without TCP connection checks
- Keep all other imports: `context`, `fmt`, `strings`, `time`, `arbor`, `interfaces`, `mcp`

**No Changes Needed:**
- `NewChatService()` constructor (lines 24-40) - already correct
- `Chat()` method (lines 43-60) - delegates to agent loop correctly
- `GetMode()` method (lines 63-65) - delegates to llmService correctly
- `HealthCheck()` method (lines 68-75) - delegates to llmService correctly

**Expected Status Response Structure:**
```
{
  "mode": "cloud",
  "healthy": true,
  "service_type": "google_adk",
  "last_check_time": "2025-01-15T10:30:00Z"
}
```

**Error Handling:**
- If `HealthCheck()` returns error, set `healthy: false` and optionally include `error` field with error message
- Log health check failures at Error level with error details

### internal\services\chat\agent_loop.go(MODIFY)

References: 

- internal\interfaces\llm_service.go
- internal\services\chat\chat_service.go(MODIFY)

**No changes required - verification only.**

**Verify Existing Implementation:**
- Confirm `AgentLoop` struct uses `llmService interfaces.LLMService` field (line 38)
- Confirm `NewAgentLoop()` constructor accepts `llmService interfaces.LLMService` parameter (line 46)
- Confirm `callLLM()` method (lines 267-284) calls `a.llmService.Chat(ctx, messages)`
- Confirm no offline-specific logic exists (no port checks, no mode conditionals)

**Why No Changes:**
- Agent loop already works through the `LLMService` interface abstraction
- The `Chat()` method signature matches what Google ADK LLM service implements
- Message conversion (lines 269-275) is interface-agnostic
- Tool use parsing (lines 286-326) is LLM-agnostic
- System prompt building (lines 257-264) doesn't depend on LLM mode

**Integration Verification:**
- When `ChatService` is initialized with the new Google ADK LLM service in `internal/app/app.go` (subsequent phase), the agent loop will automatically use it
- No code changes needed in this file for the migration to work correctly