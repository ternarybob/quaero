I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current Issues:**

1. **Line 437**: `genai.EmbeddingConfig` struct doesn't exist - should be `genai.EmbedContentConfig`
2. **Line 442**: `s.embedModel.GenerateContent()` is wrong - ADK models don't have this method, and it's for text generation not embeddings
3. **Lines 486-489**: `convertMessagesToGemini()` is called but result `geminiContents` is never used
4. **Lines 491-560**: Complex agent/runner pattern is unnecessary for simple chat completions
5. **Lines 128-143**: Two separate ADK models initialized when a single genai client would suffice
6. **Line 114**: Default model `text-embedding-004` is deprecated (EOL Jan 14, 2026); should use `gemini-embedding-001`

**Root Cause:**
The implementation confuses two different Google Go SDKs:
- **ADK** (`google.golang.org/adk`) - For agent-based workflows with tools, multi-turn conversations, complex orchestration
- **GenAI** (`google.golang.org/genai`) - For direct model access (embeddings, simple chat)

**Correct Architecture:**
- Use `genai.Client` for both embeddings and chat (simpler, more direct)
- Reserve ADK for agent service (already correctly implemented in `agents/service.go`)
- Single client instance handles both operations efficiently

**Web Search Findings:**
- Official API: `client.Models.EmbedContent(ctx, modelName, contents, config)` returns `*genai.EmbedContentResponse`
- Official API: `client.Models.GenerateContent(ctx, modelName, contents, config)` returns `*genai.GenerateContentResponse`
- Current model: `gemini-embedding-001` (GA since July 2025)
- Deprecated: `text-embedding-004` (EOL Jan 14, 2026), `embedding-001` (deprecated Oct 2025)

### Approach

Replace ADK model-based implementation with genai.Client for direct embedding and chat operations. The current code incorrectly mixes ADK's `gemini.NewModel()` with genai client methods. The fix involves creating a genai client, using `client.Models.EmbedContent()` for embeddings and `client.Models.GenerateContent()` for chat, removing the complex agent/runner pattern, and updating the model name to `gemini-embedding-001` (the current recommended model, as `text-embedding-004` is deprecated).

### Reasoning

Read the problematic `gemini_service.go` file and identified compilation errors. Examined the working `agents/service.go` to understand ADK usage patterns. Searched web for current Google GenAI Go SDK best practices and discovered that `genai.NewClient()` with `client.Models.EmbedContent()` and `client.Models.GenerateContent()` are the correct methods. Confirmed that ADK is appropriate for agent-based workflows (as in agents/service.go) but overkill for simple embeddings and chat. The model name `text-embedding-004` is deprecated; `gemini-embedding-001` is the current recommended model.

## Mermaid Diagram

sequenceDiagram
    participant Constructor as NewGeminiService
    participant GenAI as genai.Client
    participant EmbedAPI as Models.EmbedContent
    participant ChatAPI as Models.GenerateContent
    
    Note over Constructor: BEFORE (Broken)
    Constructor->>Constructor: gemini.NewModel(embedModel)
    Constructor->>Constructor: gemini.NewModel(chatModel)
    Note over Constructor: ❌ Wrong SDK - ADK not needed
    
    Note over Constructor: AFTER (Fixed)
    Constructor->>GenAI: genai.NewClient(config)
    GenAI-->>Constructor: *genai.Client
    Note over Constructor: ✅ Single client for both ops
    
    Note over EmbedAPI: Embedding Flow (Fixed)
    EmbedAPI->>EmbedAPI: Prepare content with genai.NewContentFromText
    EmbedAPI->>EmbedAPI: Create EmbedContentConfig with OutputDimensionality
    EmbedAPI->>GenAI: client.Models.EmbedContent(ctx, model, contents, config)
    GenAI-->>EmbedAPI: *EmbedContentResponse
    EmbedAPI->>EmbedAPI: Extract result.Embeddings[0].Values
    EmbedAPI-->>EmbedAPI: Return []float32 (768-d)
    
    Note over ChatAPI: Chat Flow (Fixed)
    ChatAPI->>ChatAPI: convertMessagesToGemini(messages)
    ChatAPI->>GenAI: client.Models.GenerateContent(ctx, model, contents, config)
    GenAI-->>ChatAPI: *GenerateContentResponse
    ChatAPI->>ChatAPI: Extract resp.Candidates[0].Content.Parts
    ChatAPI->>ChatAPI: Concatenate part.Text fields
    ChatAPI-->>ChatAPI: Return string response
    
    Note over ChatAPI: ❌ REMOVED: Complex agent/runner pattern
    Note over ChatAPI: ✅ ADDED: Direct GenerateContent call

## Proposed File Changes

### internal\services\llm\gemini_service.go(MODIFY)

References: 

- internal\services\agents\service.go
- internal\common\config.go

**Fix compilation errors by replacing ADK models with genai.Client for direct embedding and chat operations.**

**Update Imports (lines 3-18):**
- Remove unused imports: `"google.golang.org/adk/agent"`, `"google.golang.org/adk/agent/llmagent"`, `"google.golang.org/adk/runner"`
- Keep: `"google.golang.org/adk/model"` (for type reference only, can be removed if not needed)
- Remove: `"google.golang.org/adk/model/gemini"` (no longer using `gemini.NewModel`)
- Keep: `"google.golang.org/genai"` (primary SDK for embeddings and chat)
- All other imports remain unchanged

**Update GeminiService Struct (lines 22-28):**
- Replace `embedModel model.LLM` field with `client *genai.Client`
- Remove `chatModel model.LLM` field (single client handles both operations)
- Keep all other fields: `config`, `logger`, `timeout`
- Rationale: Single genai client is more efficient and simpler than two ADK models

**Fix NewGeminiService Constructor (lines 101-163):**

1. **Update default model name (line 114):**
   - Change from `"text-embedding-004"` to `"gemini-embedding-001"`
   - Reason: `text-embedding-004` is deprecated (EOL Jan 14, 2026)
   - `gemini-embedding-001` is the current GA model (since July 2025)

2. **Replace model initialization (lines 126-143):**
   - Remove both `gemini.NewModel()` calls for embed and chat models
   - Create single genai client using `genai.NewClient(ctx, &genai.ClientConfig{...})`
   - ClientConfig fields: `APIKey: config.LLM.GoogleAPIKey`, `Backend: genai.BackendGeminiAPI`
   - Handle error: `if err != nil { return nil, fmt.Errorf("failed to initialize genai client: %w", err) }`
   - Follow exact pattern from web search results

3. **Update service struct initialization (lines 146-152):**
   - Change `embedModel: embedModel` to `client: client`
   - Remove `chatModel: chatModel` line
   - Keep all other field assignments unchanged

4. **Update success log (lines 154-160):**
   - Keep existing log fields
   - Log message remains: "Gemini LLM service initialized successfully"

**Fix generateEmbedding Method (lines 435-472):**

1. **Replace embedding config (lines 437-439):**
   - Change `genai.EmbeddingConfig` to `genai.EmbedContentConfig`
   - Use pointer for OutputDimensionality: `OutputDimensionality: &s.config.EmbedDimension` (note: field expects `*int32`, may need type conversion)
   - Correct struct field name based on web search results

2. **Fix embedding generation call (lines 442-445):**
   - Replace `s.embedModel.GenerateContent(ctx, text, embeddingConfig)` with:
   - `s.client.Models.EmbedContent(ctx, s.config.EmbedModelName, []*genai.Content{genai.NewContentFromText(text, genai.RoleUser)}, embeddingConfig)`
   - Return type: `*genai.EmbedContentResponse` (not `*model.LLMResponse`)
   - Handle error unchanged

3. **Fix embedding extraction (lines 448-460):**
   - Response structure: `result.Embeddings` is a slice of embedding objects
   - Access first embedding: `if len(result.Embeddings) > 0 { embedding = result.Embeddings[0].Values }`
   - The `Values` field contains `[]float32` directly (no conversion loop needed)
   - Simplify extraction logic - remove the nested loop over Parts
   - Handle case where `result.Embeddings` is empty or nil

4. **Keep validation (lines 462-469):**
   - Dimension validation logic remains unchanged
   - Error messages remain unchanged

**Fix generateCompletion Method (lines 484-560):**

1. **Use convertMessagesToGemini result (lines 486-489):**
   - Keep the conversion call: `geminiContents, err := convertMessagesToGemini(messages)`
   - Remove the "declared and not used" error by actually using `geminiContents`

2. **Replace entire agent/runner pattern (lines 491-553):**
   - Remove: `llmagent.Config`, `llmagent.New()`, `runner.Config`, `runner.New()`, agent loop
   - Remove: Last user message extraction logic (lines 517-527)
   - Remove: Initial content creation (lines 529-534)
   - Remove: Agent runner execution loop (lines 536-553)

3. **Add direct GenerateContent call:**
   - Call: `resp, err := s.client.Models.GenerateContent(ctx, s.config.ChatModelName, geminiContents, &genai.GenerateContentConfig{Temperature: genai.Ptr(float32(0.7))})`
   - Handle error: `if err != nil { return "", fmt.Errorf("chat generation failed: %w", err) }`
   - Extract text from response: Iterate over `resp.Candidates[0].Content.Parts` and concatenate `part.Text` fields
   - Validate response is not empty before returning

4. **Simplify response extraction:**
   - Check `resp.Candidates` is not empty
   - Access first candidate: `resp.Candidates[0].Content.Parts`
   - Build response string by concatenating all text parts
   - Return error if no text found: `"no response generated from chat model"`

**Update Health Check Methods (lines 336-393):**

1. **Fix performEmbeddingHealthCheck (lines 336-360):**
   - Update call to use fixed `generateEmbedding` method
   - No other changes needed - method signature remains same
   - Validation logic remains unchanged

2. **Fix performChatHealthCheck (lines 364-393):**
   - Update call to use fixed `generateCompletion` method
   - No other changes needed - method signature remains same
   - Validation logic remains unchanged

3. **Update HealthCheck method (lines 287-332):**
   - Replace `s.embedModel == nil` check with `s.client == nil`
   - Remove `s.chatModel == nil` check (no longer exists)
   - Remove model name checks (lines 301-309) - client doesn't expose model names
   - Keep health check probe calls unchanged
   - Update success log to remove model name fields (client handles multiple models)

**Update Close Method (lines 414-422):**
- Replace `s.embedModel = nil` and `s.chatModel = nil` with:
- `if s.client != nil { s.client.Close() }` (genai.Client has Close method)
- Set `s.client = nil` after closing
- Keep log message unchanged

**Keep Unchanged:**
- `convertMessagesToGemini` function (lines 30-77) - correctly implemented
- `Embed` method public interface (lines 185-219) - only internal call changes
- `Chat` method public interface (lines 241-273) - only internal call changes
- `GetMode` method (lines 402-404) - returns `LLMModeCloud` correctly
- All error handling patterns and logging statements
- All method signatures and documentation comments

**Testing Verification:**
- Build should succeed without compilation errors
- Embedding generation should return 768-dimension `[]float32` vectors
- Chat completion should return non-empty string responses
- Health checks should pass with valid API key
- Service should handle missing API key gracefully (error during initialization)