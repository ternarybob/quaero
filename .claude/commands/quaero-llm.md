# quaero-llm

Implements the Ollama LLM client for Quaero, providing text generation and vision model capabilities.

## Usage

```
/quaero-llm <quaero-project-path>
```

## Arguments

- `quaero-project-path` (required): Path to Quaero monorepo

## What it does

### Phase 1: LLM Interface Definition

1. **LLM Interface** (`internal/llm/interface.go`)
   - Define generic LLM interface
   - Support for text and vision models
   - Request/response structures

2. **Interface Methods**
   ```go
   type LLMClient interface {
       Complete(ctx context.Context, prompt string) (string, error)
       CompleteWithOptions(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
       AnalyzeImage(ctx context.Context, image []byte, prompt string) (string, error)
   }

   type CompletionRequest struct {
       Prompt      string
       Model       string
       MaxTokens   int
       Temperature float64
       StopTokens  []string
   }

   type CompletionResponse struct {
       Text      string
       Model     string
       Tokens    int
       Duration  time.Duration
   }
   ```

### Phase 2: Ollama Client Implementation

1. **Ollama Client** (`internal/llm/ollama/client.go`)
   - HTTP client for Ollama API
   - Text completion using Qwen2.5-32B
   - Streaming support (optional)
   - Error handling and retries

2. **Client Structure**
   ```go
   type Client struct {
       baseURL     string
       httpClient  *http.Client
       logger      *arbor.Logger
       textModel   string
       visionModel string
   }

   type OllamaRequest struct {
       Model  string `json:"model"`
       Prompt string `json:"prompt"`
       Stream bool   `json:"stream"`
   }

   type OllamaResponse struct {
       Model     string `json:"model"`
       Response  string `json:"response"`
       Done      bool   `json:"done"`
   }
   ```

3. **Core Methods**
   - `Complete(ctx, prompt string) (string, error)` - Basic completion
   - `CompleteWithOptions(ctx, req) (*Response, error)` - Advanced completion
   - `StreamCompletion(ctx, prompt string) (<-chan string, error)` - Streaming

### Phase 3: Vision Model Support

1. **Vision Client** (`internal/llm/ollama/vision.go`)
   - Image analysis using Llama3.2-Vision-11B
   - Base64 image encoding
   - Image description generation
   - OCR capabilities

2. **Vision Methods**
   ```go
   type VisionRequest struct {
       Model  string   `json:"model"`
       Prompt string   `json:"prompt"`
       Images []string `json:"images"` // base64 encoded
   }

   func (c *Client) AnalyzeImage(ctx context.Context, image []byte, prompt string) (string, error) {
       // Encode image to base64
       encoded := base64.StdEncoding.EncodeToString(image)

       req := VisionRequest{
           Model:  c.visionModel,
           Prompt: prompt,
           Images: []string{encoded},
       }

       // Send to Ollama
       resp := c.makeRequest(ctx, "/api/generate", req)

       return resp.Response, nil
   }
   ```

### Phase 4: Configuration

1. **Config Structure**
   ```go
   type Config struct {
       URL         string
       TextModel   string  // "qwen2.5:32b"
       VisionModel string  // "llama3.2-vision:11b"
       Timeout     int     // seconds
   }
   ```

2. **Model Defaults**
   - Text: `qwen2.5:32b` (Qwen2.5-32B)
   - Vision: `llama3.2-vision:11b` (Llama3.2-Vision-11B)
   - URL: `http://localhost:11434`

### Phase 5: Error Handling

1. **Error Types**
   - Connection errors (Ollama not running)
   - Model not found errors
   - Timeout errors
   - API errors

2. **Retry Logic**
   - Retry on connection errors (3 attempts)
   - Exponential backoff
   - Timeout handling

### Phase 6: Testing

1. **Unit Tests** (`internal/llm/ollama/client_test.go`)
   - Mock Ollama API
   - Test completions
   - Test vision analysis
   - Error handling

2. **Integration Tests** (`test/integration/llm_test.go`)
   - Real Ollama connection
   - Actual completions
   - Performance benchmarks

### Phase 7: Mock Implementation

1. **Mock LLM** (`internal/llm/mock/client.go`)
   - For testing without Ollama
   - Implements LLMClient interface
   - Returns predefined responses

## Ollama API Integration

### Text Completion Flow
```
1. Prepare request
   - Model: qwen2.5:32b
   - Prompt: RAG context + question
   - Stream: false

2. Send POST to /api/generate
   {
     "model": "qwen2.5:32b",
     "prompt": "Based on...",
     "stream": false
   }

3. Receive response
   {
     "model": "qwen2.5:32b",
     "response": "To onboard a new user...",
     "done": true
   }

4. Return text
```

### Vision Analysis Flow
```
1. Load image from file
2. Encode to base64
3. Prepare vision request
   - Model: llama3.2-vision:11b
   - Prompt: "Describe this diagram"
   - Images: [base64_image]

4. Send POST to /api/generate
   {
     "model": "llama3.2-vision:11b",
     "prompt": "Describe this diagram",
     "images": ["iVBORw0KGgoAAAANS..."]
   }

5. Receive description
   "This diagram shows an authentication flow..."
```

## Code Structure

```go
// internal/llm/ollama/client.go
type Client struct {
    baseURL     string
    httpClient  *http.Client
    logger      *arbor.Logger
    textModel   string
    visionModel string
}

func NewClient(config *Config, logger *arbor.Logger) *Client {
    return &Client{
        baseURL:     config.URL,
        httpClient:  &http.Client{Timeout: time.Duration(config.Timeout) * time.Second},
        logger:      logger,
        textModel:   config.TextModel,
        visionModel: config.VisionModel,
    }
}

func (c *Client) Complete(ctx context.Context, prompt string) (string, error) {
    req := OllamaRequest{
        Model:  c.textModel,
        Prompt: prompt,
        Stream: false,
    }

    resp, err := c.makeRequest(ctx, "/api/generate", req)
    if err != nil {
        return "", err
    }

    return resp.Response, nil
}

func (c *Client) makeRequest(ctx context.Context, endpoint string, payload interface{}) (*OllamaResponse, error) {
    data, _ := json.Marshal(payload)

    req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+endpoint, bytes.NewBuffer(data))
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        c.logger.Error("Ollama request failed", "error", err)
        return nil, err
    }
    defer resp.Body.Close()

    var ollamaResp OllamaResponse
    json.NewDecoder(resp.Body).Decode(&ollamaResp)

    return &ollamaResp, nil
}
```

## Test-Driven Development (TDD) Workflow

**CRITICAL**: Follow TDD methodology for ALL code implementation.

### TDD Cycle (Red-Green-Refactor)

For EACH component, function, or feature:

1. **RED - Write Failing Test First**
   ```bash
   # Create test file before implementation
   touch internal/component/component_test.go

   # Write test that describes desired behavior
   func TestComponentBehavior(t *testing.T) {
       // Arrange - setup test data
       // Act - call the function (doesn't exist yet)
       // Assert - verify expected behavior
   }

   # Run test - should FAIL
   go test ./internal/component/... -v
   # Output: undefined: ComponentFunction
   ```

2. **GREEN - Write Minimal Code to Pass**
   ```bash
   # Implement just enough code to make test pass
   # Run test again
   go test ./internal/component/... -v
   # Output: PASS
   ```

3. **REFACTOR - Improve Code Quality**
   ```bash
   # Refactor while keeping tests green
   # Run test after each change
   go test ./internal/component/... -v
   ```

4. **REPEAT** - For next feature/function

### Testing Requirements by Component

**Before writing ANY implementation code:**

1. **Interfaces** - Create interface test
   ```go
   func TestInterfaceImplementation(t *testing.T) {
       var _ models.Source = (*ConfluenceSource)(nil) // Compile-time check
   }
   ```

2. **Core Functions** - Table-driven tests
   ```go
   func TestProcessMarkdown(t *testing.T) {
       tests := []struct{
           name     string
           input    string
           expected string
       }{
           {"basic html", "<p>test</p>", "test"},
           // Add more cases
       }
       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               result := ProcessMarkdown(tt.input)
               assert.Equal(t, tt.expected, result)
           })
       }
   }
   ```

3. **API Clients** - Mock HTTP responses
   ```go
   func TestGetPages(t *testing.T) {
       // Setup mock server
       server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
           w.WriteHeader(200)
           json.NewEncoder(w).Encode(mockPages)
       }))
       defer server.Close()

       // Test with mock
       client := NewClient(server.URL)
       pages, err := client.GetPages()

       assert.NoError(t, err)
       assert.Len(t, pages, 2)
   }
   ```

4. **Integration Tests** - Test component interactions
   ```go
   func TestFullWorkflow(t *testing.T) {
       // Setup
       storage := mock.NewStorage()
       source := NewSource(config, storage)

       // Execute
       docs, err := source.Collect(context.Background())

       // Verify
       assert.NoError(t, err)
       assert.Greater(t, len(docs), 0)
   }
   ```

### Continuous Testing Workflow

**After EVERY code change:**

```bash
# 1. Run specific component tests
go test ./internal/component/... -v

# 2. If pass, run all tests
go test ./... -v

# 3. Check coverage (must be >80%)
go test ./... -cover

# 4. If all pass, proceed to next test/feature
```

### Test Organization

```
internal/component/
├── component.go           # Implementation
├── component_test.go      # Unit tests
└── testdata/              # Test fixtures
    ├── input.json
    └── expected.json

test/integration/
├── component_flow_test.go # Integration tests
└── fixtures/              # Shared fixtures
```

### Testing Checklist

Before marking ANY component complete:
- [ ] Unit tests written BEFORE implementation
- [ ] All tests passing (`go test ./... -v`)
- [ ] Test coverage >80% (`go test -cover`)
- [ ] Table-driven tests for multiple cases
- [ ] Mock external dependencies
- [ ] Integration tests for workflows
- [ ] Edge cases tested (nil, empty, errors)
- [ ] Error paths tested


## Examples

### Implement Ollama Client
```
/quaero-llm C:\development\quaero
```

### Configure Models
```
/quaero-llm C:\development\quaero --text-model=qwen2.5:32b --vision-model=llama3.2-vision:11b
```

## Validation

After implementation, verifies:
- ✓ LLM interface defined
- ✓ Ollama client connects
- ✓ Text completions working
- ✓ Vision model analyzes images
- ✓ Base64 encoding correct
- ✓ Error handling robust
- ✓ Retry logic functional
- ✓ Mock client for testing
- ✓ Unit tests passing
- ✓ Integration tests passing (if Ollama available)

## Output

Provides detailed report:
- Files created/modified
- LLM interface defined
- Ollama client implementation
- Vision support status
- Error handling
- Mock client for testing
- Tests created

---

**Agent**: quaero-llm

**Prompt**: Implement the Ollama LLM client for Quaero at {{args.[0]}}.

## Implementation Tasks

1. **Define LLM Interface** (`internal/llm/interface.go`)
   - LLMClient interface
   - CompletionRequest/Response structs
   - Support for text and vision

2. **Implement Ollama Client** (`internal/llm/ollama/client.go`)
   - HTTP client for Ollama API
   - Complete() method for text generation
   - CompleteWithOptions() for advanced usage
   - Connection and error handling

3. **Add Vision Support** (`internal/llm/ollama/vision.go`)
   - AnalyzeImage() method
   - Base64 image encoding
   - Vision model integration (Llama3.2-Vision-11B)
   - Image description generation

4. **Configure Models** (`internal/llm/ollama/config.go`)
   - Config struct
   - Default models (qwen2.5:32b, llama3.2-vision:11b)
   - URL configuration
   - Timeout settings

5. **Add Error Handling**
   - Connection error handling
   - Model not found errors
   - Retry logic with backoff
   - Timeout handling

6. **Create Mock Client** (`internal/llm/mock/client.go`)
   - Implements LLMClient interface
   - For testing without Ollama
   - Predefined responses

7. **Create Tests**
   - Unit tests with mock API
   - Test text completions
   - Test vision analysis
   - Integration tests (if Ollama available)

## Code Quality Standards

- Implements LLMClient interface
- HTTP client with timeouts
- Structured logging (arbor)
- Comprehensive error handling
- Retry logic for resilience
- Base64 encoding for images
- Mock client for testing
- 80%+ test coverage

## Success Criteria

✓ LLM interface fully defined
✓ Ollama client connects
✓ Text completions functional
✓ Vision model analyzes images
✓ Error handling robust
✓ Retry logic works
✓ Mock client available
✓ All tests passing
✓ Can generate answers via Ollama
