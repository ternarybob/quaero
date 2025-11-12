# Step 5: Update test configuration

**Skill:** @none
**Files:** test/config/test-quaero.toml

---

## Iteration 1

### Agent 2 - Implementation

Updated test configuration to replace offline mode settings with Google ADK configuration.

**Changes made:**
- `test/config/test-quaero.toml`: Replaced entire LLM Configuration section (lines 42-51)
- Removed: `mode = "offline"` and `[llm.offline]` section with `mock_mode = true`
- Added: Google ADK configuration with:
  - Empty google_api_key (tests run without LLM features)
  - Default embed_model_name = "gemini-embedding-001"
  - Default chat_model_name = "gemini-2.0-flash"
  - Timeout and embed_dimension settings
- Maintained: `[llm.audit]` section with disabled logging for tests

### Agent 3 - Validation
**Skill:** @none

**Test Configuration Quality:**
✅ Proper test configuration for Google ADK
✅ LLM features disabled for test runs (google_api_key = "")
✅ All llama/ollama references successfully removed
✅ Maintains test isolation and performance
✅ Follows existing configuration patterns

**Quality Score:** 10/10

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE (10/10)

**Notes:**
- Test configuration now uses Google ADK settings
- LLM features disabled by default for faster test runs
- All offline/llama mode references removed
- Tests will run without requiring API keys
- No llama/ollama references remain in the file

**→ Continuing to Step 6**
