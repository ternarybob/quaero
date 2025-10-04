# quaero-rag

Implements the RAG (Retrieval-Augmented Generation) engine for Quaero, orchestrating search, context building, and LLM answer generation.

## Usage

```
/quaero-rag <quaero-project-path>
```

## Arguments

- `quaero-project-path` (required): Path to Quaero monorepo

## What it does

### Phase 1: RAG Interface Definition

1. **RAG Interface** (`pkg/models/rag.go`)
   - Define RAG interface for query processing
   - Query result structure
   - Configuration options

2. **Interface Methods**
   ```go
   type RAG interface {
       Query(ctx context.Context, question string) (*Answer, error)
       BuildContext(docs []*Document) (string, error)
       ProcessImages(images []*Image) ([]string, error)
   }

   type Answer struct {
       Text          string
       Sources       []*Document
       ProcessingTime time.Duration
       Confidence    float64
   }
   ```

### Phase 2: Search Component

1. **Search Logic** (`internal/rag/search.go`)
   - Query analysis and expansion
   - Hybrid search (full-text + vector)
   - Result ranking and filtering
   - Relevance scoring

2. **Search Methods**
   - `Search(query string) ([]*models.Document, error)`
   - `HybridSearch(query string, embedding []float64) ([]*models.Document, error)`
   - `RankResults(results []*models.Document, query string) []*models.Document`
   - `FilterByRelevance(results []*models.Document, minScore float64) []*models.Document`

### Phase 3: Context Builder

1. **Context Building** (`internal/rag/context.go`)
   - Assemble context from retrieved documents
   - Chunk selection and prioritization
   - Context window management
   - Source attribution

2. **Context Methods**
   - `BuildContext(docs []*models.Document, maxTokens int) (string, error)`
   - `SelectBestChunks(doc *models.Document, query string) []models.Chunk`
   - `FormatContext(chunks []models.Chunk) string`
   - `AddSourceReferences(context string, docs []*models.Document) string`

### Phase 4: Vision Processing

1. **Image Analysis** (`internal/rag/vision.go`)
   - Process images from relevant documents
   - Vision model integration (Llama3.2-Vision)
   - Image description generation
   - OCR for diagrams

2. **Vision Methods**
   - `ProcessImages(images []*models.Image) ([]string, error)`
   - `GenerateImageDescription(image *models.Image) (string, error)`
   - `ExtractTextFromImage(image *models.Image) (string, error)`
   - `IncludeImageContext(context string, descriptions []string) string`

### Phase 5: RAG Engine

1. **Engine Implementation** (`internal/rag/engine.go`)
   - Orchestrates entire RAG workflow
   - Integrates storage, LLM, and vision components
   - Handles errors and fallbacks
   - Tracks performance metrics

2. **Engine Structure**
   ```go
   type Engine struct {
       storage Storage
       llm     LLMClient
       logger  *arbor.Logger
       config  *Config
   }

   type Config struct {
       MaxContextTokens  int
       MinRelevanceScore float64
       UseVision         bool
       MaxSources        int
   }
   ```

3. **Workflow**
   ```
   1. Receive question
   2. Search storage for relevant documents
   3. Select best chunks from documents
   4. Build context string
   5. If images present & UseVision:
      - Process images with vision model
      - Add descriptions to context
   6. Send context + question to LLM
   7. Receive and format answer
   8. Return answer with sources
   ```

### Phase 6: Prompt Engineering

1. **Prompt Templates** (`internal/rag/prompts.go`)
   - System prompts for different query types
   - Context formatting
   - Source citation instructions

2. **Template Examples**
   ```go
   const systemPrompt = `You are a helpful assistant answering questions based on internal documentation.

   Guidelines:
   - Answer based ONLY on the provided context
   - If information is not in context, say so
   - Cite sources using [Source: Title]
   - Be concise and accurate`

   const contextTemplate = `Based on the following documentation:

   %s

   Answer the question: %s`
   ```

### Phase 7: Testing

1. **Unit Tests** (`internal/rag/engine_test.go`)
   - Search functionality
   - Context building
   - Vision processing
   - End-to-end query flow

2. **Integration Tests** (`test/integration/e2e_query_test.go`)
   - Full RAG pipeline
   - With real storage and LLM
   - Performance benchmarks

## RAG Workflow

```
User Question: "How to onboard a new user?"
   ↓
1. Search Phase
   - Full-text search: "onboard new user"
   - Vector search: embedding(question)
   - Results: 5 documents (Confluence pages, Jira issues)
   ↓
2. Ranking Phase
   - Score by relevance
   - Filter below threshold
   - Select top 3 documents
   ↓
3. Context Building
   - Extract best chunks from each document
   - Format with source references
   - Fit within token limit (e.g., 4000 tokens)
   ↓
4. Vision Processing (if applicable)
   - Find images in documents
   - Generate descriptions with vision model
   - Add to context: "Diagram showing: onboarding workflow"
   ↓
5. LLM Query
   - Build prompt: System + Context + Question
   - Send to Ollama (Qwen2.5-32B)
   - Receive answer
   ↓
6. Answer Formatting
   - Add source citations
   - Format for CLI/web display
   - Return to user
```

## Code Structure

```go
// internal/rag/engine.go
type Engine struct {
    storage Storage
    llm     LLMClient
    logger  *arbor.Logger
    config  *Config
}

func NewEngine(storage Storage, llm LLMClient, config *Config, logger *arbor.Logger) *Engine

func (e *Engine) Query(ctx context.Context, question string) (*Answer, error) {
    start := time.Now()

    // 1. Search
    e.logger.Info("Searching for relevant documents", "question", question)
    docs, err := e.search(question)
    if err != nil {
        return nil, err
    }

    // 2. Build context
    e.logger.Info("Building context", "docs", len(docs))
    context, err := e.buildContext(docs, question)
    if err != nil {
        return nil, err
    }

    // 3. Process images (if enabled)
    if e.config.UseVision {
        images := e.extractImages(docs)
        if len(images) > 0 {
            descriptions, _ := e.processImages(images)
            context = e.includeImageContext(context, descriptions)
        }
    }

    // 4. Query LLM
    e.logger.Info("Querying LLM")
    prompt := e.buildPrompt(context, question)
    response, err := e.llm.Complete(ctx, prompt)
    if err != nil {
        return nil, err
    }

    // 5. Format answer
    answer := &Answer{
        Text:           response,
        Sources:        docs,
        ProcessingTime: time.Since(start),
    }

    return answer, nil
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

### Implement RAG Engine
```
/quaero-rag C:\development\quaero
```

### Configure RAG Parameters
```
/quaero-rag C:\development\quaero --max-tokens=4000 --min-score=0.7
```

## Validation

After implementation, verifies:
- ✓ RAG interface defined
- ✓ Search component working
- ✓ Context builder functional
- ✓ Vision processing implemented
- ✓ Engine orchestrates workflow
- ✓ Prompt templates created
- ✓ Source citation working
- ✓ Unit tests passing
- ✓ Integration tests passing
- ✓ Can answer test questions

## Output

Provides detailed report:
- Files created/modified
- RAG interface defined
- Search implementation status
- Context building logic
- Vision processing capability
- Prompt templates
- Tests created

---

**Agent**: quaero-rag

**Prompt**: Implement the RAG engine for Quaero at {{args.[0]}}.

## Implementation Tasks

1. **Define RAG Interface** (`pkg/models/rag.go`)
   - RAG interface with Query method
   - Answer struct with Text, Sources, ProcessingTime
   - Configuration options

2. **Implement Search** (`internal/rag/search.go`)
   - Hybrid search (full-text + vector)
   - Result ranking by relevance
   - Filtering and deduplication
   - Integration with Storage

3. **Build Context Builder** (`internal/rag/context.go`)
   - Select best chunks from documents
   - Format context for LLM
   - Token counting and window management
   - Source attribution

4. **Create Vision Processor** (`internal/rag/vision.go`)
   - Image processing with vision model
   - Description generation
   - OCR for diagrams
   - Integration with context

5. **Implement RAG Engine** (`internal/rag/engine.go`)
   - Orchestrate search, context, vision, LLM
   - Error handling and fallbacks
   - Performance tracking
   - Logging

6. **Create Prompt Templates** (`internal/rag/prompts.go`)
   - System prompts
   - Context formatting
   - Source citation instructions
   - Different templates for query types

7. **Create Tests**
   - Unit tests for each component
   - Integration test for full workflow
   - Mock storage and LLM
   - Test with sample questions

## Code Quality Standards

- Implements RAG interface
- Integrates Storage and LLM interfaces
- Structured logging (arbor)
- Comprehensive error handling
- Token counting and management
- Performance monitoring
- 80%+ test coverage

## Success Criteria

✓ RAG interface fully defined
✓ Search finds relevant documents
✓ Context built within token limit
✓ Vision processing functional
✓ Engine orchestrates workflow
✓ Answers include source citations
✓ All tests passing
✓ Can answer complex questions
