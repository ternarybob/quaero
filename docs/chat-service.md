# Chat Service Documentation

## Overview

The Chat Service provides RAG-enabled (Retrieval-Augmented Generation) conversational AI capabilities using the Quaero document database. It combines local LLM inference with semantic document retrieval to deliver context-aware responses.

**Key Features:**
- RAG-enabled chat with vectorized document retrieval
- Support for both offline (local) and online LLM modes
- Configurable search parameters (similarity threshold, max documents)
- Conversation history support
- Custom system prompts
- Health monitoring and status endpoints

## Architecture

### Components

```
┌─────────────────┐
│   HTTP Handler  │  (handlers/chat_handler.go)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Chat Service   │  (services/chat/chat_service.go)
└────────┬────────┘
         │
         ├──────────────────┬──────────────────┬──────────────────┐
         ▼                  ▼                  ▼                  ▼
┌──────────────┐   ┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│ LLM Service  │   │   Document   │   │  Embedding   │   │    Logger    │
│              │   │   Service    │   │   Service    │   │              │
└──────────────┘   └──────────────┘   └──────────────┘   └──────────────┘
```

### RAG Flow

1. **User sends message** → HTTP handler receives request
2. **Embedding generation** → Convert query to vector embedding
3. **Document retrieval** → Search vector database for similar documents
4. **Context augmentation** → Inject retrieved documents into prompt
5. **LLM inference** → Generate response with augmented context
6. **Response delivery** → Return response with metadata

### Data Flow

```
User Request (JSON)
    ↓
Parse & Validate
    ↓
Generate Query Embedding
    ↓
Search Vector Database (if RAG enabled)
    ↓
Build Context from Retrieved Documents
    ↓
Construct LLM Messages (system + history + user + context)
    ↓
Generate Response via LLM
    ↓
Return ChatResponse (JSON)
```

## API Endpoints

### POST /api/chat

Generate a chat response with optional RAG context retrieval.

**Request Body:**
```json
{
  "message": "What is the project architecture?",
  "history": [
    {"role": "user", "content": "Hello"},
    {"role": "assistant", "content": "Hi! How can I help?"}
  ],
  "system_prompt": "You are a helpful technical assistant.",
  "rag_config": {
    "enabled": true,
    "max_documents": 5,
    "min_similarity": 0.7,
    "source_types": ["jira", "confluence"],
    "search_mode": "vector"
  }
}
```

**Request Fields:**
- `message` (required): User's message
- `history` (optional): Conversation history
- `system_prompt` (optional): Custom system prompt
- `rag_config` (optional): RAG configuration (defaults to enabled)
  - `enabled`: Enable/disable document retrieval
  - `max_documents`: Maximum documents to retrieve (default: 5)
  - `min_similarity`: Minimum similarity score 0-1 (default: 0.7)
  - `source_types`: Filter by source types (jira, confluence, github)
  - `search_mode`: "vector", "hybrid", or "keyword"

**Response:**
```json
{
  "success": true,
  "message": "The project follows clean architecture with...",
  "context_docs": [
    {
      "id": "doc123",
      "title": "Architecture Overview",
      "content": "...",
      "source_type": "confluence",
      "similarity": 0.85
    }
  ],
  "token_usage": {
    "prompt_tokens": 450,
    "completion_tokens": 120,
    "total_tokens": 570
  },
  "model": "qwen2.5-7b-instruct-q4",
  "mode": "offline"
}
```

**Error Response:**
```json
{
  "success": false,
  "error": "Message field is required"
}
```

**Status Codes:**
- `200 OK`: Success
- `400 Bad Request`: Invalid request (empty message, malformed JSON)
- `405 Method Not Allowed`: Wrong HTTP method
- `500 Internal Server Error`: Server error during processing

### GET /api/chat/health

Check chat service health status.

**Response:**
```json
{
  "healthy": true,
  "mode": "offline"
}
```

**Error Response:**
```json
{
  "healthy": false,
  "mode": "offline",
  "error": "LLM service unhealthy: llama-cli binary not accessible"
}
```

**Status Codes:**
- `200 OK`: Service healthy
- `503 Service Unavailable`: Service unhealthy

## Configuration

The chat service uses the existing LLM configuration from `config.toml`:

```toml
[llm]
mode = "offline"  # "offline" or "online"

# Offline mode settings
[llm.offline]
llama_dir = "./llama.cpp"
model_dir = "./models"
embed_model = "nomic-embed-text-v1.5-q8.gguf"
chat_model = "qwen2.5-7b-instruct-q4.gguf"
context_size = 2048
thread_count = 4
gpu_layers = 0
mock_mode = false  # Set to true for testing without models

# Online mode settings (future)
[llm.online]
provider = "openai"
api_key = "${OPENAI_API_KEY}"
model = "gpt-4"
```

### Default RAG Configuration

If no `rag_config` is provided in the request, these defaults are used:

```go
RAGConfig{
    Enabled:       true,
    MaxDocuments:  5,
    MinSimilarity: 0.7,
    SearchMode:    "vector",
}
```

## Usage Examples

### Simple Chat (No RAG)

```bash
curl -X POST http://localhost:8086/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Hello, how are you?",
    "rag_config": {
      "enabled": false
    }
  }'
```

### Chat with RAG

```bash
curl -X POST http://localhost:8086/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "What are the recent Jira issues?",
    "rag_config": {
      "enabled": true,
      "max_documents": 10,
      "min_similarity": 0.6,
      "source_types": ["jira"]
    }
  }'
```

### Chat with Conversation History

```bash
curl -X POST http://localhost:8086/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "What about GitHub issues?",
    "history": [
      {"role": "user", "content": "What issues are open?"},
      {"role": "assistant", "content": "There are 15 open issues in Jira."}
    ],
    "rag_config": {
      "enabled": true,
      "source_types": ["github"]
    }
  }'
```

### Custom System Prompt

```bash
curl -X POST http://localhost:8086/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Write a function to parse JSON",
    "system_prompt": "You are an expert Go programmer. Provide clean, idiomatic code.",
    "rag_config": {
      "enabled": false
    }
  }'
```

### Health Check

```bash
curl http://localhost:8086/api/chat/health
```

## Testing

The chat service has comprehensive test coverage across unit and API integration tests.

### Unit Tests

Located in `test/unit/chat_service_test.go`:

```bash
./test/run-tests.ps1 -Type unit
```

**Tests:**
- Chat without RAG
- Chat with RAG
- Chat with conversation history
- Custom system prompts
- Health check (success and failure)
- Default RAG configuration

### API Integration Tests

Located in `test/api/chat_api_test.go`:

```bash
./test/run-tests.ps1 -Type api
```

**Tests:**
- Health endpoint (GET)
- Health endpoint method validation
- Successful chat request
- Chat with RAG enabled
- Chat with conversation history
- Empty message error handling
- Invalid JSON error handling
- Method not allowed error handling
- Custom system prompt
- Multiple sequential requests

### Running All Tests

```bash
./test/run-tests.ps1 -Type all
```

Test results are saved to timestamped directories:
```
test/results/
├── unit-2025-10-07_11-00-00/
├── api-2025-10-07_11-04-31/
└── all-2025-10-07_11-10-00/
```

## Implementation Details

### Service Interface

Defined in `internal/interfaces/chat_service.go`:

```go
type ChatService interface {
    Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    GetMode() LLMMode
    HealthCheck(ctx context.Context) error
}
```

### Service Implementation

Located in `internal/services/chat/chat_service.go`:

**Key Methods:**
- `Chat()`: Main entry point for chat requests
- `retrieveContext()`: Retrieves relevant documents via vector search
- `buildContextText()`: Formats documents for prompt injection
- `buildMessages()`: Constructs LLM message array
- `HealthCheck()`: Validates LLM service availability

**Dependencies:**
- `LLMService`: Handles chat completion and embeddings
- `DocumentService`: Provides document search functionality
- `EmbeddingService`: Generates query embeddings
- `Logger`: Structured logging

### HTTP Handler

Located in `internal/handlers/chat_handler.go`:

**Responsibilities:**
- Request parsing and validation
- Error handling and HTTP status codes
- Response formatting
- Logging HTTP requests

## Mock Mode

For testing without actual LLM models, enable mock mode:

```toml
[llm.offline]
mock_mode = true
```

**Mock Mode Behavior:**
- Returns deterministic fake responses
- Generates consistent embeddings based on text content
- Always passes health checks
- No llama-cli binary or model files required

**Mock Response Format:**
```
"Mock response to: [user's message]"
```

## Performance Considerations

### Context Size

The chat model's context window (default 2048 tokens) limits:
- Total conversation history
- Number of retrieved documents
- Combined input + output length

**Best Practices:**
- Limit `max_documents` to 3-5 for typical queries
- Trim conversation history to recent 10-20 messages
- Use higher `min_similarity` (0.75-0.85) for focused results

### Response Time

Typical response times (offline mode, CPU inference):
- Simple chat (no RAG): 2-5 seconds
- RAG-enabled chat: 3-7 seconds
- GPU acceleration: 0.5-2 seconds

### Vector Search

Vector search performance scales with:
- Database size (number of documents)
- Embedding dimensions (768 for nomic-embed)
- Search algorithm (cosine similarity)

## Future Enhancements

### Planned Features

1. **Streaming Responses**: Server-sent events for token streaming
2. **Conversation Management**: Save/load conversation histories
3. **Multi-modal Support**: Image and file attachments
4. **Advanced RAG**: Hybrid search, re-ranking, citation tracking
5. **Online Mode**: OpenAI, Anthropic, Cohere integration
6. **Rate Limiting**: Per-user request throttling
7. **Analytics**: Usage tracking, quality metrics
8. **Caching**: Response caching for common queries

## Troubleshooting

### Common Issues

**1. Health check failing**
```
{"healthy":false,"error":"llama-cli binary not accessible"}
```
**Solution**: Verify llama-cli binary exists at configured path

**2. Empty responses**
```
{"message":""}
```
**Solution**: Check model context size, reduce history/documents

**3. Vector search errors**
```
{"error":"failed to search documents: vector search not yet implemented"}
```
**Solution**: Ensure sqlite-vec extension is loaded (future enhancement)

**4. Slow responses**
```
Response time > 30 seconds
```
**Solution**: Enable GPU acceleration with `gpu_layers > 0`

### Debug Logging

Enable debug logging to troubleshoot issues:

```toml
[logging]
level = "debug"
```

**Key Log Messages:**
- `Processing chat request` - Request received
- `Generating embedding` - Query embedding creation
- `Chat request completed` - Response generated
- `Failed to retrieve context documents` - RAG error (warning)

## Security Considerations

### Data Privacy

- All processing occurs locally (offline mode)
- No external API calls in offline mode
- Documents never leave the server
- Conversation history not persisted by default

### Input Validation

- Message length limits enforced
- JSON schema validation
- SQL injection protection via parameterized queries
- Path traversal prevention

### Rate Limiting

Not currently implemented - recommend adding for production:
- Per-IP request limits
- Authentication/authorization
- Request size limits

## References

- **RAG Overview**: https://arxiv.org/abs/2005.11401
- **Llama.cpp**: https://github.com/ggerganov/llama.cpp
- **Nomic Embed**: https://huggingface.co/nomic-ai/nomic-embed-text-v1.5
- **Qwen 2.5**: https://huggingface.co/Qwen/Qwen2.5-7B-Instruct

## Support

For issues or questions:
- Check logs in `./logs/` directory
- Review test results in `./test/results/`
- Examine service initialization in `internal/app/app.go`
- Verify configuration in `bin/quaero.toml`
