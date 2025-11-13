I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current State Analysis:**

1. **Build Script (scripts/build.ps1):**
   - Contains `Stop-LlamaServers` function (lines 302-338) that kills llama-server processes
   - Function is called on line 516 during service shutdown
   - No longer needed since Google ADK LLM service doesn't use local llama-server processes

2. **AGENTS.md Documentation:**
   - Lines 73-76: Key features list mentions "Local LLM - Offline inference with llama.cpp"
   - Lines 83-86: Technology stack mentions "LLM: llama.cpp (offline mode), Mock mode (testing)"
   - Lines 323-327: Service initialization mentions "offline/mock mode"
   - Lines 354-403: Entire "LLM Service Architecture" section (50 lines) dedicated to llama.cpp, offline mode, binary search order, configuration examples
   - Lines 555: Mentions "Unlike LLM service (which has offline/mock modes), agents require cloud API"
   - Multiple troubleshooting sections reference llama-server installation and verification

3. **README.md Documentation:**
   - Lines 30-32: Server configuration mentions `llama_dir` parameter
   - Lines 67-361: Massive "LLM Setup (Offline Mode)" section (~295 lines) with:
     - Prerequisites for llama-server and models
     - Download instructions for binaries and models
     - Binary placement and verification steps
     - Troubleshooting guide
     - Mode comparison table (offline/mock/cloud)
   - Lines 143-155: Configuration example shows offline mode LLM config
   - Lines 211-213: Security section mentions offline mode as default

4. **Deployment Config (deployments/local/quaero.toml):**
   - Line 30: Comment mentions `llama_dir` in server configuration
   - **Missing**: No LLM configuration section for Google ADK
   - Has Agent configuration section (lines 61-75) which is correct
   - Needs new `[llm]` section with Google API key configuration

5. **Test Config (test/config/test-quaero.toml):**
   - Lines 42-51: Has `[llm]` section with `mode = "offline"` and `mock_mode = true`
   - References old LLM service architecture
   - Needs update to reflect Google ADK LLM service

6. **Chrome Extension README (cmd/quaero-chrome-extension/README.md):**
   - Lines 14-24: "LLM Setup (Offline Mode)" section with llama-server instructions
   - Not critical for extension functionality but should be updated for consistency

**Design Decisions:**

1. **Preserve Embedding Storage Schema**: Keep database schema documentation for `embedding` column and vector search - this is still valid for future use even though current implementation uses Google ADK
2. **Archive vs Delete**: Documentation is being updated, not archived - this is a migration, not a removal
3. **Configuration Examples**: Replace offline mode examples with Google ADK cloud mode examples
4. **Test Configuration**: Update to use Google ADK with empty API key (graceful degradation) instead of mock mode
5. **Consistency**: All documentation should reflect that Google ADK is the **only** LLM provider, not an option among many

### Approach

Remove all llama/ollama references from documentation and configuration files, replacing them with Google ADK-based LLM service documentation. Update build script to remove llama-server process management. Add new LLM configuration section to deployment config showing Google ADK setup. Update test configuration to reflect new LLM service architecture.

### Reasoning

Explored the codebase structure, read all relevant files (build.ps1, AGENTS.md, README.md, quaero.toml, test configs, Chrome extension README), searched for llama/ollama references using grep across markdown, TOML, and PowerShell files, identified all locations requiring cleanup, and confirmed the new Google ADK LLM service implementation is complete and ready to be documented.

## Proposed File Changes

### scripts\build.ps1(MODIFY)

**Remove Stop-LlamaServers function and its invocation:**

**Delete Function Definition (lines 302-338):**
- Remove entire `Stop-LlamaServers` function including:
  - Function declaration and try-catch block
  - Process detection logic for llama-server
  - Process termination loop
  - Verification and logging
  - All 37 lines of the function

**Remove Function Call (line 516):**
- Delete the line `Stop-LlamaServers` that appears after `Stop-QuaeroService -Port $serverPort`
- This call is in the main build flow, right before dependency tidying

**Rationale:**
- Google ADK LLM service uses cloud API, not local llama-server processes
- No local processes to manage or clean up
- Simplifies build script by removing unnecessary process management
- Reduces build time by eliminating process checks

**No other changes needed:**
- Keep `Stop-QuaeroService` function (still needed for main service)
- Keep all other helper functions intact
- Build logic remains unchanged

### AGENTS.md(MODIFY)

References: 

- internal\services\llm\gemini_service.go
- internal\common\config.go

**Update Key Features section (lines 73-76):**
- Change "🤖 **Local LLM** - Offline inference with llama.cpp" to:
  - "🤖 **Cloud LLM** - Google ADK with Gemini models for embeddings and chat"
- Keep all other feature bullets unchanged

**Update Technology Stack section (lines 83-86):**
- Change "**LLM:** llama.cpp (offline mode), Mock mode (testing)" to:
  - "**LLM:** Google ADK with Gemini API (cloud-based embeddings and chat)"
- Keep all other technology stack items unchanged

**Update Service Initialization Flow section (lines 323-327):**
- Change "2. **LLM Service** - Required for embeddings (offline/mock mode)" to:
  - "2. **LLM Service** - Google ADK-based embeddings and chat (cloud mode)"
- Keep all other initialization steps unchanged

**Replace LLM Service Architecture section (lines 354-403):**
- **Delete entire section** (50 lines) including:
  - Modes description (Offline/Mock/Cloud)
  - Current Implementation details about llama.cpp
  - Binary Search Order subsection
  - Configuration example with offline mode
  - References to llama-server binary and model files

- **Replace with new section:**
  - Title: "### LLM Service Architecture"
  - Content:
    - "The LLM service provides embeddings and chat using Google ADK (Agent Development Kit) with Gemini models."
    - "**Implementation:** `internal/services/llm/gemini_service.go` - Google ADK integration"
    - "**Embedding Model:** `gemini-embedding-001` with 768-dimension output (matches database schema)"
    - "**Chat Model:** `gemini-2.0-flash` (fast, cost-effective, same as agent service)"
    - "**No Offline Mode:** Requires Google Gemini API key - no local inference or mock mode available"
    - "**Graceful Degradation:** If API key is missing, LLM service initialization fails with warning but application continues without chat/embedding features"
    - Configuration example:
      ```toml
      [llm]
      google_api_key = "YOUR_GOOGLE_GEMINI_API_KEY"  # Required
      embed_model_name = "gemini-embedding-001"      # Default
      chat_model_name = "gemini-2.0-flash"           # Default
      timeout = "5m"                                  # Operation timeout
      embed_dimension = 768                           # Must match SQLite config
      ```
    - Environment variable overrides:
      - `QUAERO_LLM_GOOGLE_API_KEY` - API key
      - `QUAERO_LLM_EMBED_MODEL_NAME` - Embedding model
      - `QUAERO_LLM_CHAT_MODEL_NAME` - Chat model
      - `QUAERO_LLM_TIMEOUT` - Timeout duration
    - API key setup: "Get API key from: https://aistudio.google.com/app/apikey"
    - Note: "Free tier available with rate limits (15 requests/minute, 1500/day as of 2024)"

**Update Agent Framework section (line 555):**
- Change "Unlike LLM service (which has offline/mock modes), agents require cloud API" to:
  - "Both LLM service and agent service require Google API keys - no offline fallback for either"

**Remove Troubleshooting sections related to llama-server:**
- Search for and remove subsections:
  - "### llama-server Issues" (if exists)
  - "### Installing llama-server Binary" (if exists)
  - "### Verifying Offline Mode" (if exists)
- Keep all other troubleshooting sections (Server Won't Start, UI Tests Fail, Agent Service Issues, etc.)

**Update Storage Schema section (lines 404-422):**
- Keep embedding column documentation as-is
- Add note: "**Note:** Embeddings are generated using Google ADK's Gemini embedding model (768 dimensions)"
- Keep all other schema documentation unchanged

**Rationale:**
- Reflects current architecture with Google ADK as sole LLM provider
- Removes outdated llama.cpp references
- Maintains consistency with agent service documentation
- Keeps embedding storage schema for future use

### README.md(MODIFY)

References: 

- internal\services\llm\gemini_service.go

**Remove entire "LLM Setup (Offline Mode)" section (lines 209-361):**
- Delete section title and all subsections (~153 lines):
  - Security & Privacy paragraph mentioning offline mode
  - Prerequisites subsection
  - Quick Start subsection with binary download instructions
  - Model download instructions
  - Binary placement instructions
  - Verification subsection
  - Troubleshooting subsection
  - Mode comparison table
- This is the largest cleanup in README.md

**Replace with new "LLM Setup (Google ADK)" section:**
- Insert new section at same location (after "Installing Chrome Extension" section)
- Title: "## LLM Setup (Google ADK)"
- Content:
  - "**Cloud-Based LLM:** Quaero uses Google ADK (Agent Development Kit) with Gemini models for embeddings and chat functionality."
  - "**API Key Required:** You must provide a Google Gemini API key for LLM features to work. No offline mode is available."
  - Subsection: "### Getting Your API Key"
    - "1. Visit https://aistudio.google.com/app/apikey"
    - "2. Sign in with your Google account"
    - "3. Create a new API key or use an existing one"
    - "4. Copy the API key for configuration"
  - Subsection: "### Configuration"
    - "Add your API key to `quaero.toml`:"
    - Configuration example:
      ```toml
      [llm]
      google_api_key = "YOUR_GOOGLE_GEMINI_API_KEY"
      ```
    - "Or set via environment variable:"
    - `QUAERO_LLM_GOOGLE_API_KEY=your_key_here ./bin/quaero`
  - Subsection: "### Verification"
    - "Start Quaero and check the startup logs:"
    - "✅ **Success** - Look for: `LLM service initialized successfully`"
    - "❌ **Failure** - If you see: `Failed to initialize LLM service - chat features will be unavailable`"
    - "Check that your API key is valid and properly configured"
  - Subsection: "### Models Used"
    - "**Embedding Model:** `gemini-embedding-001` (768 dimensions)"
    - "**Chat Model:** `gemini-2.0-flash` (fast, cost-effective)"
    - "Models are accessed via Google's API - no local downloads required"
  - Subsection: "### Rate Limits"
    - "**Free Tier:** 15 requests/minute, 1500 requests/day (as of 2024)"
    - "**Paid Tier:** Higher limits available - see Google AI Studio for pricing"
    - "Monitor usage at: https://aistudio.google.com/app/apikey"
  - Subsection: "### Privacy Considerations"
    - "⚠️ **Cloud-Based Processing:** All LLM operations send data to Google's API"
    - "**Not suitable for:** Highly sensitive or confidential data"
    - "**Suitable for:** Development, testing, non-sensitive knowledge bases"
    - "**Data handling:** Review Google's AI Studio terms of service"

**Update Configuration section (lines 143-155):**
- Remove `[llm]` configuration example showing offline mode
- Replace with:
  ```toml
  # LLM configuration (Google ADK)
  [llm]
  google_api_key = "YOUR_GOOGLE_GEMINI_API_KEY"  # Required for embeddings and chat
  ```
- Keep all other configuration sections unchanged

**Update Server Configuration comment (line 30):**
- Remove mention of `llama_dir` from server configuration defaults
- Change "Defaults: port=8080, host=\"localhost\", llama_dir=\"./llama\"" to:
  - "Defaults: port=8080, host=\"localhost\""

**Update Security & Privacy section (if exists around line 211):**
- Remove statements about "Offline mode is the default and recommended configuration"
- Replace with:
  - "**Cloud-Based LLM:** Quaero uses Google ADK for LLM features, which requires API calls to Google's servers"
  - "**Local-First Core:** Document storage, search, and crawling remain 100% local"
  - "**Data Considerations:** LLM features send document content to Google's API for processing"

**Rationale:**
- Removes 150+ lines of outdated llama.cpp documentation
- Provides clear, concise Google ADK setup instructions
- Sets proper expectations about cloud-based processing
- Maintains focus on getting users up and running quickly

### deployments\local\quaero.toml(MODIFY)

References: 

- internal\common\config.go

**Update Server Configuration comment (line 30):**
- Change comment from:
  - `# Defaults: port=8080, host="localhost", llama_dir="./llama"`
- To:
  - `# Defaults: port=8080, host="localhost"`
- Remove `llama_dir` reference as it's no longer used

**Remove QUAERO_SERVER_LLAMA_DIR from environment variables comment (line 32):**
- Change comment from:
  - `# Env vars: QUAERO_SERVER_PORT, QUAERO_SERVER_HOST, QUAERO_SERVER_LLAMA_DIR`
- To:
  - `# Env vars: QUAERO_SERVER_PORT, QUAERO_SERVER_HOST`

**Add new LLM Configuration section (after Agent Configuration section, around line 76):**
- Insert new section with proper spacing and formatting:
  ```toml
  # =============================================================================
  # LLM Configuration (Google ADK with Gemini)
  # =============================================================================
  # Configure LLM service for embeddings and chat using Google ADK.
  # Requires Google Gemini API key from: https://aistudio.google.com/app/apikey
  #
  # IMPORTANT: LLM service requires a valid API key. No offline fallback is available.
  # If the API key is missing or invalid, LLM features (chat, embeddings) will be unavailable.
  #
  # Defaults: embed_model_name="gemini-embedding-001", chat_model_name="gemini-2.0-flash",
  #           timeout="5m", embed_dimension=768
  # Env vars: QUAERO_LLM_GOOGLE_API_KEY, QUAERO_LLM_EMBED_MODEL_NAME,
  #           QUAERO_LLM_CHAT_MODEL_NAME, QUAERO_LLM_TIMEOUT, QUAERO_LLM_EMBED_DIMENSION

  # [llm]
  # google_api_key = "YOUR_GOOGLE_GEMINI_API_KEY"  # Required for LLM operations
  ```

**Rationale:**
- Adds missing LLM configuration section to deployment config
- Follows same pattern as Agent configuration section
- Provides clear documentation about requirements and defaults
- Commented out by default (user must uncomment and add API key)
- Maintains consistency with other configuration sections
- Places LLM section after Agent section for logical grouping (both use Google APIs)

### test\config\test-quaero.toml(MODIFY)

References: 

- internal\services\llm\gemini_service.go
- internal\app\app.go

**Replace LLM Configuration section (lines 42-51):**
- Remove old configuration:
  ```toml
  [llm]
  mode = "offline"

  [llm.offline]
  mock_mode = true  # Use mock responses instead of real llama-server

  [llm.audit]
  enabled = false  # Disable audit logging for tests
  ```

- Replace with new configuration:
  ```toml
  # =============================================================================
  # LLM Configuration (Google ADK with Gemini)
  # =============================================================================
  [llm]
  google_api_key = ""  # Empty by default - LLM features disabled for tests
                       # Override with custom config if testing LLM functionality
  ```

**Update section comment (lines 42-43):**
- Change section title comment to reflect Google ADK architecture
- Add note explaining graceful degradation behavior:
  - "# Empty API key causes LLM service initialization to fail gracefully"
  - "# Application continues without chat/embedding features"
  - "# This is expected behavior for tests that don't require LLM"

**Rationale:**
- Removes references to offline mode and mock_mode
- Aligns test configuration with new Google ADK LLM service
- Empty API key triggers graceful degradation (service logs warning, app continues)
- Tests that need LLM functionality can override with custom config file
- Simplifies test configuration by removing unused audit settings
- Maintains same behavior (LLM disabled by default) with cleaner implementation

### cmd\quaero-chrome-extension\README.md(MODIFY)

**Remove "LLM Setup (Offline Mode)" section (lines 14-24):**
- Delete entire section including:
  - Section title
  - Important note about llama-server requirement
  - Reference to main README.md offline mode section
  - Quick Summary with binary and model locations
  - All 11 lines

**Replace with brief note (optional):**
- Insert concise note at same location:
  - "## LLM Features"
  - "Quaero uses Google ADK with Gemini models for LLM features (embeddings, chat)."
  - "See main `README.md` for LLM setup instructions and API key configuration."

**Alternative: Remove section entirely without replacement:**
- Chrome extension doesn't directly interact with LLM service
- Extension focuses on authentication capture and crawl job creation
- LLM setup is not critical for extension functionality
- Users will find LLM documentation in main README.md when needed

**Recommended approach: Remove without replacement**
- Keeps extension README focused on extension-specific features
- Reduces maintenance burden (one less place to update LLM docs)
- Main README.md is the authoritative source for LLM setup

**No other changes needed:**
- Keep all other sections (Installation, Usage, Features, API Endpoints, Security, Files, Implementation Details)
- Extension functionality is independent of LLM service architecture