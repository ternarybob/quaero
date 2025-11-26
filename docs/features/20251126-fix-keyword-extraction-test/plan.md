# Plan: Fix TestQueueWithKeywordExtraction Test

## Analysis

### Problem
The `TestQueueWithKeywordExtraction` test is not properly monitoring the Keyword Extraction job to completion. Looking at the test output:

1. Places job completes successfully (20 documents created)
2. Keyword Extraction job is triggered and starts running
3. Test finds the job in queue and detects initial status "running"
4. Test log ends abruptly without showing job completion or timeout error

### Root Cause Analysis
From the service logs at `11:41:00`, we can see:
- Keyword Extraction job was created with 20 child jobs (`child_count=20`)
- Child jobs were being processed (`pending=19, running=1`)
- Multiple "Failed to type assert job" warnings appear

The test appears to be ending prematurely. Possible causes:
1. **Context cancellation** - The test context may be getting cancelled
2. **Chromedp browser context issue** - The browser may be closing or timing out
3. **Status detection issue** - The DOM selector may not be finding the status correctly for the running job

### Dependencies
- The test depends on both Places and Keyword Extraction jobs working
- Keyword Extraction depends on documents from Places job
- Both require valid API keys (or graceful failure handling)

### Risks
- Test may be flaky due to API rate limits (Gemini)
- Job processing time can vary

## Steps

### Step 1: Add Debug Logging to Monitor Job Function
- Skill: @go-coder
- Files: `test/ui/queue_test.go`
- Critical: no
- Depends: none

Add more verbose logging to understand where the test is stopping:
- Log each poll iteration
- Log DOM state when checking status
- Take periodic screenshots during monitoring

### Step 2: Fix Potential Browser/Context Issues
- Skill: @go-coder
- Files: `test/ui/queue_test.go`
- Critical: no
- Depends: Step 1

Ensure the monitoring loop:
- Properly handles chromedp errors
- Doesn't silently fail on DOM queries
- Takes screenshots at key points

### Step 3: Run Test and Analyze Results
- Skill: @test-writer
- Files: `test/ui/queue_test.go`
- Critical: no
- Depends: Step 2

Execute the test and verify:
- Job monitoring completes (success or proper timeout)
- All screenshots are captured
- Logs show complete monitoring history

## Execution Order
1 -> 2 -> 3 -> Final Validation

## Success Criteria
- Test runs to completion (passes or fails with clear error)
- Keyword Extraction job monitoring shows status progression
- Test logs show complete monitoring cycle
- Screenshots capture job progress
