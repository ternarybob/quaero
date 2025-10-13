# Quaero MCP Agent Refactor - Implementation Plan

**Generated:** 2025-10-13
**Status:** READY FOR IMPLEMENTATION
**Goal:** Transform Quaero from passive RAG to active AI Agent with streaming tool execution

---

## Executive Summary

### Current State (RAG Model)
- **Architecture:** Passive retrieval → context stuffing → single LLM call
- **Limitations:**
  - Quality depends entirely on initial search
  - Cannot refine queries or ask follow-up questions
  - No transparency into reasoning process
  - Poor handling of multi-hop questions
  - Context window overflow issues

### Target State (Agent Model)
- **Architecture:** Iterative planning → tool execution → reasoning loop
- **Capabilities:**
  - Dynamic information gathering on-demand
  - Multi-step reasoning with tool use
  - Real-time streaming of thought process
  - Transparent, verifiable decision-making
  - Efficient context usage

### What We Keep
✅ Server infrastructure (`internal/server/`)
✅ Configuration system (`internal/common/config.go`)
✅ LLM communication (`internal/services/llm/`)
✅ WebSocket infrastructure (`internal/handlers/websocket.go`)
✅ SQLite storage layer (`internal/storage/sqlite/`)
✅ Document models (`internal/models/document.go`)
✅ Existing MCP types (`internal/services/mcp/types.go`)

### What We Replace
❌ Atlassian-specific scrapers → Generic web scraper
❌ RAG-based chat service → Agent orchestration loop
❌ Pointer RAG complexity → Simple tool-based search
❌ Static frontend → Streaming agent UI

---

## Stage 1: Data Layer Simplification

**Goal:** Replace specialized scrapers with generic solution, enhance storage for agent tools

### Current Issues
- Tightly coupled to Atlassian APIs
- Separate services for Jira/Confluence
- Complex event-driven processing
- Maintenance burden for API changes

### Task 1.1: Implement Generic Web Scraper

**Priority:** HIGH
**Estimated Complexity:** MEDIUM
**Dependencies:** None

#### Implementation Steps

1. **Create scraper service structure**
   ```
   internal/services/scraper/
   ├── service.go          # Main scraper service
   ├── parser.go           # HTML to Markdown conversion
   ├── extractor.go        # Content extraction logic
   └── service_test.go     # Unit tests
   ```

2. **Core interfaces**
   ```go
   type ScraperService interface {
       Scrape(ctx context.Context, url string) (*models.Document, error)
       ScrapeMultiple(ctx context.Context, urls []string) ([]*models.Document, error)
   }
   ```

3. **Features to implement**
   - Standard HTTP client with retries
   - HTML parsing using `golang.org/x/net/html`
   - Content extraction (heuristics for main content)
   - Clean Markdown conversion
   - Metadata extraction (title, description, etc.)
   - Rate limiting support

4. **Testing approach**
   - Mock HTTP responses
   - Test with various HTML structures
   - Validate Markdown output quality

**Files to Create:**
- `internal/services/scraper/service.go`
- `internal/services/scraper/parser.go`
- `internal/services/scraper/extractor.go`
- `internal/services/scraper/service_test.go`

**Files to Modify:**
- `internal/app/app.go` (add scraper initialization)

### Task 1.2: Enhance SQLite Storage for Agent Tools

**Priority:** HIGH
**Estimated Complexity:** LOW
**Dependencies:** None

#### New Storage Methods Required

1. **Full-text search (already exists, verify)**
   ```go
   FullTextSearch(query string, limit int) ([]*models.Document, error)
   ```

2. **Get document by ID (already exists, verify)**
   ```go
   GetByID(ctx context.Context, id string) (*models.Document, error)
   ```

3. **Search by reference (NEW)**
   ```go
   SearchByReference(ctx context.Context, refKey string, limit int) ([]*models.Document, error)
   ```

4. **Source-filtered search (NEW)**
   ```go
   SearchByTextFiltered(ctx context.Context, query string, sources []string, limit int) ([]*models.Document, error)
   ```

**Files to Modify:**
- `internal/storage/sqlite/document_storage.go` (add new methods)
- `internal/interfaces/document_storage.go` (update interface)

**Testing:**
- Add integration tests for new search methods
- Verify FTS5 index performance

### Task 1.3: Deprecate Atlassian Services

**Priority:** LOW (do after agent is working)
**Estimated Complexity:** LOW
**Dependencies:** Generic scraper must be proven

#### Phase-out Strategy

1. **Keep initially as fallback**
   - Don't delete until generic scraper is proven
   - Mark as deprecated in comments
   - Remove from scheduler first

2. **Gradual migration**
   - Test generic scraper with Jira/Confluence URLs
   - Compare data quality
   - Switch over when confident

3. **Final cleanup**
   - Delete `internal/services/atlassian/` directory
   - Remove related handlers
   - Update documentation

**Files to Eventually Delete:**
- `internal/services/atlassian/*` (entire directory)
- `internal/handlers/jira_handler.go`
- `internal/handlers/confluence_handler.go`

---

## Stage 2: MCP Agent Framework

**Goal:** Build internal tool system for agent to execute searches and queries

### Current Status
✅ `internal/services/mcp/types.go` exists
✅ `internal/services/mcp/document_service.go` exists
✅ `internal/services/mcp/formatters.go` exists
⚠️ Need to verify completeness

### Task 2.1: Review and Complete MCP Data Contracts

**Priority:** HIGH
**Estimated Complexity:** LOW
**Dependencies:** None

#### Actions

1. **Review existing types**
   - Read `internal/services/mcp/types.go`
   - Ensure MCPRequest and MCPResponse are complete
   - Add any missing fields for agent use

2. **Define tool schemas**
   ```go
   type ToolDefinition struct {
       Name        string
       Description string
       Parameters  map[string]ParameterDef
   }

   type ParameterDef struct {
       Type        string
       Description string
       Required    bool
   }
   ```

3. **Error handling structures**
   ```go
   type ToolError struct {
       Code    string
       Message string
       Details interface{}
   }
   ```

**Files to Review/Modify:**
- `internal/services/mcp/types.go`
- `internal/models/mcp.go` (if it exists)

### Task 2.2: Implement MCP Tool Router

**Priority:** HIGH
**Estimated Complexity:** MEDIUM
**Dependencies:** Task 1.2 (storage enhancements)

#### Implementation

1. **Router structure**
   ```go
   type ToolRouter struct {
       documentStorage interfaces.DocumentStorage
       logger          arbor.ILogger
   }
   ```

2. **Core method**
   ```go
   func (r *ToolRouter) ExecuteTool(ctx context.Context, req *MCPRequest) *MCPResponse
   ```

3. **Supported tools**
   - `search_documents`: Full-text search
   - `get_document_by_id`: Retrieve specific document
   - `search_by_reference`: Find documents referencing ID
   - `list_sources`: Get available data sources

4. **Tool execution flow**
   ```
   Request → Validate → Execute → Format → Response
   ```

**Files to Create:**
- `internal/services/mcp/router.go`
- `internal/services/mcp/router_test.go`
- `internal/services/mcp/tools.go` (tool definitions)

**Files to Modify:**
- `internal/app/app.go` (initialize router)

### Task 2.3: Define Agent Tool Catalog

**Priority:** MEDIUM
**Estimated Complexity:** LOW
**Dependencies:** Task 2.2

#### Tool Definitions

Create JSON/Go definitions of all available tools for the agent:

```go
var ToolCatalog = []ToolDefinition{
    {
        Name: "search_documents",
        Description: "Search the knowledge base using full-text search",
        Parameters: map[string]ParameterDef{
            "query": {
                Type: "string",
                Description: "Search query text",
                Required: true,
            },
            "sources": {
                Type: "array",
                Description: "Filter by source types (optional)",
                Required: false,
            },
            "limit": {
                Type: "integer",
                Description: "Max results (default: 5)",
                Required: false,
            },
        },
    },
    // ... more tools
}
```

**Files to Create:**
- `internal/services/mcp/catalog.go`

---

## Stage 3: Agent Orchestration Loop

**Goal:** Replace RAG chat service with streaming agent conversation loop

### Current Issues with Chat Service
- Entire file is RAG-focused
- Pointer RAG adds complexity
- Query classifier helps but still passive
- No multi-step reasoning capability

### Task 3.1: Design Agent System Prompt

**Priority:** HIGH
**Estimated Complexity:** MEDIUM
**Dependencies:** Task 2.3 (tool catalog)

#### Prompt Requirements

1. **Clear role definition**
   - You are a research assistant
   - You have access to a local knowledge base
   - You can use tools to search for information

2. **Tool usage instructions**
   - List all available tools
   - Format for tool calls (JSON)
   - When to use which tool
   - How to combine information

3. **Response format**
   ```
   THOUGHT: [reasoning about query]
   ACTION: [tool call in JSON]
   OBSERVATION: [tool result]
   THOUGHT: [further reasoning]
   ANSWER: [final answer]
   ```

4. **Constraints**
   - Maximum iterations
   - When to stop
   - How to admit uncertainty

**Files to Create:**
- `internal/services/agent/prompts.go`

**Example:**
```go
const AgentSystemPrompt = `You are a helpful research assistant with access to a local knowledge base...

Available Tools:
1. search_documents: Search for information
2. get_document_by_id: Retrieve specific document
...

Output Format:
THOUGHT: [Your reasoning]
ACTION: {"tool": "search_documents", "arguments": {"query": "..."}}
OBSERVATION: [Wait for system response]
...
ANSWER: [Final answer when ready]`
```

### Task 3.2: Implement Agent Conversation Loop

**Priority:** HIGH
**Estimated Complexity:** HIGH
**Dependencies:** All previous tasks

#### Core Loop Logic

1. **Loop structure**
   ```
   while iterations < MAX_ITERATIONS:
       1. Send conversation to LLM
       2. Parse response
       3. If ANSWER → stream final response, break
       4. If ACTION → execute tool
       5. Add OBSERVATION to conversation
       6. Stream state to UI
       7. Continue
   ```

2. **State management**
   ```go
   type AgentState struct {
       ConversationHistory []Message
       ToolCalls           []ToolCall
       Observations        []Observation
       Iteration           int
       Status              AgentStatus
   }
   ```

3. **Streaming protocol**
   ```go
   type StreamMessage struct {
       Type    string      `json:"type"`    // "thought", "action", "observation", "answer"
       Content interface{} `json:"content"`
       Step    int         `json:"step"`
   }
   ```

4. **Error handling**
   - Tool execution failures
   - LLM parsing errors
   - Timeout handling
   - Max iteration reached

**Files to Create:**
- `internal/services/agent/agent_service.go`
- `internal/services/agent/loop.go`
- `internal/services/agent/state.go`
- `internal/services/agent/parser.go`

**Files to Modify:**
- `internal/handlers/chat_handler.go` (use agent instead of RAG)

#### Detailed Implementation

**1. Agent Service Structure**
```go
type AgentService struct {
    llmService   interfaces.LLMService
    toolRouter   *mcp.ToolRouter
    logger       arbor.ILogger
    maxIterations int
}

func (s *AgentService) RunAgent(
    ctx context.Context,
    query string,
    history []Message,
    stream StreamWriter,
) (*AgentResponse, error)
```

**2. Response Parsing**
```go
func (s *AgentService) parseResponse(text string) (*AgentAction, error) {
    // Extract THOUGHT, ACTION, or ANSWER
    // Parse JSON tool calls
    // Validate format
}
```

**3. Tool Execution**
```go
func (s *AgentService) executeTool(ctx context.Context, action *AgentAction) (*Observation, error) {
    // Call tool router
    // Format result
    // Handle errors gracefully
}
```

**4. Streaming**
```go
func (s *AgentService) streamThought(stream StreamWriter, thought string)
func (s *AgentService) streamAction(stream StreamWriter, action *AgentAction)
func (s *AgentService) streamObservation(stream StreamWriter, obs *Observation)
func (s *AgentService) streamAnswer(stream StreamWriter, answer string)
```

### Task 3.3: Refactor Chat Handler

**Priority:** HIGH
**Estimated Complexity:** MEDIUM
**Dependencies:** Task 3.2

#### Changes Required

1. **Switch from ChatService to AgentService**
   ```go
   // Old
   response, err := h.chatService.Chat(ctx, req)

   // New
   response, err := h.agentService.RunAgent(ctx, req.Message, req.History, wsConn)
   ```

2. **Remove RAG logic**
   - Delete RAG config handling
   - Remove Pointer RAG references
   - Remove query classification (agent handles this)

3. **Add WebSocket streaming**
   - Pass WebSocket connection to agent
   - Agent streams intermediate steps
   - Handle connection errors gracefully

**Files to Modify:**
- `internal/handlers/chat_handler.go`

---

## Stage 4: Frontend Streaming UI

**Goal:** Display agent's real-time thought process to user

### Task 4.1: Enhance WebSocket Manager

**Priority:** MEDIUM
**Estimated Complexity:** LOW
**Dependencies:** Task 3.2

#### JavaScript Changes

1. **Message type handling**
   ```javascript
   onMessage(event) {
       const msg = JSON.parse(event.data);
       switch(msg.type) {
           case 'thought':
               this.renderThought(msg.content, msg.step);
               break;
           case 'action':
               this.renderAction(msg.content, msg.step);
               break;
           case 'observation':
               this.renderObservation(msg.content, msg.step);
               break;
           case 'answer':
               this.renderFinalAnswer(msg.content);
               break;
       }
   }
   ```

2. **Rendering functions**
   - Thought: Light grey box with thinking emoji
   - Action: Code block showing tool call
   - Observation: Collapsible data display
   - Answer: Final message in chat

**Files to Modify:**
- `pages/static/websocket-manager.js`

### Task 4.2: Update Chat UI

**Priority:** MEDIUM
**Estimated Complexity:** LOW
**Dependencies:** Task 4.1

#### HTML Changes

1. **Add agent status container**
   ```html
   <div id="agent-status" class="agent-status-box">
       <!-- Agent thinking process appears here -->
   </div>

   <div id="chat-messages">
       <!-- Final answers appear here -->
   </div>
   ```

2. **Add CSS for agent states**
   ```css
   .agent-thought { background: #f5f5f5; padding: 8px; margin: 4px 0; }
   .agent-action { background: #e3f2fd; font-family: monospace; }
   .agent-observation { background: #e8f5e9; border-left: 3px solid #4caf50; }
   ```

**Files to Modify:**
- `pages/chat.html`
- `pages/static/common.css`

### Task 4.3: Add Alpine.js Components for Agent Display

**Priority:** LOW
**Estimated Complexity:** LOW
**Dependencies:** Task 4.2

#### Component Features

- Expandable/collapsible agent steps
- Progress indicator
- Tool call visualization
- Data preview for observations

**Files to Modify:**
- `pages/static/alpine-components.js`

---

## Stage 5: Testing & Validation

**Goal:** Ensure agent works correctly with comprehensive testing

### Task 5.1: Unit Tests

**Priority:** HIGH
**Estimated Complexity:** MEDIUM

#### Test Coverage

1. **MCP Tool Router**
   - Tool execution
   - Error handling
   - Invalid requests

2. **Agent Parser**
   - THOUGHT/ACTION/ANSWER extraction
   - JSON tool call parsing
   - Malformed response handling

3. **Agent Loop**
   - Multi-step reasoning
   - Tool execution flow
   - Max iteration handling

**Files to Create:**
- `internal/services/mcp/router_test.go`
- `internal/services/agent/parser_test.go`
- `internal/services/agent/loop_test.go`

### Task 5.2: Integration Tests

**Priority:** MEDIUM
**Estimated Complexity:** MEDIUM

#### Test Scenarios

1. **Simple query** → Single search → Answer
2. **Multi-hop query** → Search → Refine → Search → Answer
3. **Reference following** → Search → Get by ID → Answer
4. **Error recovery** → Tool fails → Retry with different approach

**Files to Create:**
- `test/api/agent_integration_test.go`

### Task 5.3: UI Testing

**Priority:** LOW
**Estimated Complexity:** LOW

#### Browser Tests

- Agent status display
- Streaming updates
- Collapsible sections
- Error states

**Files to Modify:**
- `test/ui/chat_agent_test.go`

---

## Stage 6: Migration & Cleanup

**Goal:** Remove old code, update documentation

### Task 6.1: Remove RAG Code

**Priority:** LOW
**Estimated Complexity:** LOW
**Dependencies:** Agent proven working

**Files to Delete:**
- `internal/services/chat/augmented_retrieval.go`
- `internal/services/chat/document_formatter.go`
- `internal/services/chat/query_classifier.go`
- `internal/services/identifiers/` (entire directory)

**Files to Modify:**
- `internal/services/chat/chat_service.go` (simplify or delete)

### Task 6.2: Update Documentation

**Priority:** MEDIUM
**Estimated Complexity:** LOW

**Files to Update:**
- `README.md` - Update architecture description
- `CLAUDE.md` - Update development guidelines
- `docs/ARCHITECTURE.md` - Add agent flow diagrams

### Task 6.3: Remove Atlassian Services

**Priority:** LOW
**Estimated Complexity:** LOW
**Dependencies:** Generic scraper proven

**Files to Delete:**
- `internal/services/atlassian/` (entire directory)
- Related handlers and tests

---

## Implementation Order

### Phase 1: Foundation (Parallel Work Possible)
1. ✅ Stage 2.1 - Review MCP types
2. ✅ Stage 1.2 - Enhance storage methods
3. ✅ Stage 2.2 - Implement tool router
4. ✅ Stage 2.3 - Define tool catalog

### Phase 2: Core Agent (Sequential)
5. ✅ Stage 3.1 - Design system prompt
6. ✅ Stage 3.2 - Implement agent loop
7. ✅ Stage 3.3 - Refactor chat handler
8. ✅ Stage 5.1 - Unit tests

### Phase 3: User Experience (Parallel)
9. ✅ Stage 4.1 - WebSocket manager
10. ✅ Stage 4.2 - Chat UI updates
11. ✅ Stage 5.2 - Integration tests

### Phase 4: Polish (After Agent Works)
12. ⏸️ Stage 1.1 - Generic scraper (optional)
13. ⏸️ Stage 4.3 - Alpine components (optional)
14. ⏸️ Stage 5.3 - UI tests
15. ⏸️ Stage 6 - Cleanup (when ready)

---

## Risk Assessment

### High Risk
- **Agent loop complexity** - Parsing LLM responses unreliably
  - *Mitigation:* Strict output format, extensive testing, fallbacks
- **Context window management** - Agent history grows too large
  - *Mitigation:* Conversation summarization, sliding window

### Medium Risk
- **Performance degradation** - Multiple LLM calls per query
  - *Mitigation:* Caching, parallel tool execution, streaming UX
- **Tool execution failures** - Search returns no results
  - *Mitigation:* Graceful error handling, retry strategies

### Low Risk
- **Generic scraper quality** - May miss Atlassian-specific features
  - *Mitigation:* Keep Atlassian services as fallback initially
- **UI complexity** - Streaming display may confuse users
  - *Mitigation:* Progressive disclosure, collapsible sections

---

## Success Criteria

### Must Have
- ✅ Agent can execute multi-step queries
- ✅ Tool execution is reliable and logged
- ✅ Streaming works without blocking
- ✅ Error handling is graceful
- ✅ Tests cover core functionality

### Should Have
- ⭐ Agent reasoning is transparent to user
- ⭐ Performance is acceptable (<30s for complex queries)
- ⭐ UI is intuitive and responsive
- ⭐ Documentation is updated

### Nice to Have
- 🎁 Generic scraper replaces Atlassian-specific code
- 🎁 Conversation summarization for long sessions
- 🎁 Parallel tool execution for speed
- 🎁 Agent learns from user feedback

---

## Estimated Timeline

- **Phase 1:** 3-5 days (foundation)
- **Phase 2:** 5-7 days (core agent)
- **Phase 3:** 2-3 days (UI)
- **Phase 4:** 2-3 days (polish)

**Total:** 12-18 days for complete refactor

---

## Next Steps

1. Review this plan with stakeholders
2. Prioritize tasks based on business needs
3. Start with Phase 1, Task 2.1 (MCP review)
4. Build incrementally with tests
5. Keep RAG system as fallback during development
6. Switch over when agent is proven stable

---

**Document Version:** 1.0
**Last Updated:** 2025-10-13
**Owner:** Development Team
