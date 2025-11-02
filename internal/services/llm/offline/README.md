# Offline LLM Service

**SECURITY CRITICAL**: This service provides 100% local LLM operations with NO network calls.

## Architecture

The offline LLM service uses HTTP API execution of `llama-server` (from llama.cpp) instead of CGo bindings. This provides:

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

Implements `interfaces.LLMService` using local llama-server binary with HTTP API.

**Interface Methods:**
- `Embed(ctx, text)` - Generate 768-dimension embedding vector
- `Chat(ctx, messages)` - Generate chat completion
- `HealthCheck(ctx)` - Verify service is operational
- `GetMode()` - Returns `LLMModeOffline`
- `Close()` - Release resources (stops llama-server subprocess)

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
    "./llama",                            // llama-server binary directory
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

## llama-server Binary

**Important**: The service uses `llama-server` (HTTP API mode), not `llama-cli` (CLI mode). The HTTP server binary was renamed from `server` to `llama-server` in 2024.

The service searches for `llama-server` in the following locations:
1. `{llamaDir}/llama-server` (or `.exe` on Windows) - where llamaDir defaults to `./llama`
2. `./bin/llama-server` (or `.exe`)
3. `./llama-server` (or `.exe`)
4. `llama-server` in PATH

You can configure the llama directory via:
- `config.Server.LlamaDir` in quaero.toml
- `QUAERO_SERVER_LLAMA_DIR` environment variable

### Building llama-server

To build from llama.cpp source:

```bash
git clone https://github.com/ggerganov/llama.cpp
cd llama.cpp
make llama-server

# Copy to Quaero bin directory
cp llama-server /path/to/quaero/bin/
```

For GPU support (CUDA):
```bash
make llama-server LLAMA_CUBLAS=1
```

For GPU support (Metal on macOS):
```bash
make llama-server LLAMA_METAL=1
```

**Note**: The service manages llama-server as a subprocess with OpenAI-compatible HTTP endpoints. The binary is automatically started, stopped, and health-checked by the service.

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
- ✅ Localhost-only HTTP communication using net/http with dialer enforcing 127.0.0.1 only (no external network connections)
- ✅ Only file system I/O (local models)
- ✅ Only local binary execution (`os/exec`)
- ✅ No external API calls
- ✅ No telemetry or logging to external services

**Code audit checklist:**
1. Only net/http with localhost enforcement (127.0.0.1 only)
2. Only `os/exec` for binary execution
3. All file paths are local
4. No HTTP clients or external network connections
5. Binary must be local (not downloaded at runtime)

## Troubleshooting

### Binary Not Found

**Error:** `llama-server binary not found`

**Solution:** Ensure llama-server is in one of the search paths:
```bash
# Check if binary exists (Windows)
where llama-server

# Check if binary exists (Unix/macOS)
which llama-server

# Or place in project bin directory
cp /path/to/llama-server ./bin/
```

### Service Falls Back to Mock Mode

**Symptom:** Log shows "Failed to create offline LLM service, falling back to MOCK mode"

**Cause:** llama-server binary not found in any search path

**Solution:**
1. Verify binary exists with the commands above
2. Check the configured llama_dir matches the binary location
3. Ensure binary has execute permissions (Unix/macOS): `chmod +x ./llama/llama-server`

**Verification commands:**
```powershell
# Windows
where llama-server
Test-Path .\llama\llama-server.exe
dir .\llama\llama-server.exe

# Unix/macOS
which llama-server
ls -la ./llama/llama-server
./llama/llama-server --version
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
