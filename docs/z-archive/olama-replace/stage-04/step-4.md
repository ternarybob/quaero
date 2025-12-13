# Step 4: Update deployment configuration

**Skill:** @none
**Files:** deployments/local/quaero.toml

---

## Iteration 1

### Agent 2 - Implementation

Updated deployment configuration to remove llama-server references and add Google ADK documentation.

**Changes made:**
- `deployments/local/quaero.toml`: Removed `llama_dir` reference from Server Configuration comment (line 30)
- `deployments/local/quaero.toml`: Removed `QUAERO_SERVER_LLAMA_DIR` from environment variables list (line 32)
- `deployments/local/quaero.toml`: Added comprehensive LLM Configuration section (lines 76-95) with:
  - Google ADK configuration documentation
  - API key requirements and setup URL
  - Default values for models and timeout
  - Environment variable overrides
  - Important notes about API key requirements

### Agent 3 - Validation
**Skill:** @none

**Configuration Quality:**
✅ Clean removal of llama_dir references
✅ Comprehensive LLM configuration documentation
✅ Clear environment variable documentation
✅ Proper section structure following existing pattern
✅ All llama/ollama references successfully removed

**Quality Score:** 10/10

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE (10/10)

**Notes:**
- Deployment configuration now documents Google ADK LLM setup
- Removed all llama-server related configuration references
- Added detailed LLM configuration section for users
- No llama/ollama references remain in the file

**→ Continuing to Step 5**
