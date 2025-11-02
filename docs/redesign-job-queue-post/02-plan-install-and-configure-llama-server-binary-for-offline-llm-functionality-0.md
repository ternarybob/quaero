I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The log file analysis revealed that the LLM service is falling back to **MOCK mode** because the `llama-server` binary cannot be found in any of the expected search paths. The `findLlamaServer()` function in `internal/services/llm/offline/llama.go` searches for the binary in multiple locations but fails to locate it.

**Critical Documentation Inconsistency Discovered:**
- The code in `llama.go` searches for `llama-server` binary (HTTP API mode)
- The documentation in `internal/services/llm/offline/README.md` describes `llama-cli` binary (CLI mode)
- This inconsistency causes confusion and needs correction

**Current Search Paths** (from `llama.go` lines 194-231):
1. `{config.Server.LlamaDir}/llama-server` (default: `./llama/llama-server`)
2. `{config.Server.LlamaDir}/llama-server.exe` (Windows)
3. `./bin/llama-server` and `./bin/llama-server.exe`
4. `./llama-server` and `./llama-server.exe`
5. System PATH via `exec.LookPath("llama-server")`

**User's Documentation Target Error:**
The user mentioned updating `cmd/quaero-chrome-extension/README.md`, but this is incorrect. The Chrome extension README is for extension installation only, not LLM setup. The correct documentation targets are:
- Main `README.md` (add LLM setup section)
- `internal/services/llm/offline/README.md` (fix binary name and update instructions)
- `AGENTS.md` (enhance troubleshooting section)

### Approach

Implement a **three-tier documentation strategy**:

1. **Quick Start Guide** in main README.md - Provide immediate, copy-paste instructions for getting started with offline LLM
2. **Detailed Technical Documentation** in `internal/services/llm/offline/README.md` - Fix binary name inconsistency and provide comprehensive setup instructions
3. **Troubleshooting Enhancement** in AGENTS.md - Add specific verification steps and common issues

This approach ensures users can quickly get started while having access to detailed technical documentation when needed. The documentation will cover all major platforms (Windows, macOS, Linux) and provide multiple installation methods (prebuilt binaries, package managers, source builds).

### Reasoning

Analyzed the log file showing LLM mock mode fallback, explored the codebase to understand binary search logic in `llama.go`, reviewed configuration defaults in `config.go`, examined existing documentation in README.md and offline LLM service README, discovered the llama-cli vs llama-server naming inconsistency, and performed web research to confirm current best practices for llama.cpp installation in 2025.

## Proposed File Changes

### README.md(MODIFY)

References: 

- internal\services\llm\offline\README.md(MODIFY)
- AGENTS.md(MODIFY)

**Add comprehensive LLM Setup section after the "Installing Chrome Extension" section (after line 157):**

1. Create a new top-level section titled "## LLM Setup (Offline Mode)" with subsections:
   - **Prerequisites** - Explain that offline mode requires llama-server binary and model files
   - **Quick Start** - Provide fastest path to get running (prebuilt binaries)
   - **Installation Methods** - Cover three approaches:
     a. **Prebuilt Binaries (Recommended)** - Link to llama.cpp releases page with platform-specific instructions
     b. **Package Managers** - One-line install commands for Homebrew (macOS/Linux), winget (Windows), MacPorts, Nix
     c. **Build from Source** - Brief overview with link to detailed docs
   - **Binary Placement** - Explain the search order and recommend `./llama/llama-server.exe` or `./bin/llama-server.exe`
   - **Model Downloads** - Provide direct download commands for:
     - Embedding model: `nomic-embed-text-v1.5-q8.gguf` (~137 MB)
     - Chat model: `qwen2.5-7b-instruct-q4.gguf` (~4.3 GB)
   - **Verification** - Show how to verify offline mode is working (check startup logs for "LLM service initialized in offline mode" instead of "falling back to MOCK mode")
   - **Troubleshooting** - Link to detailed troubleshooting in `internal/services/llm/offline/README.md` and AGENTS.md

2. Update the existing "LLM configuration" section (lines 92-103) to add a note:
   - Add comment: "# See 'LLM Setup (Offline Mode)' section below for binary and model installation"
   - Add warning about mock mode: "# If llama-server binary is not found, service falls back to MOCK mode (fake responses)"

3. Add a callout box or note emphasizing:
   - Offline mode is the default and recommended for security
   - Mock mode is for testing only and provides fake responses
   - Cloud mode sends data to external APIs (security implications)

**Rationale:** Main README should provide quick-start instructions that get users running immediately, with links to detailed documentation for advanced scenarios. This reduces friction for new users while maintaining comprehensive coverage.

### internal\services\llm\offline\README.md(MODIFY)

References: 

- internal\services\llm\offline\llama.go

**Fix critical binary name inconsistency and update installation instructions:**

1. **Global Find-Replace** throughout the entire file:
   - Replace all instances of `llama-cli` with `llama-server`
   - Update all code examples and commands accordingly
   - This affects sections: "llama-cli Binary" (line 108), "Building llama-cli" (line 115), troubleshooting (line 272)

2. **Update "llama-server Binary" section (lines 108-136):**
   - Correct the search paths to match actual code in `llama.go`:
     - `{llamaDir}/llama-server` (or `.exe` on Windows) where llamaDir defaults to `./llama`
     - `./bin/llama-server` (or `.exe`)
     - `./llama-server` (or `.exe`)
     - `llama-server` in system PATH
   - Add note explaining that llamaDir can be configured via `config.Server.LlamaDir` or `QUAERO_SERVER_LLAMA_DIR` env var

3. **Expand "Building llama-server" section with 2025 best practices:**
   - Add **Prebuilt Binaries** subsection (recommended method):
     - Link to official releases: https://github.com/ggml-org/llama.cpp/releases
     - Provide platform-specific download instructions:
       - Windows: `llama-b6922-bin-win-cpu-x64.zip` (CPU) or CUDA/ROCm variants
       - macOS: `llama-b6922-bin-macos-arm64.zip` (Apple Silicon) or x64 (Intel)
       - Linux: `llama-b6922-bin-ubuntu-x64.zip` (CPU) or Vulkan variant
     - Show extraction and placement commands for each platform
   - Add **Package Manager Installation** subsection:
     - Homebrew (macOS/Linux): `brew install llama.cpp`
     - winget (Windows): `winget install llama.cpp`
     - MacPorts (macOS): `sudo port install llama.cpp`
     - Nix (macOS/Linux): `nix profile install nixpkgs#llama-cpp`
   - Keep existing **Build from Source** instructions but update CMake commands to current syntax
   - Add note about binary renaming in 2024: "The HTTP server binary was renamed from `server` to `llama-server` in 2024"

4. **Update "Usage" section (lines 42-66):**
   - Clarify that the service uses llama-server's HTTP API mode, not CLI mode
   - Explain that llama-server runs as a subprocess with HTTP endpoints
   - Add note about automatic server lifecycle management (start/stop/health checks)

5. **Enhance "Troubleshooting" section (lines 268-310):**
   - Update "Binary Not Found" error to reference `llama-server` instead of `llama-cli`
   - Add new troubleshooting entry: "Service Falls Back to Mock Mode"
     - Symptom: Log shows "Failed to create offline LLM service, falling back to MOCK mode"
     - Cause: llama-server binary not found in any search path
     - Solution: Verify binary exists with `which llama-server` or check expected paths
   - Add verification command for Windows: `where llama-server`
   - Add verification command for Unix: `which llama-server` or `ls -la ./llama/llama-server`

6. **Update "Architecture" section (lines 5-13):**
   - Correct description: "uses HTTP API execution of `llama-server`" instead of "binary execution of `llama-cli`"
   - Explain that llama-server provides OpenAI-compatible HTTP endpoints
   - Clarify that the service manages llama-server as a subprocess with HTTP communication

**Rationale:** This file is the authoritative technical documentation for the offline LLM service. Fixing the binary name inconsistency is critical to prevent user confusion. Adding prebuilt binary instructions provides the fastest path to success, while maintaining source build instructions for advanced users.

### AGENTS.md(MODIFY)

References: 

- internal\services\llm\offline\README.md(MODIFY)
- README.md(MODIFY)
- internal\services\llm\offline\llama.go

**Enhance the "llama-server Issues" troubleshooting section (lines 849-856):**

1. **Expand the existing checklist** with more specific verification steps:
   - Add step 0: "Check startup logs for 'LLM service initialized in offline mode' vs 'falling back to MOCK mode'"
   - Enhance step 1: "`llama-server` binary exists in configured llama_dir" → Add verification commands:
     - Windows: `where llama-server` or `Test-Path .\llama\llama-server.exe`
     - Unix: `which llama-server` or `ls -la ./llama/llama-server`
   - Add step 1.5: "Binary has execute permissions (Unix/macOS only): `chmod +x ./llama/llama-server`"
   - Enhance step 2: Add model verification commands:
     - `ls -lh ./models/nomic-embed-text-v1.5-q8.gguf`
     - `ls -lh ./models/qwen2.5-7b-instruct-q4.gguf`
   - Add step 6: "Check llama-server version compatibility: `./llama/llama-server --version`"

2. **Add new subsection: "### Installing llama-server Binary"** after line 856:
   - Provide quick reference for installation methods:
     - **Prebuilt Binaries**: Link to https://github.com/ggml-org/llama.cpp/releases with note about platform selection
     - **Package Managers**: One-line commands for Homebrew, winget, MacPorts, Nix
     - **Build from Source**: Link to detailed instructions in `internal/services/llm/offline/README.md`
   - Recommend placement in `./llama/llama-server.exe` (Windows) or `./llama/llama-server` (Unix)
   - Note about system PATH: "If installed via package manager, llama-server will be in PATH and automatically found"

3. **Add new subsection: "### Verifying Offline Mode"** after the installation section:
   - Show expected startup log output:
     - ✅ Success: `"LLM service initialized in offline mode"`
     - ❌ Failure: `"Failed to create offline LLM service, falling back to MOCK mode"`
   - Explain mock mode behavior: "Mock mode provides fake responses for testing. Embeddings and chat will not work properly."
   - Provide health check endpoint: `curl http://localhost:8080/api/health` (adjust port if configured differently)

4. **Update the "LLM Service Architecture" section (lines 299-328):**
   - Add note about binary search order and configuration options
   - Clarify that `llama_dir` defaults to `./llama` but can be overridden via config or env var
   - Add example of setting custom llama_dir:
     ```toml
     [server]
     llama_dir = "C:/llama.cpp"  # Windows example
     # llama_dir = "/usr/local/llama"  # Unix example
     ```

5. **Add cross-reference links:**
   - In the "llama-server Issues" section, add: "See `internal/services/llm/offline/README.md` for detailed installation and troubleshooting"
   - In the "LLM Service Architecture" section, add: "See main README.md 'LLM Setup' section for quick start guide"

**Rationale:** AGENTS.md is the primary reference for AI agents and developers working on the project. Enhancing the troubleshooting section with specific verification commands and installation instructions ensures developers can quickly diagnose and resolve LLM setup issues. The cross-references create a cohesive documentation ecosystem.

### deployments\local\quaero.toml(MODIFY)

References: 

- README.md(MODIFY)
- internal\services\llm\offline\llama.go

**Add helpful comments to the LLM configuration section (lines 71-93):**

1. **Enhance the `[llm]` section header comment** (before line 77):
   - Add note: "# IMPORTANT: Offline mode requires llama-server binary and model files"
   - Add note: "# If binary not found, service falls back to MOCK mode (fake responses)"
   - Add note: "# See README.md 'LLM Setup' section for installation instructions"

2. **Add inline comment for `[llm.offline]` section** (before line 81):
   - Add: "# Binary search paths (in order):"
   - Add: "#   1. ./llama/llama-server (or .exe on Windows)"
   - Add: "#   2. ./bin/llama-server (or .exe)"
   - Add: "#   3. System PATH"
   - Add: "# Override with QUAERO_SERVER_LLAMA_DIR environment variable"

3. **Add model download links as comments** (after lines 83-84):
   - After embed_model line: "# Download: https://huggingface.co/nomic-ai/nomic-embed-text-v1.5-GGUF/resolve/main/nomic-embed-text-v1.5.Q8_0.gguf"
   - After chat_model line: "# Download: https://huggingface.co/Qwen/Qwen2.5-7B-Instruct-GGUF/resolve/main/qwen2.5-7b-instruct-q4_0.gguf"

4. **Add example of custom llama_dir configuration** (as commented-out example after line 84):
   - Add: "# llama_dir = \"./llama\"  # Default: searches ./llama, ./bin, and PATH"
   - Add: "# llama_dir = \"C:/llama.cpp\"  # Windows: custom path"
   - Add: "# llama_dir = \"/usr/local/llama\"  # Unix: custom path"

**Rationale:** The configuration file is often the first place users look when setting up the application. Adding helpful comments with installation links and search path information reduces friction and provides immediate guidance without requiring users to search through documentation.