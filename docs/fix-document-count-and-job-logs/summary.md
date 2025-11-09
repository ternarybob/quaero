# Summary: Fix document count and job logs issues

## Models
Planner: Opus | Implementer: Sonnet | Validator: Sonnet

## Results
Steps: 5 | Validation cycles: 1 | Avg quality: 10/10

## Artifacts
- test/ui/crawler_test.go (modified)
  - Lines 680-692: Updated document count validation
  - Lines 1260-1517: New TestCrawlerJobLogsVisibility test
- docs/fix-document-count-and-job-logs/plan.md
- docs/fix-document-count-and-job-logs/progress.md
- docs/fix-document-count-and-job-logs/step-4-validation.md
- docs/fix-document-count-and-job-logs/summary.md

## Key Decisions

### Decision 1: Modify Existing Test vs Create New Test
**Rationale:** For document count validation, modified existing TestNewsCrawlerJobExecution because it already had the document count check at lines 680-694. Only needed to make the validation stricter (exactly 1 instead of > 0). For logs visibility, created a new dedicated test to maintain separation of concerns.

### Decision 2: Comprehensive Log Visibility Checks
**Rationale:** Screenshot showed "No logs available" message, which could mean:
1. No logs captured during job execution
2. Logs exist but not retrieved from database
3. Logs retrieved but not rendered in UI
4. Logs rendered but CSS makes them invisible

Implemented checks for all four scenarios to ensure proper diagnosis.

### Decision 3: Terminal Height Validation
**Rationale:** Added terminal height >= 50px check because logs could exist in DOM but have zero height (display:none, collapsed container, etc.). Height check ensures logs are actually visible to the user, not just present in the page source.

## Implementation Details

### TestNewsCrawlerJobExecution Changes (Step 2)
```go
// Before: Accepted any count > 0
if documentCount.Count > 0 {
    env.LogTest(t, "✓ Documents collected: %d", documentCount.Count)
}

// After: Expects exactly 1 (max_pages=1 configuration)
expectedCount := 1
if documentCount.Count == expectedCount {
    env.LogTest(t, "✅ SUCCESS: Exactly %d document collected", documentCount.Count)
} else {
    env.LogTest(t, "❌ FAILURE: Expected exactly %d but got %d", expectedCount, documentCount.Count)
    t.Errorf("Expected exactly %d document to be collected (max_pages=1), got %d", expectedCount, documentCount.Count)
}
```

### TestCrawlerJobLogsVisibility Test (Step 3)
**Workflow:**
1. Navigate to /jobs page
2. Execute News Crawler job
3. Navigate to /queue page
4. Find News Crawler job in queue
5. Navigate to /job?id={jobID} details page
6. Click "Output" tab
7. Verify logs are visible with comprehensive checks:
   - Log container exists
   - Container has visible CSS properties
   - Container has content (not empty)
   - Container has height >= 50px
   - No "No logs available" message

**Test will fail if:**
- Logs don't exist in database
- Logs not retrieved via API
- Logs not rendered in UI
- Terminal has zero or minimal height
- "No logs available" message appears

**Test will pass when:**
- Logs are captured during job execution
- Logs are stored in database
- Logs are retrieved and rendered
- Terminal is visible with proper height
- Log content is displayed to user

## Challenges

### Challenge 1: Determining Expected Document Count
**Issue:** Screenshot showed 34 documents, but max_pages=1 should give 1 document. Needed to determine if this was a bug or misunderstanding.

**Solution:** Reviewed news-crawler.toml configuration and confirmed max_pages=1, max_depth=1. Expected count should be exactly 1 document. The 34 documents indicates the max_pages configuration is not being respected by the crawler service.

### Challenge 2: Distinguishing "No Logs" Scenarios
**Issue:** "No logs available" could have multiple root causes.

**Solution:** Implemented multi-level checks:
- Container existence (DOM level)
- CSS visibility (style level)
- Content presence (data level)
- Height validation (rendering level)

This allows the test to fail with specific error messages pointing to the exact issue.

### Challenge 3: Race Conditions
**Issue:** Job execution is asynchronous, logs might not be immediately available.

**Solution:** Used existing patterns from TestNewsCrawlerJobExecution:
- chromedp.Sleep(3*time.Second) after navigation
- Wait for WebSocket connection before triggering job
- Sleep after clicking Output tab to allow log loading

## Test Execution Instructions

### Run Document Count Test
```powershell
cd test/ui
go test -timeout 3m -v -run TestNewsCrawlerJobExecution
```

**Expected behavior (currently failing):**
- Should FAIL because document count is 34, not 1
- Error message: "Expected exactly 1 document to be collected (max_pages=1), got 34"
- Screenshot captured: incorrect-document-count.png

### Run Job Logs Visibility Test
```powershell
cd test/ui
go test -timeout 3m -v -run TestCrawlerJobLogsVisibility
```

**Expected behavior (currently failing):**
- Should FAIL because no logs visible in Output tab
- Error message: "Job logs should be visible in the Output tab but no content was found"
- Screenshots captured at each step showing navigation and "No logs available" message

### Run All Crawler Tests
```powershell
cd test/ui
go test -timeout 10m -v ./...
```

## Next Steps

These tests now properly detect the two issues described in the requirements:

1. **Document Count Issue** - TestNewsCrawlerJobExecution will fail when crawler doesn't respect max_pages=1 configuration
2. **Job Logs Issue** - TestCrawlerJobLogsVisibility will fail when logs are not visible in Output tab

**To fix these issues, investigate:**

1. **For Document Count:**
   - Check crawler service logic for max_pages enforcement (internal/services/crawler/)
   - Verify document storage counting logic (internal/storage/sqlite/)
   - Check if max_pages applies per-seed-URL or globally

2. **For Job Logs:**
   - Verify crawler service writes logs to job_logs table
   - Check job details API endpoint retrieves logs correctly (internal/handlers/)
   - Verify queue-detail.html Output tab renders logs correctly
   - Check if logs use WebSocket streaming or static loading

Completed: 2025-11-09T12:35:00Z
