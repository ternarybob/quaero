# Plan: Fix Gemini ADK Integration Issue

**Agent:** Planner (Claude Opus)
**Created:** 2025-11-18 20:17:50
**Issue:** Nil pointer dereference in Google ADK runner during keyword extraction

---

## Problem Analysis

### Current State

**What Works:**
- ✅ Test infrastructure: 3 documents created successfully
- ✅ Agent jobs: Created and enqueued correctly
- ✅ Agent worker: Registered and picks up jobs
- ✅ Gemini API: Verified working via curl (user confirmed)
- ✅ Service initialization: Agent service initializes with health check passing

**What Fails:**
- ❌ Runtime panic: `runtime error: invalid memory address or nil pointer dereference`
- ❌ Location: `google.golang.org/adk@v0.1.0/runner/runner.go:78`
- ❌ Trigger: Inside `agentRunner.Run()` call at `keyword_extractor.go:161`

### Root Cause Analysis

After reviewing the go-genai example and our implementation, I've identified a **fundamental architecture mismatch**:

**Current Implementation (keyword_extractor.go):**
```go
// Lines 11-15: Imports BOTH ADK and genai
import (
    "google.golang.org/adk/agent"
    "google.golang.org/adk/agent/llmagent"
    "google.golang.org/adk/model"
    "google.golang.org/adk/runner"
    "google.golang.org/genai"
)

// Lines 76-83: Creates ADK model via gemini.NewModel()
geminiModel, err := gemini.NewModel(ctx, config.AgentModel, &genai.ClientConfig{
    APIKey:  apiKey,
    Backend: genai.BackendGeminiAPI,
})

// Lines 138-161: Uses ADK's agent/runner pattern
llmAgent, err := llmagent.New(agentConfig)
agentRunner, err := runner.New(runnerConfig)
for event, err := range agentRunner.Run(ctx, "user", "session_"+documentID, initialContent, agent.RunConfig{}) {
    // Process events...
}
```

**go-genai Example (chat.go):**
```go
// Simple, direct API usage - NO ADK!
client, err := genai.NewClient(ctx, nil)
model := "gemini-2.5-flash"
config := &genai.GenerateContentConfig{
    Temperature: genai.Ptr[float32](0.5)
}
chat, err := client.Chats.Create(ctx, model, config, nil)
result, err := chat.SendMessage(ctx, genai.Part{Text: "..."})
fmt.Println(result.Text())
```

**Key Differences:**

1. **ADK vs Direct API:**
   - Our code uses Google ADK (Agent Development Kit) - a complex framework with agents, runners, and event loops
   - The example uses direct genai client API - simple, straightforward content generation

2. **Model Initialization:**
   - Our code: `gemini.NewModel()` creates an ADK model wrapper
   - Example: `genai.NewClient()` creates a direct API client

3. **Execution Pattern:**
   - Our code: Complex agent loop with runner.Run() returning event sequences
   - Example: Simple chat.SendMessage() returning result directly

4. **Library Versions:**
   - ADK: v0.1.0 (very early version, likely unstable)
   - genai: v1.34.0 (mature, stable)

### Why This Fails

The nil pointer dereference at `runner.go:78` suggests the ADK runner is not properly initialized or has internal state issues. Given:
- ADK is at v0.1.0 (experimental/early development)
- genai library is at v1.34.0 (mature)
- User's example shows direct genai usage without ADK
- Our Gemini API key works via curl (direct API call)

**Conclusion:** We should **abandon the ADK pattern** and use the direct genai client API pattern shown in the example.

---

## Solution Strategy

### Option A: Switch to Direct genai Client (RECOMMENDED)

**Why:**
- Simpler, more stable API
- Example provided by user directly demonstrates this pattern
- Removes dependency on experimental ADK framework
- Matches the curl test that works

**Implementation:**
1. Replace ADK imports with direct genai client
2. Simplify Execute() to use genai.NewClient() and GenerateContent()
3. Remove agent/runner pattern, use direct content generation
4. Keep existing prompt engineering and JSON parsing logic

**Files to Modify:**
- `internal/services/agents/service.go` - Model initialization
- `internal/services/agents/keyword_extractor.go` - Execute() method

### Option B: Debug and Fix ADK Integration

**Why NOT to do this:**
- ADK is v0.1.0 (too early/unstable)
- No clear documentation on what's wrong
- User example doesn't use ADK
- Would require deep diving into ADK internals

---

## Implementation Plan

### Phase 1: Refactor service.go (Model Initialization)

**File:** `internal/services/agents/service.go`

**Changes:**

1. Replace ADK model initialization with genai client:
```go
// OLD (lines 76-83):
geminiModel, err := gemini.NewModel(ctx, config.AgentModel, &genai.ClientConfig{
    APIKey:  apiKey,
    Backend: genai.BackendGeminiAPI,
})

// NEW:
genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
    APIKey:  apiKey,
    Backend: genai.BackendGeminiAPI,
})
```

2. Update Service struct to store client and model name:
```go
// OLD:
type Service struct {
    config  *common.GeminiConfig
    logger  arbor.ILogger
    model   model.LLM  // ADK model
    agents  map[string]AgentExecutor
    timeout time.Duration
}

// NEW:
type Service struct {
    config    *common.GeminiConfig
    logger    arbor.ILogger
    client    *genai.Client  // Direct genai client
    modelName string         // Model name to use
    agents    map[string]AgentExecutor
    timeout   time.Duration
}
```

3. Update AgentExecutor interface:
```go
// OLD:
type AgentExecutor interface {
    Execute(ctx context.Context, model model.LLM, input map[string]interface{}) (map[string]interface{}, error)
    GetType() string
}

// NEW:
type AgentExecutor interface {
    Execute(ctx context.Context, client *genai.Client, modelName string, input map[string]interface{}) (map[string]interface{}, error)
    GetType() string
}
```

4. Update Execute() to pass client instead of model:
```go
// OLD (line 160):
output, err := agent.Execute(timeoutCtx, s.model, input)

// NEW:
output, err := agent.Execute(timeoutCtx, s.client, s.modelName, input)
```

5. Update HealthCheck() to validate client:
```go
// OLD (lines 194-206):
if s.model == nil {
    return fmt.Errorf("agent service model is not initialized")
}
modelName := s.model.Name()
if modelName == "" {
    return fmt.Errorf("ADK model name is empty")
}

// NEW:
if s.client == nil {
    return fmt.Errorf("agent service client is not initialized")
}
if s.modelName == "" {
    return fmt.Errorf("model name is not set")
}
// Optional: Test with a simple generation call to verify API key works
```

6. Update Close() to close client:
```go
// NEW:
func (s *Service) Close() error {
    s.logger.Info().Msg("Closing agent service")
    if s.client != nil {
        s.client.Close()
    }
    s.client = nil
    s.agents = nil
    return nil
}
```

### Phase 2: Refactor keyword_extractor.go (Direct API Usage)

**File:** `internal/services/agents/keyword_extractor.go`

**Changes:**

1. Update imports - remove ADK, keep genai:
```go
// OLD (lines 11-16):
import (
    "google.golang.org/adk/agent"
    "google.golang.org/adk/agent/llmagent"
    "google.golang.org/adk/model"
    "google.golang.org/adk/runner"
    "google.golang.org/genai"
)

// NEW:
import (
    "google.golang.org/genai"
)
```

2. Update Execute() signature:
```go
// OLD (line 99):
func (k *KeywordExtractor) Execute(ctx context.Context, llmModel model.LLM, input map[string]interface{}) (map[string]interface{}, error) {

// NEW:
func (k *KeywordExtractor) Execute(ctx context.Context, client *genai.Client, modelName string, input map[string]interface{}) (map[string]interface{}, error) {
```

3. Replace ADK agent execution with direct GenerateContent:
```go
// OLD (lines 106-174): Complex ADK agent/runner pattern

// NEW: Simple, direct content generation
instruction := fmt.Sprintf(`You are a keyword extraction specialist.

Task: Extract exactly %d of the most semantically relevant keywords from the document.

Rules:
- Single words or short phrases (2-3 words max)
- Domain-specific terminology and technical concepts
- No stop words (the, is, and, etc.)
- Extract exactly %d keywords (no more, no less)

Output Format: JSON only, no markdown fences
{"keywords": ["keyword1", "keyword2"], "confidence": {"keyword1": 0.95, "keyword2": 0.87}}

If you cannot assign meaningful confidence scores, use simple array: ["keyword1", "keyword2"]

Document:
%s`, maxKeywords, maxKeywords, content)

// Generate content with direct API call
config := &genai.GenerateContentConfig{
    Temperature: genai.Ptr(float32(0.3)),
}

result, err := client.Models.GenerateContent(ctx, modelName, []*genai.Content{
    {
        Role: "user",
        Parts: []*genai.Part{
            genai.NewPartFromText(instruction),
        },
    },
}, config)

if err != nil {
    return nil, fmt.Errorf("failed to generate content for document %s: %w", documentID, err)
}

// Extract text from response
var response string
if result.Candidates != nil && len(result.Candidates) > 0 {
    candidate := result.Candidates[0]
    if candidate.Content != nil && candidate.Content.Parts != nil {
        for _, part := range candidate.Content.Parts {
            if part.Text != "" {
                response += part.Text
            }
        }
    }
}

if response == "" {
    return nil, fmt.Errorf("no response from API for document %s", documentID)
}

// Continue with existing parsing logic...
response = cleanMarkdownFences(response)
keywords, confidence, err := parseKeywordResponse(response, maxKeywords)
// ... rest remains the same
```

4. Keep all helper functions unchanged:
   - `validateInput()` - no changes needed
   - `cleanMarkdownFences()` - no changes needed
   - `parseKeywordResponse()` - no changes needed
   - `GetType()` - no changes needed
   - Test helpers - no changes needed

### Phase 3: Testing and Validation

**Steps:**

1. Run the test:
```bash
cd test/ui && go test -timeout 720s -run "^TestKeywordJob$" -v
```

2. Expected outcomes:
   - ✅ Agent worker registers successfully
   - ✅ Agent jobs created and picked up
   - ✅ Keyword extraction completes without panic
   - ✅ result_count > 0
   - ✅ Test passes

3. Review logs for:
   - Service initialization with genai client
   - Successful content generation
   - Keywords extracted and stored
   - No panics or runtime errors

4. If issues arise:
   - Check error messages from genai client
   - Verify API key is still valid
   - Ensure config.AgentModel is supported (gemini-2.0-flash)
   - Review JSON parsing of response

---

## Risk Assessment

### Low Risk:
- Direct genai API is stable (v1.34.0)
- Example provided by user demonstrates working pattern
- Simpler code = fewer failure points
- User confirmed API key works via curl

### Medium Risk:
- Interface change affects all current/future agent types
- Need to update any other agents if they exist
- Testing required to verify no regression

### Mitigation:
- Keep helper functions unchanged (parsing, validation)
- Preserve existing prompt engineering
- Maintain same input/output contract
- Test thoroughly before considering other agent types

---

## Success Criteria

1. ✅ Test runs without panic/crash
2. ✅ Keywords extracted from all 3 test documents
3. ✅ result_count > 0 in job response
4. ✅ Test passes: `TestKeywordJob` PASS
5. ✅ Service logs show successful generation
6. ✅ No nil pointer dereferences

---

## Implementation Order

1. **Implementer Agent:** Modify service.go (Phase 1)
2. **Implementer Agent:** Modify keyword_extractor.go (Phase 2)
3. **Validator Agent:** Run test (Phase 3)
4. **Validator Agent:** Review logs and verify success criteria
5. **If test fails:** Iterate with Implementer to fix issues

---

## Additional Notes

- Keep documentation comments up to date
- Remove ADK references from comments
- Update any mentions of "ADK" to "genai direct API"
- Consider future agents: they will follow the same direct API pattern

---

## References

- User's curl test: Proves Gemini API key works
- go-genai example: https://github.com/googleapis/go-genai/blob/main/examples/chats/chat.go
- Current implementation: `internal/services/agents/keyword_extractor.go:161`
- Stack trace: `runner.go:78` in ADK v0.1.0
- Library versions: ADK v0.1.0, genai v1.34.0
