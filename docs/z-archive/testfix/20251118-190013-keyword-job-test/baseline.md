# Baseline Test Results

**Test File:** test/ui/keyword_job_test.go
**Test Command:** `cd test/ui && go test -timeout 720s -run "^TestKeywordJob$" -v`
**Timestamp:** 2025-11-18T19:00:13+11:00

## Test Output

```
=== RUN   TestKeywordJob
✓ google_api_key inserted successfully (status: 409)
✓ WebSocket connected (status: ONLINE)
=== PHASE 1: Places Job - SKIPPED (requires Places API Legacy) ===
⚠️  Skipping Phase 1 - requires Places API (Legacy) enablement
=== PHASE 2: Keyword Extraction Agent Job ===
✓ Keyword Extraction job definition created/exists
✓ Keyword Extraction job definition visible in UI
✓ Found execute button with ID: keyword-extraction-demo-run
✓ Keyword Extraction job execution button clicked and dialog accepted
✓ Found Keyword Extraction parent job: 781c26c4
✓ Keyword job appeared in queue
✓ Keyword job status: completed
✅ PHASE 2 PASS: Job executed and status properly displayed in UI
--- PASS: TestKeywordJob (15.00s)
```

## Failures Identified

**Current State:** Test passes but doesn't meet requirements

1. **Issue:** No documents created for keyword extraction to process
   - **Current Behavior:** Phase 1 (Places job) is commented out and skipped
   - **Expected:** Documents should be created in database for keyword job to process
   - **Impact:** Keyword extraction runs with 0 documents (result_count: 0)
   - **Source:** test/ui/keyword_job_test.go:115-361 (commented Phase 1)

2. **Issue:** Test doesn't verify document_count > 0
   - **Current Behavior:** Test checks job status (completed) but not document count
   - **Expected:** Test should verify keyword job creates documents with extracted keywords
   - **Actual:** Job completes with result_count: 0, test still passes
   - **Source:** test/ui/keyword_job_test.go:558-617 (Phase 2 verification)

3. **Issue:** No markdown content for keyword extraction
   - **Current Behavior:** No test documents are inserted into database
   - **Expected:** Test should insert markdown documents for keyword extraction to process
   - **Impact:** Keyword extractor has no content to analyze

## Requirements (from user)

1. **Update Places search** to generate documents in the database (recent implementation)
2. **Insert a test document** containing markdown for keyword job to process
3. **Run keyword job** to generate keywords with Gemini
4. **Pass condition:** document_count > 0 in the keyword queue job

## Technical Analysis

### API Response (Current)
```json
{
  "status": "completed",
  "result_count": 0,
  "child_count": 0,
  "metadata": {
    "job_definition_id": "keyword-extractor-agent"
  }
}
```

The job completes successfully but processes 0 documents because no documents exist in the database.

### Gemini API Connectivity
User confirms Gemini API is accessible via curl:
```bash
curl "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent" \
  -H 'Content-Type: application/json' \
  -H 'X-goog-api-key: AIzaSyA_WWLx4iThpfq0Gc7tOwQ5DRvphC7myzk' \
  -X POST \
  -d '{"contents": [{"parts": [{"text": "Explain how AI works in a few words"}]}]}'
```

User notes: "the gemini connection does not timeout, as tested with curl below. It is a different issue, review https://github.com/googleapis/go-genai and cross check implementation."

## Source Files to Fix

### Primary: `test/ui/keyword_job_test.go`
- **What:** Add test document insertion before keyword extraction
- **Why:** Keyword job needs documents to process
- **How:** Insert markdown documents via HTTP POST /api/documents

### Alternative Approach: Enable Places Job
- **What:** Uncomment and fix Phase 1 (lines 130-361)
- **Why:** Originally intended to create documents via Places API
- **Challenge:** Requires legacy Places API enablement (not available)
- **Recommendation:** Don't use this approach - use direct document insertion instead

## Proposed Solution

**Iteration 1: Insert test documents directly**

1. Add helper function to insert markdown documents via API
2. Insert 2-3 test documents with markdown content before Phase 2
3. Update Phase 2 verification to check `result_count` field
4. Test should fail if `result_count == 0`
5. Test should pass if `result_count > 0`

## Test Statistics

- **Total Tests:** 1 (TestKeywordJob)
- **Passing:** 1 (but doesn't verify requirements)
- **Failing:** 0 (but should fail until documents are verified)
- **Skipped:** 0

**Status:** Test passes superficially but doesn't verify core requirement (document creation)

**→ Starting Iteration 1**
