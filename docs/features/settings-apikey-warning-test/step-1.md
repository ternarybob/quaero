# Step 1: Analyze existing test patterns and config setup

**Skill:** @none
**Files:**
- test/ui/settings_apikeys_test.go
- test/ui/jobs_agent_disabled_test.go
- test/config/test-quaero-no-variables.toml
- test/config/test-quaero-apikeys.toml

---

## Iteration 1

### Agent 2 - Implementation

Analyzed existing test patterns and configuration setup to understand how to properly implement the new tests.

**Key Findings:**

1. **Test Pattern from `settings_apikeys_test.go`:**
   - Uses `common.SetupTestEnvironment()` with custom config path
   - Example: `env, err := common.SetupTestEnvironment("SettingsAPIKeysLoad", "../config/test-quaero-apikeys.toml")`
   - Navigates to settings with query param: `url := env.GetBaseURL() + "/settings?a=auth-apikeys"`
   - Uses chromedp for browser automation
   - Takes screenshots at key verification points
   - Uses `chromedp.Evaluate()` to check for DOM elements and extract data
   - Waits for Alpine.js to load with `chromedp.Sleep(2*time.Second)`

2. **Test Pattern from `jobs_agent_disabled_test.go`:**
   - Tests the "Configuration Required" warning display
   - Uses `quaero-no-ai.toml` config to disable agent service
   - Loads test job definition: `env.LoadJobDefinitionFile("../config/job-definitions/test-agent-job.toml")`
   - Navigates to `/jobs` page
   - Uses JavaScript evaluation to extract job card details:
     ```javascript
     const cards = Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card'));
     const agentCard = cards.find(card => card.textContent.includes('Test Keyword Extraction'));
     ```
   - Checks for `runtime_status === 'disabled'` and `runtime_error` message
   - Verifies "Configuration Required" warning in toast element: `.toast.toast-error`
   - Checks that buttons are disabled when runtime_status is 'disabled'

3. **Config File Structure (`test-quaero-no-variables.toml`):**
   ```toml
   environment = "development"
   [server]
   port = 18085  # Test port (UI tests)
   [storage.sqlite]
   reset_on_startup = true
   [variables]
   dir = "./variables-no-variables"  # Empty variables directory
   [agent]
   google_api_key = ""  # Disabled for tests
   [llm]
   google_api_key = ""  # Disabled for tests
   ```

4. **Config File Structure (`test-quaero-apikeys.toml`):**
   - Similar to no-variables but uses `./variables` directory
   - Contains `test-keys.toml` with pre-populated API keys

5. **Warning Display Logic (from `pages/jobs.html` lines 115-122):**
   ```html
   <template x-if="jobDef.runtime_status === 'disabled' && jobDef.runtime_error">
       <div class="toast toast-error" style="margin-top: 0.8rem; ...">
           <i class="fas fa-exclamation-circle"></i>
           <strong>Configuration Required:</strong>
           <span x-text="jobDef.runtime_error"></span>
       </div>
   </template>
   ```

6. **Important Behavioral Note:**
   - Runtime validation happens on the server when listing job definitions
   - Setting `agent.google_api_key` in TOML affects whether agent service initializes
   - Adding keys via KV storage (settings page) does NOT immediately affect runtime status
   - Runtime status is set based on service availability at startup

**Test Strategy Clarification:**

Based on analysis, the tests should verify:
1. **Scenario 1:** `agent.google_api_key = ""` in TOML → Warning IS displayed
2. **Scenario 2:** `agent.google_api_key = "test-value"` in TOML → Warning is NOT displayed
3. **Scenario 3:** Add key via settings UI → Key is saved to KV storage (but runtime status doesn't change without restart)

**Files/Directories Identified:**
- Test file to create: `test/ui/settings_apikey_warning_test.go`
- Config file to create: `test/config/test-quaero-with-apikey.toml`
- Variables directory structure: `test/config/variables-no-variables/` (already exists per config)

### Agent 3 - Validation
**Skill:** @none

**Analysis Quality:**
✅ Thorough analysis of existing test patterns
✅ Correct understanding of config file structure
✅ Identified key DOM selectors and JavaScript patterns
✅ Clear behavioral understanding of runtime validation
✅ Identified all necessary files and patterns

**Documentation:**
✅ Clear findings documented
✅ Code examples provided
✅ Test strategy defined
✅ Files identified

**Quality Score:** 10/10

**Issues Found:** None

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
Analysis complete. Ready to proceed with test implementation. Key insights:
- Use existing test patterns from `settings_apikeys_test.go` and `jobs_agent_disabled_test.go`
- Runtime validation is server-side and tied to service initialization
- Warning display controlled by `runtime_status` and `runtime_error` fields
- Need to test both TOML-based and UI-based key management

**→ Continuing to Step 2**
