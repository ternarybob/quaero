# Step 2: Add agent configuration to config files

**Skill:** @go-coder
**Files:** `internal/common/config.go`, `deployments/local/quaero.toml`

---

## Iteration 1

### Agent 2 - Implementation
Added agent configuration structure and environment variable support to the config system. Created AgentConfig struct with Google API key, model name, max turns, and timeout fields. Integrated into main Config struct and added [agent] section to quaero.toml with comprehensive documentation.

**Changes made:**
- `internal/common/config.go`: Added `AgentConfig` struct with 4 fields (GoogleAPIKey, ModelName, MaxTurns, Timeout)
- `internal/common/config.go`: Added `Agent AgentConfig` field to main `Config` struct
- `internal/common/config.go`: Added default values in `NewDefaultConfig()` (empty API key, gemini-2.0-flash model, 10 max turns, 5m timeout)
- `internal/common/config.go`: Added 4 environment variable overrides in `applyEnvOverrides()` (QUAERO_AGENT_GOOGLE_API_KEY, etc.)
- `deployments/local/quaero.toml`: Added `[agent]` configuration section with documentation and commented defaults

**Commands run:**
```bash
cd cmd/quaero && go build -o /tmp/quaero
```

### Agent 3 - Validation
**Skill:** @go-coder

**Compilation:**
✅ Compiles cleanly - no errors or warnings

**Tests:**
⚙️ No tests applicable - configuration changes only

**Code Quality:**
✅ AgentConfig struct follows existing pattern (matches PlacesAPIConfig)
✅ Environment variable overrides follow existing convention
✅ Default values are sensible (gemini-2.0-flash, 10 turns, 5m timeout)
✅ TOML documentation is comprehensive and clear
✅ Includes important warning about no offline fallback
✅ API key defaults to empty string (user must provide)

**Quality Score:** 10/10

**Issues Found:**
None - configuration integrates seamlessly with existing config system

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
Successfully integrated agent configuration into Quaero's config system. The AgentConfig struct provides all necessary parameters for Google ADK integration: API key (required), model selection, conversation turn limits, and execution timeouts. Environment variable overrides enable flexible deployment configuration. The TOML documentation clearly warns users that agents require a valid API key with no offline fallback.

**→ Continuing to Step 3**
