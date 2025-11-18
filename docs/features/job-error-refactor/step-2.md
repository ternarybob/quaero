# Step 2: Design new test structure for two-job scenario

**Skill:** @code-architect
**Files:** test/ui/job_error_display_simple_test.go

---

## Iteration 1

### Agent 2 - Implementation

Designed the new test structure combining UI verification (ChromeDP) with API-based job execution patterns:

**Test Architecture:**

```
TestJobErrorDisplay_Simple
├── Setup Phase
│   ├── Initialize test environment (common.SetupTestEnvironment)
│   ├── Create HTTPTestHelper for API calls
│   └── Setup ChromeDP context for UI verification
│
├── Phase 1: Places Job (Document Creation)
│   ├── Create job definition (places-nearby-restaurants)
│   ├── Execute job via API
│   ├── Navigate to /queue page with ChromeDP
│   ├── Wait for WebSocket connection
│   ├── Poll for job appearance in UI
│   ├── Wait for job completion (polling job status via API)
│   ├── Verify documents created (document_count > 0)
│   ├── Take screenshots showing success
│   └── PASS if documents created
│
├── Phase 2: Keyword Agent Job (Expected Failure)
│   ├── Create job definition (keyword-extractor-agent)
│   ├── Execute job via API
│   ├── Wait for job to appear in queue UI
│   ├── Poll for job status via API
│   ├── Expect job to fail (missing Gemini API key)
│   ├── Verify error display in UI (chromedp.Poll for error elements)
│   ├── Take screenshots showing error state
│   └── PASS if error properly displayed
│
└── Cleanup
    └── env.Cleanup() via defer
```

**Key Design Decisions:**

1. **Hybrid approach:** Use API for job execution/polling (more reliable) + UI for verification (what we're testing)

2. **Helper functions to add:**
   - `pollForJobCompletionWithUI()` - combines API polling with UI updates
   - `waitForJobInUI()` - waits for specific job ID to appear in queue
   - `verifyErrorDisplay()` - checks for error indicators in UI

3. **Screenshot strategy:**
   - `phase1-queue-initial.png` - Initial queue state
   - `phase1-job-running.png` - Places job executing
   - `phase1-job-complete.png` - Places job completed with documents
   - `phase2-job-running.png` - Keyword job executing
   - `phase2-error-display.png` - Keyword job error display

4. **Error verification approach:**
   - Check for error badge/indicator in job card
   - Look for error text in HTML (case-insensitive "error", "failed")
   - Verify job status shows "failed" in UI
   - Confirm error details are visible (not just status change)

5. **Job definitions:**
   - Use exact definitions from test/config/job-definitions/*.toml
   - Places: 20 restaurants, 2km radius, Wheelers Hill
   - Keyword: agent_type "keyword_extractor", limit 100 docs

6. **Timeouts:**
   - Places job: 5 minutes (same as API test)
   - Keyword job: 10 minutes (same as API test, though will fail quickly)
   - UI polling: 30 seconds for job appearance
   - WebSocket: 10 seconds for connection

**Changes made:**
- Designed complete test flow
- Identified helper functions needed
- Planned screenshot and verification strategy
- Documented timeout values

**Commands run:**
None (design phase)

### Agent 3 - Validation
**Skill:** @code-architect

**Compilation:**
⚙️ Not applicable (design phase)

**Tests:**
⚙️ Not applicable (design phase)

**Code Quality:**
✅ Well-structured test flow with clear phases
✅ Hybrid API+UI approach is pragmatic and testable
✅ Screenshot strategy provides good observability
✅ Timeout values match existing test patterns
✅ Helper functions clearly defined
✅ Error verification approach is comprehensive

**Quality Score:** 9/10

**Issues Found:**
None - solid architectural design

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 9/10

**Notes:**
Design is clear and follows existing test patterns. Ready for implementation. The hybrid approach (API for reliability, UI for verification) is the right choice for this test.

**→ Continuing to Step 3**
