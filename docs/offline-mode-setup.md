# Quaero Offline Mode Setup Guide

**Complete guide to running Quaero with local LLM processing for secure, air-gapped deployments.**

---

## Table of Contents

1. [Introduction](#introduction)
2. [Prerequisites](#prerequisites)
3. [Installing llama.cpp](#installing-llamacpp)
4. [Downloading Models](#downloading-models)
5. [Configuring Quaero](#configuring-quaero)
6. [Building and Running](#building-and-running)
7. [Testing the Installation](#testing-the-installation)
8. [Troubleshooting](#troubleshooting)
9. [Security Verification](#security-verification)
10. [Performance Tuning](#performance-tuning)
11. [Model Selection Guide](#model-selection-guide)
12. [Frequently Asked Questions](#frequently-asked-questions)

---

## Introduction

### What is Offline Mode?

Offline mode runs Quaero with **completely local LLM processing** using llama.cpp. No data is sent to external APIs.

**Key Features:**
- All embeddings and chat completions generated locally
- Works in air-gapped environments (no internet required after setup)
- Full audit trail for compliance
- Network isolation verifiable through code review
- No ongoing API costs

### Why Offline Mode?

**MANDATORY for:**
- Government data (any level: local, state, federal)
- Healthcare records (HIPAA compliance)
- Financial information (customer data, internal financials)
- Personal information (PII, employee records)
- Confidential business data (trade secrets, strategic plans)
- Any data where breach would cause legal/reputational harm

**Reference Incident:**

The [ABC News Northern Rivers data breach](https://www.abc.net.au/news/2025-10-06/data-breach-northern-rivers-resilient-homes-program-chatgpt/105855284) demonstrates exactly what offline mode prevents:
- Government agency sent sensitive citizen data to OpenAI ChatGPT
- Data breach of personal information including addresses and damage reports
- Exactly the scenario Quaero's offline mode was designed to prevent

### Security Benefits

**Offline mode guarantees:**
1. ✅ No HTTP clients created in offline code paths
2. ✅ No DNS lookups or socket connections to external servers
3. ✅ All inference occurs via local llama.cpp bindings
4. ✅ Model files loaded only from local disk
5. ✅ Comprehensive audit trail proving no external API calls
6. ✅ Sanity check for internet connectivity on startup

---

## Prerequisites

### System Requirements

**Minimum:**
- Go 1.25 or later
- 8GB RAM
- 4-core CPU
- 10GB disk space (5GB models + 5GB data)
- Git
- CMake 3.14 or later
- C++ compiler (GCC 7+, Clang 8+, MSVC 2019+)

**Recommended:**
- 16GB+ RAM
- 8+ core CPU (or 4-core with hyperthreading)
- SSD storage
- CUDA-compatible GPU (optional, for acceleration)

### Platform-Specific Prerequisites

**Linux (Debian/Ubuntu):**
```bash
# Install build tools
sudo apt-get update
sudo apt-get install -y build-essential git cmake

# For GPU support (optional)
sudo apt-get install -y nvidia-cuda-toolkit
```

**Linux (RHEL/CentOS/Fedora):**
```bash
# Install build tools
sudo dnf groupinstall "Development Tools"
sudo dnf install git cmake

# For GPU support (optional)
sudo dnf install cuda
```

**macOS:**
```bash
# Install Xcode Command Line Tools
xcode-select --install

# Install CMake via Homebrew
brew install cmake

# GPU acceleration available via Metal (M1/M2/M3 Macs)
```

**Windows:**
```powershell
# Install Visual Studio 2019 or later with C++ support
# Download from: https://visualstudio.microsoft.com/

# Install CMake
# Download from: https://cmake.org/download/

# Install Git
# Download from: https://git-scm.com/download/win

# For GPU support (optional)
# Install CUDA Toolkit: https://developer.nvidia.com/cuda-downloads
```

---

## Installing llama.cpp

Quaero uses llama.cpp for offline inference. You have two options: use Go bindings (recommended) or build llama-cli separately.

### Option 1: Go Bindings (Recommended)

The Go bindings will be built automatically when you build Quaero. No separate installation required.

**Verify Go bindings work:**
```bash
cd C:\development\quaero
go mod download
go build ./cmd/quaero
```

### Option 2: Standalone llama-cli

For advanced use cases or debugging, you can build llama-cli separately.

**Linux/macOS:**
```bash
# Clone llama.cpp repository
git clone https://github.com/ggerganov/llama.cpp.git
cd llama.cpp

# Build with default settings (CPU only)
mkdir build
cd build
cmake ..
cmake --build . --config Release

# Verify installation
./bin/llama-cli --version

# Install to system path (optional)
sudo cp bin/llama-cli /usr/local/bin/
```

**Windows (PowerShell):**
```powershell
# Clone llama.cpp repository
git clone https://github.com/ggerganov/llama.cpp.git
cd llama.cpp

# Build with Visual Studio
mkdir build
cd build
cmake ..
cmake --build . --config Release

# Verify installation
.\bin\Release\llama-cli.exe --version

# Add to PATH (optional)
$env:PATH += ";C:\path\to\llama.cpp\build\bin\Release"
```

**GPU Acceleration:**

For CUDA GPU support:
```bash
# Linux/macOS
cmake .. -DLLAMA_CUDA=ON
cmake --build . --config Release

# Windows
cmake .. -DLLAMA_CUDA=ON
cmake --build . --config Release
```

For Apple Silicon (Metal):
```bash
# Enabled by default on macOS with M1/M2/M3
cmake .. -DLLAMA_METAL=ON
cmake --build . --config Release
```

---

## Downloading Models

Quaero requires two models for offline mode:

1. **Embedding Model** - Converts text to vectors (~150MB)
2. **Chat Model** - Generates responses to queries (~4.5GB)

### Recommended Models

**Embedding: nomic-embed-text-v1.5-q8.gguf**
- Size: ~150MB
- Dimensions: 768
- Quality: Excellent for document embeddings
- License: Apache 2.0

**Chat: qwen2.5-7b-instruct-q4.gguf**
- Size: ~4.5GB
- Parameters: 7 billion
- Quality: Good balance of quality and resource usage
- License: Apache 2.0

### Download Instructions

**Linux/macOS:**
```bash
# Create models directory
mkdir -p models
cd models

# Download embedding model (~150MB)
curl -L -o nomic-embed-text-v1.5-q8.gguf \
  https://huggingface.co/nomic-ai/nomic-embed-text-v1.5-GGUF/resolve/main/nomic-embed-text-v1.5.q8_0.gguf

# Download chat model (~4.5GB)
curl -L -o qwen2.5-7b-instruct-q4.gguf \
  https://huggingface.co/Qwen/Qwen2.5-7B-Instruct-GGUF/resolve/main/qwen2.5-7b-instruct-q4_k_m.gguf

# Return to project root
cd ..
```

**Windows (PowerShell):**
```powershell
# Create models directory
New-Item -ItemType Directory -Force -Path models
cd models

# Download embedding model (~150MB)
Invoke-WebRequest -Uri "https://huggingface.co/nomic-ai/nomic-embed-text-v1.5-GGUF/resolve/main/nomic-embed-text-v1.5.q8_0.gguf" `
  -OutFile "nomic-embed-text-v1.5-q8.gguf"

# Download chat model (~4.5GB)
Invoke-WebRequest -Uri "https://huggingface.co/Qwen/Qwen2.5-7B-Instruct-GGUF/resolve/main/qwen2.5-7b-instruct-q4_k_m.gguf" `
  -OutFile "qwen2.5-7b-instruct-q4.gguf"

# Return to project root
cd ..
```

### Verify Downloads

**Check file sizes:**
```bash
# Linux/macOS
ls -lh models/

# Expected output:
# nomic-embed-text-v1.5-q8.gguf     ~150M
# qwen2.5-7b-instruct-q4.gguf       ~4.5G
```

```powershell
# Windows
Get-ChildItem models/ | Select-Object Name, @{Name="Size";Expression={"{0:N2} MB" -f ($_.Length / 1MB)}}
```

**Verify checksums (recommended):**
```bash
# Linux/macOS
sha256sum models/*.gguf

# Compare with official checksums from HuggingFace
```

```powershell
# Windows
Get-FileHash models\*.gguf -Algorithm SHA256
```

---

## Configuring Quaero

### Step 1: Copy Example Configuration

```bash
# Copy offline configuration template
cp deployments/config.offline.example.toml config.toml
```

### Step 2: Verify Model Paths

**Open config.toml and verify:**

```toml
[llm]
mode = "offline"  # CRITICAL: Must be "offline"

[llm.offline]
model_dir = "./models"
embed_model = "nomic-embed-text-v1.5-q8.gguf"
chat_model = "qwen2.5-7b-instruct-q4.gguf"
context_size = 2048
thread_count = 4
gpu_layers = 0
```

### Step 3: Optimize for Your Hardware

**CPU Threads:**

Find your CPU core count:
```bash
# Linux
nproc

# macOS
sysctl -n hw.ncpu

# Windows
echo $env:NUMBER_OF_PROCESSORS
```

Set `thread_count` to your **physical core count** (not hyperthreads):
```toml
# Example: 8-core CPU
thread_count = 8
```

**GPU Acceleration (Optional):**

If you have a CUDA-compatible GPU, set `gpu_layers` based on your GPU memory:

```toml
# RTX 3060 (12GB VRAM)
gpu_layers = 20

# RTX 3090 (24GB VRAM)
gpu_layers = 35

# RTX 4090 (24GB VRAM)
gpu_layers = 40

# A100 (40GB VRAM)
gpu_layers = 60
```

**Context Size:**

Adjust based on your RAM and use case:
```toml
# Smaller context, less RAM, faster
context_size = 2048

# Balanced
context_size = 4096

# Larger context, more RAM, slower
context_size = 8192
```

### Step 4: Configure Data Sources

**Confluence/Jira:**

Authentication is handled via Chrome extension. Just enable:
```toml
[sources.confluence]
enabled = true
spaces = []  # Empty = all accessible spaces

[sources.jira]
enabled = true
projects = []  # Empty = all accessible projects
```

**GitHub (Optional):**

Create a personal access token:
1. Go to https://github.com/settings/tokens
2. Generate new token (classic)
3. Select scopes: `repo`, `read:org`
4. Copy token

Set via environment variable:
```bash
# Linux/macOS
export QUAERO_GITHUB_TOKEN=ghp_xxxxxxxxxxxx

# Windows PowerShell
$env:QUAERO_GITHUB_TOKEN = "ghp_xxxxxxxxxxxx"
```

Or in config.toml:
```toml
[sources.github]
enabled = true
token = "${QUAERO_GITHUB_TOKEN}"  # Reads from env var
repos = ["owner/repo1", "owner/repo2"]
```

---

## Building and Running

### Build Quaero

**Using build script (recommended):**
```bash
# Windows
.\scripts\build.ps1

# Linux/macOS
./scripts/build.sh
```

**Manual build:**
```bash
go build -o bin/quaero ./cmd/quaero
```

### Run Quaero

```bash
# Windows
.\bin\quaero.exe serve --config config.toml

# Linux/macOS
./bin/quaero serve --config config.toml
```

### Expected Startup Output

```
 ╔═══════════════════════════════════════════════════════╗
 ║                       Quaero                          ║
 ║              Knowledge Search System                  ║
 ╠═══════════════════════════════════════════════════════╣
 ║  Version    │ 0.1.0                                   ║
 ║  Mode       │ ✓ OFFLINE (local processing)           ║
 ║  Server     │ localhost:8085                          ║
 ║  Config     │ config.toml                             ║
 ╚═══════════════════════════════════════════════════════╝

INFO Quaero starting version=0.1.0
INFO Loading embedding model path=./models/nomic-embed-text-v1.5-q8.gguf
INFO Loading chat model path=./models/qwen2.5-7b-instruct-q4.gguf
INFO Models loaded successfully
INFO ✓ OFFLINE MODE ACTIVE
INFO ✓ All processing will be local
INFO Server starting host=localhost port=8085
```

**Critical checks:**
- ✅ Mode shows "✓ OFFLINE (local processing)"
- ✅ No "WARNING: Cloud mode active" message
- ✅ Logs show "Loading embedding model" and "Loading chat model"
- ✅ "Models loaded successfully"

---

## Testing the Installation

### Step 1: Verify Server is Running

**Check server health:**
```bash
curl http://localhost:8085/health
# Expected: {"status":"ok","mode":"offline"}
```

### Step 2: Test Embedding Generation

**Via Web UI:**
1. Open http://localhost:8085
2. Navigate to Settings → LLM Test
3. Enter test text: "This is a test document"
4. Click "Generate Embedding"
5. Verify: 768-dimensional vector returned

**Via API:**
```bash
curl -X POST http://localhost:8085/api/embed \
  -H "Content-Type: application/json" \
  -d '{"text":"This is a test document"}'

# Expected: 768-element float array
```

### Step 3: Test Chat Completion

**Via Web UI:**
1. Navigate to Search
2. Enter query: "What is Quaero?"
3. Click Search
4. Verify: Response generated locally

**Via API:**
```bash
curl -X POST http://localhost:8085/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {"role": "user", "content": "What is Quaero?"}
    ]
  }'

# Expected: JSON response with generated text
```

### Step 4: Verify Audit Logs

**Check audit trail:**
```bash
# Connect to SQLite database
sqlite3 ./data/quaero.db

# Query audit log
SELECT * FROM audit_log WHERE mode = 'offline' ORDER BY timestamp DESC LIMIT 10;

# Expected columns:
# - mode: 'offline'
# - operation: 'embed' or 'chat'
# - provider: 'llama.cpp'
# - success: 1
# - latency_ms: ~5000-8000
```

### Step 5: Performance Benchmark

**Run benchmark:**
```bash
# Generate 10 embeddings
time for i in {1..10}; do
  curl -s -X POST http://localhost:8085/api/embed \
    -H "Content-Type: application/json" \
    -d "{\"text\":\"Test document $i\"}"
done
```

**Expected performance:**
- CPU-only (8 cores): ~1-2 seconds per embedding
- GPU-accelerated: ~0.5-1 second per embedding

---

## Troubleshooting

### llama-cli not found

**Symptom:**
```
Error: exec: "llama-cli": executable file not found in $PATH
```

**Solutions:**

**Option 1: Use Go bindings (recommended)**
The error suggests you're trying to use standalone llama-cli. Quaero uses Go bindings by default.

Verify Go bindings:
```bash
go mod download
go build ./cmd/quaero
```

**Option 2: Install llama-cli to PATH**
```bash
# Linux/macOS
sudo cp /path/to/llama.cpp/build/bin/llama-cli /usr/local/bin/

# Windows
$env:PATH += ";C:\path\to\llama.cpp\build\bin\Release"
```

### Model file not found

**Symptom:**
```
Error: embedding model not found: ./models/nomic-embed-text-v1.5-q8.gguf
```

**Solutions:**

1. **Verify model directory exists:**
   ```bash
   ls -la models/
   ```

2. **Check file names match exactly:**
   ```bash
   # Should show exact filenames from config
   ls models/nomic-embed-text-v1.5-q8.gguf
   ls models/qwen2.5-7b-instruct-q4.gguf
   ```

3. **Re-download if missing:**
   See [Downloading Models](#downloading-models) section

4. **Check file permissions:**
   ```bash
   # Linux/macOS
   chmod 644 models/*.gguf
   ```

### Out of memory

**Symptom:**
```
Error: failed to load model: insufficient memory
```

**Solutions:**

1. **Use smaller quantization:**
   ```toml
   # Instead of q8 (8-bit), use q4 (4-bit)
   embed_model = "nomic-embed-text-v1.5-q4.gguf"
   ```

2. **Reduce context size:**
   ```toml
   context_size = 1024  # Down from 2048
   ```

3. **Offload to GPU:**
   ```toml
   gpu_layers = 20  # Move some layers to GPU
   ```

4. **Close other applications** to free RAM

5. **Upgrade RAM** (minimum 8GB, recommended 16GB)

### Slow performance

**Symptom:**
Embeddings take >10 seconds, chat completions take >30 seconds

**Solutions:**

1. **Increase thread count:**
   ```toml
   thread_count = 8  # Match your CPU cores
   ```

2. **Enable GPU acceleration:**
   ```toml
   gpu_layers = 35  # Offload to GPU
   ```

3. **Use smaller model:**
   ```toml
   # Instead of 7B model, use 3B or 1.5B
   chat_model = "qwen2.5-3b-instruct-q4.gguf"
   ```

4. **Reduce context size:**
   ```toml
   context_size = 1024  # Smaller context, faster inference
   ```

5. **Verify CPU governor** (Linux):
   ```bash
   # Set to performance mode
   sudo cpupower frequency-set -g performance
   ```

### GPU not working

**Symptom:**
```
Warning: GPU layers requested but CUDA not available, falling back to CPU
```

**Solutions:**

1. **Verify CUDA installation:**
   ```bash
   nvidia-smi  # Should show GPU info
   nvcc --version  # Should show CUDA version
   ```

2. **Rebuild llama.cpp with CUDA:**
   ```bash
   cd llama.cpp/build
   cmake .. -DLLAMA_CUDA=ON
   cmake --build . --config Release
   ```

3. **Check CUDA libraries:**
   ```bash
   # Linux
   ldconfig -p | grep cuda

   # Windows
   # Verify CUDA in: C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA
   ```

4. **Update GPU drivers:**
   - NVIDIA: https://www.nvidia.com/download/index.aspx
   - Minimum CUDA 11.0 required

### Permission denied

**Symptom:**
```
Error: failed to read model file: permission denied
```

**Solutions:**

```bash
# Linux/macOS
chmod 644 models/*.gguf
chmod 755 models/

# Windows (PowerShell as Administrator)
icacls models /grant Everyone:R /T
```

### Database locked

**Symptom:**
```
Error: database is locked
```

**Solutions:**

1. **Increase busy timeout:**
   ```toml
   [storage.sqlite]
   busy_timeout_ms = 10000  # 10 seconds
   ```

2. **Enable WAL mode:**
   ```toml
   [storage.sqlite]
   wal_mode = true  # Better concurrency
   ```

3. **Close other connections:**
   ```bash
   # Check for other processes
   lsof ./data/quaero.db  # Linux/macOS
   ```

---

## Security Verification

### Network Isolation Test

**Verify no external connections:**

**Linux:**
```bash
# Start Quaero
./bin/quaero serve --config config.toml &

# Monitor network connections (should be only localhost)
netstat -an | grep ESTABLISHED | grep quaero

# Expected: Only connections to localhost:8085
# No connections to external IPs
```

**macOS:**
```bash
# Monitor network connections
lsof -i -P | grep quaero

# Expected: Only localhost connections
```

**Windows:**
```powershell
# Monitor network connections
netstat -ano | Select-String quaero

# Expected: Only Local Address = 127.0.0.1:8085
```

### Audit Log Verification

**Verify all operations are local:**

```sql
-- Connect to database
sqlite3 ./data/quaero.db

-- Check all LLM operations are offline
SELECT
  mode,
  COUNT(*) as operations,
  AVG(latency_ms) as avg_latency,
  SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) as successful
FROM audit_log
GROUP BY mode;

-- Expected result:
-- mode     | operations | avg_latency | successful
-- offline  | 100        | 5234        | 100

-- Should NOT see any 'cloud' mode entries
```

### Code Review Verification

**Verify no HTTP clients in offline code:**

```bash
# Search for http.Client in offline code
grep -r "http.Client" internal/services/llm/offline/

# Expected: No results (no HTTP clients)

# Search for net.Dial
grep -r "net.Dial" internal/services/llm/offline/

# Expected: No results (no network connections)
```

### File System Permissions

**Set restrictive permissions for security:**

```bash
# Linux/macOS
chmod 700 ./data/             # Only owner can access
chmod 600 ./data/quaero.db    # Only owner can read/write
chmod 600 config.toml         # Protect configuration
chmod 644 models/*.gguf       # Models can be read-only

# Verify
ls -la ./data/
ls -la config.toml
ls -la models/
```

### Process Isolation

**Run as dedicated user (recommended for production):**

```bash
# Create dedicated user
sudo useradd -r -s /bin/false quaero

# Set ownership
sudo chown -R quaero:quaero /opt/quaero

# Run as quaero user
sudo -u quaero ./bin/quaero serve --config config.toml
```

---

## Performance Tuning

### CPU Optimization

**Thread Count:**

```toml
# Find optimal thread count
# Rule of thumb: Number of physical cores

# Test different values
thread_count = 4   # Baseline
thread_count = 8   # 8-core CPU
thread_count = 16  # 16-core CPU

# Monitor CPU usage
# Optimal: 80-100% CPU utilization during inference
```

**CPU Affinity (Linux):**

```bash
# Pin to specific cores
taskset -c 0-7 ./bin/quaero serve --config config.toml

# Verify
ps -eLo pid,tid,psr,comm | grep quaero
```

**NUMA Optimization (Linux):**

```bash
# Check NUMA topology
numactl --hardware

# Run on specific NUMA node
numactl --cpunodebind=0 --membind=0 ./bin/quaero serve --config config.toml
```

### GPU Optimization

**Layer Offloading:**

Start with conservative values and increase:
```toml
# Test incrementally
gpu_layers = 10   # Start conservative
gpu_layers = 20   # Increase if VRAM allows
gpu_layers = 35   # RTX 3090 sweet spot
gpu_layers = 60   # Full offload for large VRAM
```

**Monitor VRAM usage:**
```bash
# Watch GPU memory
watch -n 1 nvidia-smi

# Expected: 80-90% VRAM utilization
# If <50%: Increase gpu_layers
# If >95%: Decrease gpu_layers
```

**Batch Size (Advanced):**

```toml
# For high-throughput scenarios
batch_size = 512  # Default
batch_size = 1024 # Higher throughput
batch_size = 2048 # Maximum (requires VRAM)
```

### Context Size Tuning

**Balance quality vs speed:**

```toml
# Small: Fast, less context
context_size = 1024  # ~750 words context

# Medium: Balanced (recommended)
context_size = 2048  # ~1500 words context

# Large: Slower, more context
context_size = 4096  # ~3000 words context

# Extra Large: Very slow, maximum context
context_size = 8192  # ~6000 words context
```

**RAM usage estimate:**
- 2048 context: ~6GB RAM
- 4096 context: ~10GB RAM
- 8192 context: ~18GB RAM

### Disk I/O Optimization

**Use SSD for model files:**
```bash
# Check disk type
lsblk -d -o name,rota
# rota=0: SSD, rota=1: HDD

# Move models to SSD if on HDD
mv models /mnt/ssd/quaero-models
```

**SQLite optimization:**
```toml
[storage.sqlite]
# Larger cache for better performance
cache_size_mb = 256  # Up from 64

# WAL mode for better concurrency
wal_mode = true

# Increase busy timeout
busy_timeout_ms = 10000
```

---

## Model Selection Guide

### Embedding Models

**nomic-embed-text-v1.5 (Recommended)**
- Size: 150MB (q8), 85MB (q4)
- Dimensions: 768
- Quality: Excellent
- License: Apache 2.0
- Use case: General purpose

**all-MiniLM-L6-v2 (Alternative)**
- Size: 90MB
- Dimensions: 384
- Quality: Good
- License: Apache 2.0
- Use case: Resource-constrained

### Chat Models

**Small (1.5B-3B parameters):**
- Qwen2.5-1.5B-Instruct (~1GB)
  - Best for: Low RAM systems
  - Speed: Very fast (~1-2s)
  - Quality: Basic

**Medium (7B parameters) - Recommended:**
- Qwen2.5-7B-Instruct (~4.5GB q4)
  - Best for: Balanced quality/speed
  - Speed: Fast (~3-5s)
  - Quality: Good

**Large (13B-14B parameters):**
- Qwen2.5-14B-Instruct (~8GB q4)
  - Best for: Higher quality needed
  - Speed: Medium (~8-12s)
  - Quality: Very good

**Extra Large (30B+ parameters):**
- Qwen2.5-32B-Instruct (~18GB q4)
  - Best for: Maximum quality
  - Speed: Slow (~15-30s)
  - Quality: Excellent

### Quantization Levels

**q4 (4-bit) - Recommended:**
- Size: Smallest
- Speed: Fastest
- Quality: Good (95% of fp16)
- RAM: Lowest

**q5 (5-bit):**
- Size: Medium
- Speed: Medium
- Quality: Better (97% of fp16)
- RAM: Medium

**q8 (8-bit):**
- Size: Larger
- Speed: Slower
- Quality: Excellent (99% of fp16)
- RAM: Higher

**fp16 (16-bit):**
- Size: Largest
- Speed: Slowest
- Quality: Reference (100%)
- RAM: Highest

**Recommendation:** Start with q4, upgrade to q8 if quality insufficient.

---

## Frequently Asked Questions

### Can I use Quaero completely offline?

**Yes.** After initial model download, Quaero works 100% offline with no internet connection required.

**Setup:**
1. Download models on internet-connected machine
2. Copy models to air-gapped machine
3. Run Quaero in offline mode

### How do I verify no data is sent externally?

**Three verification methods:**

1. **Startup logs:** Should show "✓ OFFLINE MODE ACTIVE"
2. **Audit logs:** Query `SELECT DISTINCT mode FROM audit_log;` should only show "offline"
3. **Network monitoring:** Use `netstat` or `tcpdump` to verify no external connections

### What's the difference between offline and cloud mode?

| Aspect | Offline Mode | Cloud Mode |
|--------|--------------|------------|
| Data privacy | ✅ Stays local | ❌ Sent to API |
| Internet | Setup only | Always required |
| Performance | Slower (~5-8s) | Faster (~1-2s) |
| Quality | Good | Excellent |
| Cost | One-time | Per-use |
| Compliance | ✅ Audit-ready | ❌ Not for regulated data |

### Can I switch between offline and cloud mode?

**Yes, but requires restart:**

1. Stop Quaero
2. Edit `config.toml`:
   ```toml
   [llm]
   mode = "offline"  # or "cloud"
   ```
3. Restart Quaero

**Warning:** Changing mode does NOT migrate existing embeddings. Re-process documents after mode change.

### How much disk space do I need?

**Minimum:**
- Models: 5GB (embed + chat)
- Database: 1GB (per 10,000 documents)
- Logs: 100MB
- Total: ~7GB

**Recommended:**
- Models: 5GB
- Database: 10GB (room to grow)
- Logs: 1GB
- Total: ~16GB

### Can I run multiple instances?

**Yes, with different ports:**

```toml
# Instance 1
[server]
port = 8085

# Instance 2
[server]
port = 8086

# Instance 3
[server]
port = 8087
```

**Note:** Each instance loads models into RAM. For 3 instances with 7B model, need ~24GB RAM.

### How do I update models?

1. **Download new model:**
   ```bash
   curl -L -o models/new-model.gguf https://...
   ```

2. **Update config:**
   ```toml
   [llm.offline]
   chat_model = "new-model.gguf"
   ```

3. **Restart Quaero:**
   ```bash
   ./bin/quaero serve --config config.toml
   ```

4. **Re-process documents** to use new model

### What hardware do you recommend?

**Minimum (Personal use):**
- CPU: 4-core Intel/AMD
- RAM: 8GB
- Storage: 10GB SSD
- GPU: Not required

**Recommended (Team use):**
- CPU: 8-core Intel/AMD or Apple M1/M2
- RAM: 16GB
- Storage: 50GB SSD
- GPU: RTX 3060 (12GB) or better

**Production (Enterprise):**
- CPU: 16+ core Xeon/EPYC
- RAM: 32GB+
- Storage: 100GB NVMe SSD
- GPU: RTX 3090/4090 or A100

### How do I backup my data?

**Backup checklist:**
```bash
# 1. Database
cp ./data/quaero.db ./backups/quaero-$(date +%Y%m%d).db

# 2. Configuration
cp config.toml ./backups/config-$(date +%Y%m%d).toml

# 3. Models (optional, can re-download)
cp -r models ./backups/models-$(date +%Y%m%d)/

# 4. Audit logs
sqlite3 ./data/quaero.db ".dump audit_log" > ./backups/audit-$(date +%Y%m%d).sql
```

**Restore:**
```bash
# Stop Quaero
./bin/quaero stop

# Restore database
cp ./backups/quaero-20251006.db ./data/quaero.db

# Restart
./bin/quaero serve --config config.toml
```

---

## Additional Resources

### Documentation
- [Quaero Requirements](requirements.md)
- [Architecture Guide](architecture.md)
- [Configuration Reference](../deployments/config.offline.example.toml)

### External Resources
- [llama.cpp GitHub](https://github.com/ggerganov/llama.cpp)
- [Nomic Embed Models](https://huggingface.co/nomic-ai)
- [Qwen Models](https://huggingface.co/Qwen)
- [GGUF Format Specification](https://github.com/ggerganov/ggml/blob/master/docs/gguf.md)

### Support
- GitHub Issues: https://github.com/ternarybob/quaero/issues
- Discussions: https://github.com/ternarybob/quaero/discussions

---

**Last Updated:** 2025-10-06
**Version:** 1.0
**Status:** Production Ready
