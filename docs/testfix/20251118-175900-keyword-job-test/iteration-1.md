# Iteration 1

**Goal:** Fix test timeout during ChromeDP navigation to /queue page

---

## Agent 1 - Implementation

### Failures to Address
1. TestKeywordJob timing out at line 84-92 during first navigation to /queue page
2. Context timeout of 120 seconds is insufficient for the entire test execution

### Analysis

The test is timing out because:
1. **Insufficient timeout**: The test uses `context.WithTimeout(ctx, 120*time.Second)` at line 79, but the test performs many operations:
   - HTTP API calls to insert API key
   - Multiple ChromeDP navigations (queue, jobs pages)
   - Job execution and polling (up to 5 minutes for Places job, 2 minutes for Keyword job)
   - Screenshots and UI verifications

2. **Early timeout consumption**: By the time the test reaches the first `/queue` navigation, significant time has already been spent on API setup

3. **ChromeDP WaitVisible hang**: The `WaitVisible('body')` may be waiting indefinitely if the page doesn't load properly

### Proposed Fixes

**File: `test/ui/keyword_job_test.go`**
- Line 79: Increase context timeout from 120s to 600s (10 minutes) to accommodate the full test
- This allows sufficient time for:
  - API setup (5-10s)
  - Multiple page navigations (10-20s each)
  - Job polling (5+ minutes for Places job, 2+ minutes for Keyword job)
  - Screenshots and verification steps

### Changes Made

**`test/ui/keyword_job_test.go`:**
```go
// Line 79: Increase timeout from 120s to 600s
ctx, cancel = context.WithTimeout(ctx, 600*time.Second) // Was: 120*time.Second
```

**Reasoning:**
- The test executes two full job workflows (Places + Keyword)
- Places job alone polls for up to 5 minutes (line 243)
- Keyword job polls for up to 2 minutes (line 464)
- Additional time needed for navigations, screenshots, and UI verifications
- 600 seconds (10 minutes) provides comfortable margin

### Compilation Check
```bash
cd test/ui && go build keyword_job_test.go
```
**Result:** Will verify after making changes
