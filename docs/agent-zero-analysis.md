# Agent Zero Research Analysis

**Date:** 2025-10-06
**Project:** Agent0 (https://github.com/agent0ai/agent-zero)
**Purpose:** Evaluate Agent0's architecture for potential adoption in Quaero
**Status:** Complete

---

## Executive Summary

Agent Zero is a dynamic, organic AI framework designed for general-purpose AI assistant tasks. It features a sophisticated memory system using FAISS vector database, flexible LLM integration via LiteLLM, and Docker-based deployment. This analysis examines key architectural patterns relevant to Quaero's knowledge base system.

**Key Findings:**
- Memory system uses FAISS (not SQLite) for vector storage
- Supports multiple LLM providers through LiteLLM abstraction
- Docker-required architecture (not designed for local deployment)
- Polling-based UI updates (not WebSocket streaming)
- Extension-based architecture for customization
- Python-based (vs Quaero's Go implementation)

---

## 1. Memory System Architecture

### Storage Technology

**Vector Database:** FAISS (Facebook AI Similarity Search)
- CPU-based implementation: `faiss-cpu==1.11.0`
- Not SQLite-based like Quaero
- In-memory and file-based storage modes

**Implementation:** `python/helpers/memory.py`

```python
class MyFaiss(FAISS):
    """Extended FAISS vector store with custom retrieval"""

    def __init__(self, embedding_function, index, ...):
        # Extends LangChain's FAISS implementation
        super().__init__(embedding_function, index, ...)
```

**Storage Areas:**
Agent0 organizes memories into categories:
```python
class Area(Enum):
    MAIN = "main"              # Primary memories
    FRAGMENTS = "fragments"    # Conversation fragments
    SOLUTIONS = "solutions"    # Solution repository
    INSTRUMENTS = "instruments" # Custom functions
```

### Embedding Generation

**Technology Stack:**
- LangChain Core: `langchain-core==0.3.49`
- Sentence Transformers: `sentence-transformers==3.0.1`
- Cached embeddings for performance

**Configuration:**
```python
# Configurable embedding models
embedding_model = models.get_embedding_model()
cache_backed = CacheBackedEmbeddings.from_bytes_store(
    embedding_model,
    cache_store,
    namespace=embedding_model.model_name
)
```

**Supports:**
- Local embedding models
- API-based embedding services
- Multiple embedding dimensions

### Memory Operations

**Insertion:**
```python
async def insert_text(text: str, metadata: dict) -> str:
    """Insert text with automatic embedding and metadata"""
    metadata = {"area": area, **kwargs}
    db = await Memory.get(self.agent)
    id = await db.insert_text(text, metadata)
```

**Retrieval:**
```python
async def search_similarity_threshold(
    query: str,
    limit: int = 10,
    threshold: float = 0.7,
    filter: dict = None
) -> List[Document]:
    """Semantic search with similarity cutoff"""
    # Uses cosine similarity normalized to 0-1
    # Returns only documents above threshold
```

**Key Features:**
- Automatic ID generation (UUID-based)
- Timestamp tracking
- Metadata filtering
- Similarity threshold filtering
- Semantic search using vector similarity

### Knowledge Base Management

**Preloading:**
```python
async def preload_knowledge(directory_path: str):
    """Load knowledge base from files"""
    # Supports multiple document formats
    # via unstructured library
```

**Document Processing:**
- Uses `unstructured[all-docs]==0.16.23` for document parsing
- Supports: PDF, DOCX, HTML, Markdown, etc.
- Automatic chunking for large documents
- Metadata extraction from file properties

---

## 2. LLM Integration Architecture

### Multi-Provider Support via LiteLLM

**Core Library:** `litellm==1.75.0`

**Implementation:** `models.py`

```python
class ModelConfig:
    """Unified model configuration"""
    provider: str           # ollama, openai, anthropic, etc.
    model_name: str
    ctx_length: int
    limit_requests: int
    limit_input: int
    limit_output: int
    vision: bool

def get_chat_model(config: ModelConfig):
    """Create chat model with provider-specific settings"""
    # Supports streaming and non-streaming
    # Handles rate limiting
    # Provider-specific parameter normalization
```

### Supported Providers

**Explicitly Documented:**
- Ollama (local models)
- OpenAI
- GitHub Copilot
- Azure OpenAI
- Anthropic
- Google (Gemini)
- Groq

**Configuration Pattern:**
```python
# Models.py uses environment variables or config
# LiteLLM handles provider authentication
# Unified API across all providers
```

### Model Roles

Agent0 uses multiple models with distinct purposes:
```python
# From agent configuration
chat_llm: ModelConfig          # Primary conversation
utility_llm: ModelConfig        # Internal tasks
embedding_llm: ModelConfig      # Memory/retrieval
browser_llm: ModelConfig        # Web browsing tasks
```

### Ollama Integration

**No Docker Required for Ollama:**
- Ollama runs as separate service
- Agent0 Docker container connects via HTTP
- Default URL: `http://localhost:11434`

**Model Management:**
```bash
# Installation (from docs/installation.md)
# Windows: Official installer
# macOS: brew install ollama
# Linux: curl -fsSL https://ollama.com/install.sh | sh

# Usage
ollama pull model-name
ollama list
ollama rm model-name
```

**Agent0 Configuration:**
```python
# Agent connects to Ollama API
# No special Docker setup needed for Ollama
# Works with local Ollama installation
```

### LLM Call Pattern

**Implementation:** `agent.py`

```python
async def call_chat_model(
    agent_context: AgentContext,
    system_prompt: str,
    user_message: str,
    stream: bool = True
):
    """Unified LLM calling with streaming support"""
    # Supports streaming callbacks
    # Handles reasoning and response separately
    # Provider-agnostic interface
```

**Streaming Support:**
```python
# Real-time streaming with callbacks
callbacks=[
    StreamingStdOutCallbackHandler(),  # Console output
    ReasoningCallback(agent_context),   # Reasoning tracking
    ResponseCallback(agent_context)     # Response tracking
]
```

---

## 3. User Interaction

### Web UI Architecture

**Framework:** Flask 3.0.3 with async support
```python
from flask import Flask
from flask[async] import ...

app = Flask(__name__)
# Session-based authentication
# CSRF protection
# Loopback-only by default
```

**Entry Point:** `run_ui.py`
- Initializes Flask app
- Registers API routes dynamically
- Starts job loop for background tasks
- Serves static files and templates

### Real-Time Communication

**NO WebSocket Implementation**

Instead, Agent0 uses **polling**:

**Implementation:** `python/api/poll.py`

```python
class Poll:
    async def process(self):
        """Called periodically by frontend"""
        return {
            "contexts": self.get_active_contexts(),
            "tasks": scheduler.get_tasks(),
            "logs": self.get_recent_logs(),
            "notifications": notification_manager.get_all(),
            "version_info": {
                "log_guid": current_log_version,
                "notification_guid": current_notification_version
            }
        }
```

**Frontend Pattern:**
```javascript
// Periodic polling (every 1-2 seconds)
setInterval(async () => {
    const state = await fetch('/api/poll');
    updateUI(state);
}, 1000);
```

**Advantages:**
- Simpler implementation
- No persistent connection management
- Works through firewalls/proxies easily

**Disadvantages:**
- Higher latency for updates
- More network overhead
- Not true real-time

### Notification System

**Implementation:** Dedicated notification manager
```python
# python/api/notification_create.py
# python/api/notifications_clear.py
# python/api/notifications_history.py
# python/api/notifications_mark_read.py
```

**Features:**
- Persistent notification history
- Read/unread tracking
- Priority levels
- Dismissible notifications

### Log Streaming

**Pattern:** Memory-based log collection
```python
# Logs stored in context objects
# Retrieved via polling endpoint
# Frontend displays in real-time via polling
```

**NOT streaming like Quaero's WebSocket approach**

---

## 4. Deployment Model

### Docker-Centric Architecture

**REQUIRED:** Docker Desktop

**Installation:**
```bash
# Quick Start
docker pull agent0ai/agent-zero
docker run -p 50001:80 agent0ai/agent-zero
# Visit http://localhost:50001
```

**Dockerfile Structure:** `DockerfileLocal`

```dockerfile
FROM agent-zero-base:latest

# Multi-stage installation
RUN /exe/pre-install.sh
RUN /exe/install-agent-zero.sh
RUN /exe/install-software.sh
RUN /exe/post-install.sh

# Expose ports
EXPOSE 22 80 9000-9009

CMD ["/exe/initialize.sh", "local"]
```

**Why Docker Required:**
- Isolated execution environment (agents can run code)
- Dependency management (Python + system packages)
- Security sandboxing
- Cross-platform consistency

### Local Deployment Options

**NO standalone deployment without Docker**

**Dependencies from `requirements.txt`:**
```
faiss-cpu==1.11.0
langchain-core==0.3.49
langchain-community==0.3.19
litellm==1.75.0
sentence-transformers==3.0.1
flask[async]==3.0.3
unstructured[all-docs]==0.16.23
docker==7.1.0           # Docker SDK for container management
tiktoken==0.8.0
newspaper3k==0.2.8
```

**System Requirements:**
- Docker Desktop (8GB+ RAM recommended)
- Modern CPU
- ~5GB disk space for container
- Ollama (optional, for local LLM)

### Complexity Assessment

**Setup Complexity:** Medium-High
- Requires Docker installation
- Container configuration
- Volume mapping for persistence
- API key management

**Runtime Complexity:** Medium
- Docker resource management
- Container updates
- Backup/restore procedures
- Network configuration

---

## 5. Key Design Patterns

### Agent Architecture

**Hierarchical Agents:**
```python
class Agent:
    def __init__(self, number, context, superior=None):
        self.number = number
        self.context = context
        self.superior = superior  # Parent agent
        self.subordinates = []    # Child agents
```

**Agent Creation Pattern:**
```python
# Main agent can spawn subordinates for complex tasks
subordinate = Agent(
    number=self.next_number(),
    context=new_context,
    superior=self
)
subordinate.execute_task(subtask)
```

### Tool System

**Base Tool Pattern:**
```python
class Tool:
    """Base class for all tools"""

    async def execute(self, **kwargs):
        """Tool execution logic"""
        pass

    def get_schema(self):
        """Tool description for LLM"""
        pass
```

**Dynamic Tool Loading:**
```python
# Tools loaded from directories
# Profile-specific overrides
# Default tools always available

default_tools = load_tools("python/tools/")
profile_tools = load_tools(f"profiles/{profile}/tools/")
all_tools = {**default_tools, **profile_tools}
```

**Available Tools:**
- `memory_save.py` - Save memories
- `memory_load.py` - Retrieve memories
- `memory_delete.py` - Remove memories
- `document_query.py` - Query documents
- `search_engine.py` - Web search
- `call_subordinate.py` - Create sub-agents

### Extension System

**Extension Points:**
```python
class Extension:
    """Hook into agent lifecycle"""

    async def execute(self, extension_point, **kwargs):
        """Called at specific lifecycle points"""
        pass

# Extension points:
# - agent_init
# - before_main_llm_call
# - after_main_llm_call
# - message_loop_start
# - message_loop_end
# - monologue_start
# - monologue_end
```

**Example Extension:**
```python
class ExampleExtension(Extension):
    async def execute(self, **kwargs):
        if kwargs.get('extension_point') == 'agent_init':
            self.agent.agent_name = "SuperAgent" + str(self.agent.number)
```

### Context Management

**Agent Context Pattern:**
```python
class AgentContext:
    """Encapsulates agent execution state"""

    def __init__(self):
        self.id = generate_id()
        self.messages = []
        self.memory = Memory.get(self)
        self.tools = load_tools()
        self.history = ConversationHistory()
```

**Context Window Management:**
```python
class LoopData:
    """Tracks conversation loop state"""

    iteration: int
    messages: List[Message]
    extras_persistent: dict  # Carried across loops
    extras_temporary: dict   # Reset each loop
```

### Prompt Engineering

**Prompt Structure:**
```
prompts/
├── default/
│   ├── agent.system.md      # System prompt
│   ├── tool.*.md            # Tool descriptions
│   └── fw.*.md              # Framework messages
└── {profile}/
    └── (overrides)
```

**Dynamic Prompt Loading:**
```python
def load_prompt(name, profile=None):
    """Load prompt with profile override"""
    # Check profile-specific first
    if profile and exists(f"prompts/{profile}/{name}"):
        return read_file(f"prompts/{profile}/{name}")

    # Fall back to default
    return read_file(f"prompts/default/{name}")
```

**Variable Substitution:**
```markdown
<!-- In prompt files -->
{{agent.name}}
{{context.id}}
{{tools.list}}

<!-- Expanded at runtime -->
```

---

## 6. Comparison with Quaero

### Architecture Comparison

| Aspect | Agent0 | Quaero |
|--------|---------|---------|
| **Language** | Python | Go |
| **Vector DB** | FAISS (in-memory/file) | SQLite with embeddings |
| **Storage** | File-based FAISS index | SQLite database |
| **LLM** | LiteLLM (multi-provider) | Ollama only |
| **Deployment** | Docker required | Standalone binary |
| **UI Updates** | HTTP polling | WebSocket streaming |
| **Purpose** | General AI assistant | Knowledge base search |
| **Scope** | Broad (code execution, tools) | Focused (collect + search) |

### Memory System Comparison

| Feature | Agent0 | Quaero |
|---------|---------|---------|
| **Vector Store** | FAISS | SQLite BLOB |
| **Full-Text** | Not integrated | FTS5 built-in |
| **Persistence** | File-based index | Database transactions |
| **Scalability** | FAISS optimized for millions | SQLite good to 100k+ |
| **Query Types** | Similarity only | Keyword + vector (future) |
| **Metadata** | Stored with embeddings | Relational columns |

**Agent0 Advantages:**
- FAISS highly optimized for vector search
- Approximate nearest neighbor (ANN) algorithms
- Better for large-scale similarity search

**Quaero Advantages:**
- Single database file (SQLite)
- Integrated full-text + vector
- ACID transactions
- Simpler deployment (no separate index files)

### LLM Integration Comparison

| Feature | Agent0 | Quaero |
|---------|---------|---------|
| **Provider Abstraction** | LiteLLM (unified API) | Direct Ollama API |
| **Supported Providers** | 10+ (OpenAI, Anthropic, etc.) | Ollama only |
| **Streaming** | Yes (via callbacks) | Planned |
| **Model Switching** | Runtime config | Static config |
| **Rate Limiting** | Built-in | Manual |

**Agent0 Advantages:**
- Easy provider switching
- Unified API across providers
- Production-ready rate limiting

**Quaero Current State:**
- Simpler (single provider)
- No API key management
- Fully local

### Deployment Comparison

| Aspect | Agent0 | Quaero |
|---------|---------|---------|
| **Runtime** | Docker container | Native binary |
| **Setup** | Docker pull + run | Download + run |
| **Dependencies** | Container-managed | Go binary (none) |
| **Updates** | Docker image pull | Binary replacement |
| **Resource** | Container overhead | Direct execution |
| **Security** | Container isolation | OS-level only |

**Agent0 Trade-offs:**
- More complex setup
- Better isolation
- Cross-platform consistency
- Resource overhead

**Quaero Trade-offs:**
- Simpler setup
- Direct execution
- Platform-specific builds
- Lower overhead

### UI/UX Comparison

| Feature | Agent0 | Quaero |
|---------|---------|---------|
| **Framework** | Flask + polling | Native Go + WebSocket |
| **Update Method** | HTTP polling (1s) | WebSocket push |
| **Latency** | 500-1000ms | <100ms |
| **Overhead** | Constant polling | Event-driven |
| **Simplicity** | Simple (REST only) | Complex (WS + HTTP) |

**Agent0 Pattern:**
```javascript
// Polling every 1 second
setInterval(() => fetch('/api/poll'), 1000);
```

**Quaero Pattern:**
```javascript
// WebSocket push
ws.onmessage = (event) => updateUI(event.data);
```

---

## 7. Recommendations for Quaero

### Patterns to Adopt

#### 1. Memory Area Organization

**Agent0 Pattern:**
```python
class Area(Enum):
    MAIN = "main"
    FRAGMENTS = "fragments"
    SOLUTIONS = "solutions"
    INSTRUMENTS = "instruments"
```

**Quaero Adaptation:**
```go
// Add memory area categorization
type MemoryArea string

const (
    MemoryAreaMain       MemoryArea = "main"        // Primary knowledge
    MemoryAreaFragments  MemoryArea = "fragments"   // Conversation history
    MemoryAreaSolutions  MemoryArea = "solutions"   // Solved queries
    MemoryAreaFacts      MemoryArea = "facts"       // Extracted facts
)

// Extend Document model
type Document struct {
    // ... existing fields
    MemoryArea MemoryArea `json:"memory_area"`
}

// Filtering in search
func (s *DocumentService) SearchByArea(
    ctx context.Context,
    area MemoryArea,
    query string,
) ([]*models.Document, error)
```

**Benefits:**
- Better organization of different knowledge types
- Targeted retrieval based on use case
- Cleaner separation of concerns

#### 2. Similarity Threshold Filtering

**Agent0 Pattern:**
```python
async def search_similarity_threshold(
    query: str,
    limit: int = 10,
    threshold: float = 0.7,  # Only return scores > 0.7
    filter: dict = None
) -> List[Document]:
    # Filter results by minimum similarity
```

**Quaero Adaptation:**
```go
type SearchOptions struct {
    Query              string
    Limit              int
    SimilarityThreshold float32  // NEW: 0.0-1.0
    MinScore           float32   // For FTS5 keyword search
}

func (s *DocumentStorage) VectorSearch(
    ctx context.Context,
    embedding []float32,
    opts *SearchOptions,
) ([]*models.Document, error) {
    // Compute similarities
    results := computeCosineSimilarity(embedding, storedEmbeddings)

    // Filter by threshold
    filtered := []Result{}
    for _, r := range results {
        if r.Score >= opts.SimilarityThreshold {
            filtered = append(filtered, r)
        }
    }

    return filtered
}
```

**Benefits:**
- Reduces noise in search results
- Configurable precision/recall trade-off
- Better UX (only relevant results)

#### 3. Embedding Caching

**Agent0 Pattern:**
```python
cache_backed = CacheBackedEmbeddings.from_bytes_store(
    embedding_model,
    cache_store,
    namespace=embedding_model.model_name
)
```

**Quaero Adaptation:**
```go
type EmbeddingService struct {
    client   *ollama.Client
    cache    *EmbeddingCache  // NEW
    logger   arbor.ILogger
}

type EmbeddingCache struct {
    mu    sync.RWMutex
    cache map[string][]float32  // text hash -> embedding
    ttl   time.Duration
}

func (s *EmbeddingService) GenerateEmbedding(
    ctx context.Context,
    text string,
) ([]float32, error) {
    hash := computeHash(text)

    // Check cache first
    if embedding := s.cache.Get(hash); embedding != nil {
        s.logger.Debug("Embedding cache hit", "hash", hash)
        return embedding, nil
    }

    // Generate and cache
    embedding, err := s.client.Embed(ctx, text)
    if err == nil {
        s.cache.Set(hash, embedding)
    }

    return embedding, err
}
```

**Benefits:**
- Avoid redundant embedding generation
- Faster for duplicate content
- Reduced Ollama load

#### 4. Batch Embedding Support

**Agent0 Pattern:**
```python
# LangChain supports batch embeddings
embeddings = embedding_model.embed_documents(texts)
```

**Quaero Adaptation:**
```go
func (s *EmbeddingService) EmbedDocuments(
    ctx context.Context,
    docs []*models.Document,
) error {
    // Batch request to Ollama
    texts := make([]string, len(docs))
    for i, doc := range docs {
        texts[i] = doc.Title + "\n\n" + doc.Content
    }

    // Single API call for multiple embeddings
    embeddings, err := s.client.EmbedBatch(ctx, texts)
    if err != nil {
        return err
    }

    // Assign to documents
    for i, embedding := range embeddings {
        docs[i].Embedding = embedding
        docs[i].EmbeddingModel = s.modelName
    }

    return nil
}
```

**Benefits:**
- Reduce API round-trips
- Better throughput
- Lower latency for batch processing

#### 5. Tool/Extension Pattern (for RAG)

**Agent0 Pattern:**
```python
class Tool:
    async def execute(self, **kwargs):
        pass

    def get_schema(self):
        pass

# Tools auto-discovered and loaded
tools = load_tools("python/tools/")
```

**Quaero Adaptation:**
```go
// For RAG pipeline
type RAGTool interface {
    Name() string
    Description() string
    Execute(ctx context.Context, input map[string]interface{}) (interface{}, error)
}

type SearchTool struct {
    documentService interfaces.DocumentService
}

func (t *SearchTool) Execute(
    ctx context.Context,
    input map[string]interface{},
) (interface{}, error) {
    query := input["query"].(string)
    return t.documentService.Search(ctx, &SearchQuery{
        Query: query,
        Mode:  "hybrid",
    })
}

// RAG orchestrator uses tools
type RAGService struct {
    tools map[string]RAGTool
    llm   *OllamaClient
}

func (s *RAGService) Answer(ctx context.Context, question string) (string, error) {
    // Use search tool to find context
    searchResult := s.tools["search"].Execute(ctx, map[string]interface{}{
        "query": question,
    })

    // Build prompt with context
    prompt := buildPromptWithContext(question, searchResult)

    // Generate answer with LLM
    return s.llm.Generate(ctx, prompt)
}
```

**Benefits:**
- Modular RAG components
- Easy to add new tools
- Clear separation of concerns

### Patterns to Avoid

#### 1. Docker Requirement

**Why Avoid:**
- Quaero goal is simple local deployment
- Go binary is self-contained
- No isolation needed (read-only operations)

**Quaero Alternative:**
- Single native binary
- No container overhead
- Direct execution

#### 2. Polling for UI Updates

**Why Avoid:**
- Quaero already has WebSocket
- Real-time streaming is better UX
- More efficient than polling

**Quaero Current:**
```go
// Keep WebSocket streaming
wsHandler.Broadcast(LogMessage{
    Level:   "info",
    Message: "Processing complete",
    Time:    time.Now(),
})
```

#### 3. FAISS for Small-Scale Vector Search

**Why Avoid:**
- SQLite sufficient for Quaero's scale
- FAISS adds dependency complexity
- File-based index separate from data

**Quaero Alternative:**
- SQLite with embeddings in BLOB
- Future: sqlite-vec extension
- Single database file

#### 4. Multi-Provider LLM Abstraction (for now)

**Why Avoid:**
- Quaero focused on local-first
- Ollama sufficient for current needs
- Avoid API key management complexity

**Future Consideration:**
- Add LiteLLM later if needed
- Start simple with Ollama only

---

## 8. Actionable Next Steps for Quaero

### Immediate (Phase 1.2 - RAG)

1. **Implement Memory Areas**
   ```go
   // Add to Document model
   MemoryArea MemoryArea `json:"memory_area"`

   // Add filtering in storage layer
   func (s *DocumentStorage) ListByArea(area MemoryArea) ([]*Document, error)
   ```

2. **Add Similarity Threshold**
   ```go
   type SearchOptions struct {
       // ... existing fields
       SimilarityThreshold float32  // 0.0-1.0
   }
   ```

3. **Implement Embedding Cache**
   ```go
   type EmbeddingCache struct {
       cache map[string][]float32
       mu    sync.RWMutex
   }

   // Add to EmbeddingService
   cache *EmbeddingCache
   ```

4. **Create RAG Tool Interface**
   ```go
   type RAGTool interface {
       Name() string
       Execute(ctx context.Context, input any) (any, error)
   }
   ```

### Short-Term (Phase 2.0)

5. **Batch Embedding Support**
   - Check if Ollama supports batch embeddings
   - Implement EmbedBatch() method
   - Use in ProcessingService

6. **Metadata Filtering**
   ```go
   type SearchOptions struct {
       // ... existing fields
       MetadataFilters map[string]interface{}
   }
   ```

7. **Document Chunking**
   - Implement for large documents
   - Store in document_chunks table
   - Retrieve relevant chunks for RAG

### Medium-Term (Phase 3.0)

8. **sqlite-vec Integration**
   - Replace manual vector search
   - Use native SQL vector operations
   - Benchmark performance

9. **Hybrid Search**
   - Combine FTS5 + vector
   - Weighted ranking
   - Configurable weights

10. **Query Expansion**
    - Use LLM to expand query
    - Generate synonyms
    - Improve recall

---

## 9. Code Examples for Quaero

### Example 1: Memory Area Implementation

```go
// internal/models/document.go
type MemoryArea string

const (
    MemoryAreaMain       MemoryArea = "main"
    MemoryAreaFragments  MemoryArea = "fragments"
    MemoryAreaSolutions  MemoryArea = "solutions"
    MemoryAreaFacts      MemoryArea = "facts"
)

type Document struct {
    // ... existing fields
    MemoryArea MemoryArea `json:"memory_area"`
}

// internal/storage/sqlite/document_storage.go
func (s *DocumentStorage) SaveDocument(
    ctx context.Context,
    doc *models.Document,
) error {
    // Set default memory area if not specified
    if doc.MemoryArea == "" {
        doc.MemoryArea = models.MemoryAreaMain
    }

    _, err := s.db.ExecContext(ctx, `
        INSERT INTO documents (
            id, source_type, source_id, title, content,
            embedding, embedding_model, memory_area, ...
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ...)
    `, doc.ID, doc.SourceType, doc.SourceID, doc.Title,
       doc.Content, serializeEmbedding(doc.Embedding),
       doc.EmbeddingModel, doc.MemoryArea, ...)

    return err
}

func (s *DocumentStorage) SearchByArea(
    ctx context.Context,
    area models.MemoryArea,
    opts *SearchOptions,
) ([]*models.Document, error) {
    query := `
        SELECT * FROM documents
        WHERE memory_area = ?
        AND (title LIKE ? OR content LIKE ?)
        ORDER BY updated_at DESC
        LIMIT ?
    `

    rows, err := s.db.QueryContext(ctx, query,
        area, "%"+opts.Query+"%", "%"+opts.Query+"%", opts.Limit)
    // ... parse and return
}
```

### Example 2: Embedding Cache

```go
// internal/services/embeddings/cache.go
type EmbeddingCache struct {
    mu      sync.RWMutex
    cache   map[string]CachedEmbedding
    maxSize int
    ttl     time.Duration
}

type CachedEmbedding struct {
    Embedding []float32
    CreatedAt time.Time
}

func NewEmbeddingCache(maxSize int, ttl time.Duration) *EmbeddingCache {
    return &EmbeddingCache{
        cache:   make(map[string]CachedEmbedding),
        maxSize: maxSize,
        ttl:     ttl,
    }
}

func (c *EmbeddingCache) Get(text string) []float32 {
    c.mu.RLock()
    defer c.mu.RUnlock()

    key := computeHash(text)
    cached, exists := c.cache[key]

    if !exists {
        return nil
    }

    // Check TTL
    if time.Since(cached.CreatedAt) > c.ttl {
        return nil
    }

    return cached.Embedding
}

func (c *EmbeddingCache) Set(text string, embedding []float32) {
    c.mu.Lock()
    defer c.mu.Unlock()

    key := computeHash(text)

    // Evict oldest if at capacity
    if len(c.cache) >= c.maxSize {
        c.evictOldest()
    }

    c.cache[key] = CachedEmbedding{
        Embedding: embedding,
        CreatedAt: time.Now(),
    }
}

func computeHash(text string) string {
    h := sha256.Sum256([]byte(text))
    return hex.EncodeToString(h[:])
}

// internal/services/embeddings/embedding_service.go
type Service struct {
    client    *OllamaClient
    cache     *EmbeddingCache
    modelName string
    dimension int
    logger    arbor.ILogger
}

func NewService(
    ollamaURL string,
    modelName string,
    dimension int,
    logger arbor.ILogger,
) *Service {
    return &Service{
        client:    NewOllamaClient(ollamaURL),
        cache:     NewEmbeddingCache(1000, 24*time.Hour),
        modelName: modelName,
        dimension: dimension,
        logger:    logger,
    }
}

func (s *Service) GenerateEmbedding(
    ctx context.Context,
    text string,
) ([]float32, error) {
    // Check cache first
    if cached := s.cache.Get(text); cached != nil {
        s.logger.Debug("Embedding cache hit")
        return cached, nil
    }

    // Generate via Ollama
    s.logger.Debug("Generating embedding", "model", s.modelName)
    embedding, err := s.client.Embed(ctx, s.modelName, text)
    if err != nil {
        return nil, fmt.Errorf("failed to generate embedding: %w", err)
    }

    // Cache result
    s.cache.Set(text, embedding)

    return embedding, nil
}
```

### Example 3: Similarity Threshold Search

```go
// internal/storage/sqlite/document_storage.go
type SearchOptions struct {
    Query               string
    Limit               int
    Mode                SearchMode  // keyword, vector, hybrid
    SimilarityThreshold float32     // NEW: 0.0-1.0
    MinScore            float32     // For FTS5
}

func (s *DocumentStorage) VectorSearch(
    ctx context.Context,
    queryEmbedding []float32,
    opts *SearchOptions,
) ([]*models.Document, error) {
    // Get all documents with embeddings
    rows, err := s.db.QueryContext(ctx, `
        SELECT id, source_type, source_id, title, content,
               embedding, embedding_model, metadata, url,
               created_at, updated_at
        FROM documents
        WHERE embedding IS NOT NULL
    `)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    type ScoredDocument struct {
        Doc   *models.Document
        Score float32
    }

    scored := []ScoredDocument{}

    for rows.Next() {
        doc := &models.Document{}
        var embeddingBytes []byte

        err := rows.Scan(
            &doc.ID, &doc.SourceType, &doc.SourceID,
            &doc.Title, &doc.Content, &embeddingBytes,
            &doc.EmbeddingModel, &doc.Metadata, &doc.URL,
            &doc.CreatedAt, &doc.UpdatedAt,
        )
        if err != nil {
            continue
        }

        // Deserialize embedding
        doc.Embedding = deserializeEmbedding(embeddingBytes)

        // Compute cosine similarity
        similarity := cosineSimilarity(queryEmbedding, doc.Embedding)

        // Filter by threshold
        if similarity >= opts.SimilarityThreshold {
            scored = append(scored, ScoredDocument{
                Doc:   doc,
                Score: similarity,
            })
        }
    }

    // Sort by score descending
    sort.Slice(scored, func(i, j int) bool {
        return scored[i].Score > scored[j].Score
    })

    // Limit results
    if opts.Limit > 0 && len(scored) > opts.Limit {
        scored = scored[:opts.Limit]
    }

    // Extract documents
    results := make([]*models.Document, len(scored))
    for i, s := range scored {
        results[i] = s.Doc
        // Store score in metadata for display
        results[i].Metadata["similarity_score"] = s.Score
    }

    return results, nil
}

func cosineSimilarity(a, b []float32) float32 {
    if len(a) != len(b) {
        return 0
    }

    var dotProduct, normA, normB float32
    for i := range a {
        dotProduct += a[i] * b[i]
        normA += a[i] * a[i]
        normB += b[i] * b[i]
    }

    if normA == 0 || normB == 0 {
        return 0
    }

    return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}
```

### Example 4: RAG Tool Interface

```go
// internal/interfaces/rag_tool.go
type RAGTool interface {
    Name() string
    Description() string
    Execute(ctx context.Context, input map[string]interface{}) (interface{}, error)
}

// internal/services/rag/search_tool.go
type SearchTool struct {
    documentService interfaces.DocumentService
    logger          arbor.ILogger
}

func NewSearchTool(
    documentService interfaces.DocumentService,
    logger arbor.ILogger,
) *SearchTool {
    return &SearchTool{
        documentService: documentService,
        logger:          logger,
    }
}

func (t *SearchTool) Name() string {
    return "search"
}

func (t *SearchTool) Description() string {
    return "Search the knowledge base for relevant documents"
}

func (t *SearchTool) Execute(
    ctx context.Context,
    input map[string]interface{},
) (interface{}, error) {
    query, ok := input["query"].(string)
    if !ok {
        return nil, fmt.Errorf("query parameter required")
    }

    limit := 5
    if l, ok := input["limit"].(int); ok {
        limit = l
    }

    threshold := float32(0.7)
    if th, ok := input["threshold"].(float32); ok {
        threshold = th
    }

    t.logger.Debug("Executing search tool",
        "query", query,
        "limit", limit,
        "threshold", threshold)

    results, err := t.documentService.Search(ctx, &SearchQuery{
        Query:               query,
        Limit:               limit,
        Mode:                "hybrid",
        SimilarityThreshold: threshold,
    })

    if err != nil {
        return nil, fmt.Errorf("search failed: %w", err)
    }

    return results, nil
}

// internal/services/rag/rag_service.go
type RAGService struct {
    tools  map[string]RAGTool
    llm    *OllamaClient
    logger arbor.ILogger
}

func NewRAGService(
    documentService interfaces.DocumentService,
    llmURL string,
    logger arbor.ILogger,
) *RAGService {
    tools := map[string]RAGTool{
        "search": NewSearchTool(documentService, logger),
    }

    return &RAGService{
        tools:  tools,
        llm:    NewOllamaClient(llmURL),
        logger: logger,
    }
}

func (s *RAGService) Answer(
    ctx context.Context,
    question string,
) (string, error) {
    // Step 1: Use search tool to find context
    s.logger.Info("Searching for context", "question", question)

    searchResult, err := s.tools["search"].Execute(ctx, map[string]interface{}{
        "query":     question,
        "limit":     5,
        "threshold": 0.7,
    })
    if err != nil {
        return "", fmt.Errorf("search failed: %w", err)
    }

    docs := searchResult.([]*models.Document)

    // Step 2: Build context from results
    context := buildContext(docs)

    // Step 3: Build prompt
    prompt := fmt.Sprintf(`You are a helpful assistant answering questions based on a knowledge base.

Context:
%s

Question: %s

Answer the question based ONLY on the context provided above. If the context doesn't contain enough information, say so.

Answer:`, context, question)

    // Step 4: Generate answer with LLM
    s.logger.Info("Generating answer with LLM")

    answer, err := s.llm.Generate(ctx, prompt)
    if err != nil {
        return "", fmt.Errorf("LLM generation failed: %w", err)
    }

    return answer, nil
}

func buildContext(docs []*models.Document) string {
    var sb strings.Builder

    for i, doc := range docs {
        sb.WriteString(fmt.Sprintf("Document %d:\n", i+1))
        sb.WriteString(fmt.Sprintf("Title: %s\n", doc.Title))
        sb.WriteString(fmt.Sprintf("Source: %s\n", doc.URL))
        sb.WriteString(fmt.Sprintf("Content: %s\n\n", doc.Content))
    }

    return sb.String()
}
```

---

## 10. Conclusion

### Key Takeaways

**Agent0 Strengths:**
- Mature memory system with FAISS
- Flexible LLM integration via LiteLLM
- Extensible tool/extension architecture
- Production-ready patterns

**Quaero Advantages:**
- Simpler deployment (native binary)
- Single database file (SQLite)
- Real-time UI updates (WebSocket)
- Go performance and concurrency

### Recommended Adoptions

**High Priority:**
1. Memory area categorization
2. Similarity threshold filtering
3. Embedding caching
4. Tool interface for RAG

**Medium Priority:**
5. Batch embedding support
6. Metadata filtering
7. Document chunking

**Low Priority:**
8. Multi-provider LLM (future)
9. Extension system (if needed)

### What NOT to Adopt

- Docker deployment
- FAISS vector database
- HTTP polling for UI
- Python ecosystem

### Final Recommendation

Quaero should **selectively adopt** Agent0 patterns that improve the knowledge base and RAG capabilities while maintaining its core strengths: simplicity, local-first architecture, and Go performance.

**Focus on:**
- Better memory organization
- Smarter search filtering
- Efficient caching
- Modular RAG components

**Avoid:**
- Architectural complexity
- Deployment complexity
- Unnecessary abstractions

---

**Document Version:** 1.0
**Last Updated:** 2025-10-06
**Status:** Complete
