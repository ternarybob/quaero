# Offline LLM Service

**SECURITY CRITICAL**: This service provides 100% local LLM operations with NO network calls.

## Architecture

The offline LLM service uses binary execution of `llama-cli` (from llama.cpp) instead of CGo bindings. This provides:

- **No CGo dependencies** - Simpler builds and cross-platform support
- **Better process isolation** - Clear security boundary
- **No network capability** - Guaranteed offline operation
- **Easy testing** - Mock mode for testing without binary

## Components

### ModelManager (`models.go`)

Manages model file verification and path resolution.

**Functions:**
- `NewModelManager()` - Create model manager
- `VerifyModels()` - Check model files exist and are readable
- `GetEmbedModelPath()` - Get full path to embedding model
- `GetChatModelPath()` - Get full path to chat model
- `GetModelInfo()` - Read model metadata

### OfflineLLMService (`llama.go`)

Implements `interfaces.LLMService` using local llama-cli binary execution.

**Interface Methods:**
- `Embed(ctx, text)` - Generate 768-dimension embedding vector
- `Chat(ctx, messages)` - Generate chat completion
- `HealthCheck(ctx)` - Verify service is operational
- `GetMode()` - Returns `LLMModeOffline`
- `Close()` - Release resources (no-op for binary execution)

**Testing Methods:**
- `SetMockMode(enabled)` - Enable/disable mock mode for testing

## Usage

### Basic Initialization

```go
import (
    "github.com/ternarybob/arbor"
    "github.com/ternarybob/quaero/internal/services/llm/offline"
)

logger := arbor.NewLogger()

service, err := offline.NewOfflineLLMService(
    "./models",                           // Model directory
    "nomic-embed-text-v1.5-q8.gguf",     // Embedding model
    "qwen2.5-7b-instruct-q4.gguf",       // Chat model
    2048,                                 // Context size
    4,                                    // Thread count
    0,                                    // GPU layers (0 = CPU only)
    logger,
)
if err != nil {
    log.Fatal(err)
}
defer service.Close()
```

### Generate Embeddings

```go
ctx := context.Background()
text := "This is a sample document to embed"

embedding, err := service.Embed(ctx, text)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Generated %d-dimension embedding\n", len(embedding))
// Output: Generated 768-dimension embedding
```

### Chat Completions

```go
messages := []interfaces.Message{
    {Role: "system", Content: "You are a helpful assistant."},
    {Role: "user", Content: "What is the capital of France?"},
}

response, err := service.Chat(ctx, messages)
if err != nil {
    log.Fatal(err)
}

fmt.Println(response)
// Output: The capital of France is Paris.
```

### Health Checks

```go
if err := service.HealthCheck(ctx); err != nil {
    log.Printf("Service unhealthy: %v", err)
}
```

## llama-cli Binary

The service searches for `llama-cli` in the following locations:
1. `./bin/llama-cli` (or `.exe` on Windows)
2. `./llama-cli` (or `.exe` on Windows)
3. `llama-cli` in PATH

### Building llama-cli

To build from llama.cpp source:

```bash
git clone https://github.com/ggerganov/llama.cpp
cd llama.cpp
make llama-cli

# Copy to Quaero bin directory
cp llama-cli /path/to/quaero/bin/
```

For GPU support (CUDA):
```bash
make llama-cli LLAMA_CUBLAS=1
```

For GPU support (Metal on macOS):
```bash
make llama-cli LLAMA_METAL=1
```

## Models

### Recommended Models

**Embedding Model:**
- `nomic-embed-text-v1.5-q8.gguf` (137 MB)
- 768-dimension embeddings
- Optimized for search and retrieval

**Chat Model:**
- `qwen2.5-7b-instruct-q4.gguf` (4.3 GB) - Recommended for most systems
- `qwen2.5-3b-instruct-q4.gguf` (2.1 GB) - Smaller, faster option
- `qwen2.5-14b-instruct-q4.gguf` (8.5 GB) - Better quality, more resources

### Downloading Models

Models can be downloaded from Hugging Face:

```bash
# Create models directory
mkdir -p models

# Download embedding model
wget https://huggingface.co/nomic-ai/nomic-embed-text-v1.5-GGUF/resolve/main/nomic-embed-text-v1.5.Q8_0.gguf \
  -O models/nomic-embed-text-v1.5-q8.gguf

# Download chat model (choose one)
wget https://huggingface.co/Qwen/Qwen2.5-7B-Instruct-GGUF/resolve/main/qwen2.5-7b-instruct-q4_0.gguf \
  -O models/qwen2.5-7b-instruct-q4.gguf
```

## Configuration

The service is configured via `config.toml`:

```toml
[llm]
mode = "offline"  # Use offline mode

[llm.offline]
model_dir = "./models"
embed_model = "nomic-embed-text-v1.5-q8.gguf"
chat_model = "qwen2.5-7b-instruct-q4.gguf"
context_size = 2048
thread_count = 4    # Adjust based on CPU cores
gpu_layers = 0      # Set to > 0 for GPU offloading
```

### Environment Variables

```bash
export QUAERO_LLM_MODE=offline
export QUAERO_LLM_OFFLINE_MODEL_DIR=./models
export QUAERO_LLM_OFFLINE_EMBED_MODEL=nomic-embed-text-v1.5-q8.gguf
export QUAERO_LLM_OFFLINE_CHAT_MODEL=qwen2.5-7b-instruct-q4.gguf
export QUAERO_LLM_OFFLINE_CONTEXT_SIZE=2048
export QUAERO_LLM_OFFLINE_THREAD_COUNT=4
export QUAERO_LLM_OFFLINE_GPU_LAYERS=0
```

## Prompt Format

The service uses Qwen 2.5's ChatML format:

```
<|im_start|>system
You are a helpful assistant.<|im_end|>
<|im_start|>user
What is the capital of France?<|im_end|>
<|im_start|>assistant
```

This format is automatically applied by `formatPrompt()`.

## Performance

### Embedding Generation
- **Speed**: 100-500 tokens/second (CPU)
- **Latency**: 50-200ms per embedding
- **Memory**: ~500 MB (model size)

### Chat Completion
- **Speed**: 10-50 tokens/second (CPU), 50-200 tokens/second (GPU)
- **Latency**: 1-5 seconds for typical responses
- **Memory**: 4-16 GB depending on model size

### Optimization Tips

1. **Use quantized models** - Q4 or Q5 for chat, Q8 for embeddings
2. **Adjust thread count** - Match CPU core count
3. **Enable GPU layers** - Set `gpu_layers` > 0 if CUDA/Metal available
4. **Reduce context size** - Use 1024 or 512 if not needed
5. **Batch operations** - Generate multiple embeddings in sequence

## Testing

### Unit Tests

```bash
go test ./internal/services/llm/offline/...
```

### Mock Mode

For testing without llama-cli binary:

```go
service.SetMockMode(true)

// Now uses deterministic fake responses
embedding, _ := service.Embed(ctx, "test")
response, _ := service.Chat(ctx, messages)
```

## Security Guarantees

**This service guarantees 100% local operation:**
- ✅ No network imports (`net/http`, `net`, etc.)
- ✅ Only file system I/O (local models)
- ✅ Only local binary execution (`os/exec`)
- ✅ No external API calls
- ✅ No telemetry or logging to external services

**Code audit checklist:**
1. No `import "net/http"` or networking packages
2. Only `os/exec` for binary execution
3. All file paths are local
4. No HTTP clients or network connections
5. Binary must be local (not downloaded at runtime)

## Troubleshooting

### Binary Not Found

**Error:** `llama-cli binary not found`

**Solution:** Ensure llama-cli is in one of the search paths:
```bash
# Check if binary exists
which llama-cli

# Or place in project bin directory
cp /path/to/llama-cli ./bin/
```

### Models Not Found

**Error:** `embedding model not found: ./models/nomic-embed-text-v1.5-q8.gguf`

**Solution:** Verify model files exist:
```bash
ls -lh models/
# Download if missing (see Models section)
```

### Out of Memory

**Error:** Process killed or OOM error

**Solution:** Use smaller model or reduce context size:
- Try `qwen2.5-3b-instruct-q4.gguf` instead of 7B
- Reduce `context_size` to 1024 or 512
- Close other applications

### Slow Performance

**Issue:** Chat responses take 10+ seconds

**Solution:**
- Increase `thread_count` to match CPU cores
- Use quantized models (Q4 instead of Q8)
- Enable GPU layers if available
- Reduce `context_size`

## License

This service uses llama.cpp (MIT License) for model inference.
