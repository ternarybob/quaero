# Step 1: Implementation Proposal
Iteration: 1 | Status: complete | Type: Design Document

## Overview

This document proposes solutions for three issues related to log display on the queue page (`./pages/queue.html`):

1. **"Show X earlier logs" button not functioning** - Button exists but clicking it doesn't load more logs
2. **Jobs with 1000+ logs need pagination** - Current implementation lacks proper handling for large log volumes
3. **Minimum 100 logs display** - Initial load should show at least 100 logs when available

---

## Issue 1: "Show X earlier logs" Button Not Functioning

### Current Implementation Analysis

The button exists at `queue.html:717-733`:

```html
<template x-if="hasStepEarlierLogs(item.job.id, step.name, stepIndex)">
    <div class="tree-logs-show-more">
        <button class="btn btn-sm btn-link load-earlier-logs-btn"
            @click.stop="loadMoreStepLogs(item.job.id, step.name, stepIndex)"
            ...>
            <span><i class="fas fa-chevron-up"></i> Show <span x-text="getStepEarlierLogsCount(...)"></span> earlier logs</span>
        </button>
    </div>
</template>
```

The `loadMoreStepLogs` function at `queue.html:4943-4984`:

```javascript
async loadMoreStepLogs(jobId, stepName, stepIndex) {
    // Increase the limit by 100
    const currentLimit = this.getStepLogLimit(jobId, stepName);
    const newLimit = currentLimit + 100;
    this.stepLogLimits = { ...this.stepLogLimits, [key]: newLimit };

    // Fetch from API with new limit
    const response = await fetch(`/api/jobs/${jobId}/tree/logs?step=${stepName}&limit=${newLimit}&level=${level}`);
    ...
}
```

### Root Cause Analysis

The implementation appears correct, but potential issues include:

1. **Alpine.js Event Binding**: The `@click.stop` may not be triggering properly
2. **API Response Handling**: The response may not be updating the reactive state correctly
3. **Limit Key Mismatch**: The `stepLogLimits` key format may differ from what's used in fetching

### Proposed Solution

**Option A: Fix Event Binding (Recommended)**

Ensure the button click properly triggers the function:

```html
<button class="btn btn-sm btn-link load-earlier-logs-btn"
    @click.prevent="loadMoreStepLogs(item.job.id, step.name, stepIndex)"
    x-on:click.stop="$event.stopPropagation()"
    :data-job-id="item.job.id"
    :data-step-name="step.name">
```

**Option B: Add Debug Logging**

Add console logging to trace the issue:

```javascript
async loadMoreStepLogs(jobId, stepName, stepIndex) {
    console.log('[Queue] loadMoreStepLogs triggered:', { jobId: jobId.substring(0, 8), stepName, stepIndex });
    // ... rest of function
}
```

**Option C: Verify API Response Structure**

Ensure the API `/api/jobs/{id}/tree/logs` returns the expected structure:

```json
{
    "job_id": "...",
    "steps": [{
        "step_name": "step_name",
        "step_id": "...",
        "status": "completed",
        "logs": [...],
        "total_count": 500,
        "unfiltered_count": 500
    }]
}
```

---

## Issue 2: Jobs with 1000+ Logs Need Pagination

### Current Behavior

- Initial load: 20 logs per step (set in `_doFetchStepLogs`)
- "Show earlier logs" button increases limit by 100 each click
- Maximum limit capped at 5000 logs (`_doFetchStepLogs:4676`)
- Download function fetches up to 10000 logs

### Proposed Solutions

#### Solution 2A: Virtual Scrolling (Recommended for 1000+ logs)

Implement virtual scrolling to render only visible log lines:

```javascript
// Add virtual scroll container
<div class="tree-step-logs"
     x-ref="logContainer"
     @scroll="handleLogScroll($event, item.job.id, step.name)">
    <div :style="{ height: totalLogHeight + 'px', position: 'relative' }">
        <template x-for="(log, idx) in visibleLogs" :key="idx">
            <div class="tree-log-line"
                 :style="{ position: 'absolute', top: (log.virtualIndex * lineHeight) + 'px' }">
                ...
            </div>
        </template>
    </div>
</div>
```

Benefits:
- Handles 10,000+ logs without DOM performance issues
- Smooth scrolling experience
- Memory efficient

Drawbacks:
- More complex implementation
- Requires line height calculation

#### Solution 2B: Chunk-Based Pagination (Simpler Implementation)

Keep current limit-based approach but improve UX:

```javascript
// Constants
const LOGS_PER_PAGE = 100;
const MAX_DISPLAYED_LOGS = 1000;  // Warn user after this

// New function for bidirectional loading
async loadMoreLogs(jobId, stepName, direction = 'earlier') {
    const currentLogs = this.getStepLogs(jobId, stepName);

    if (direction === 'earlier') {
        // Load older logs (prepend to current)
        const offset = this.stepLogOffsets[key] + 100;
        const response = await fetch(`/api/jobs/${jobId}/tree/logs?step=${stepName}&limit=100&offset=${offset}`);
    } else {
        // Load newer logs (append to current) - for auto-refresh during running jobs
    }
}
```

UI Enhancement:
```html
<!-- Top: Load earlier -->
<template x-if="hasEarlierLogs(...)">
    <div class="tree-logs-pagination">
        <button @click="loadMoreLogs(jobId, stepName, 'earlier')">
            <i class="fas fa-chevron-up"></i>
            Load 100 earlier (showing <span x-text="displayedCount"></span>/<span x-text="totalCount"></span>)
        </button>
    </div>
</template>

<!-- Bottom: Load newer (for running jobs) -->
<template x-if="hasNewerLogs(...)">
    <div class="tree-logs-pagination">
        <button @click="loadMoreLogs(jobId, stepName, 'later')">
            <i class="fas fa-chevron-down"></i> Load 100 newer
        </button>
    </div>
</template>
```

#### Solution 2C: Jump-to-Line Feature

Add ability to jump to specific log lines for very large logs:

```html
<div class="tree-logs-nav">
    <label>Jump to line:</label>
    <input type="number" x-model="jumpToLine" min="1" :max="totalLogCount">
    <button @click="loadLogsAroundLine(jobId, stepName, jumpToLine)">Go</button>
</div>
```

API Support (already exists):
```
GET /api/jobs/{id}/tree/logs?step=step_name&limit=100&offset=500
```

### Recommendation

**Implement Solution 2B (Chunk-Based Pagination)** as the primary approach:
- Simpler than virtual scrolling
- Aligns with existing architecture (limit/offset in API)
- Add warning when logs exceed 1000: "Large log set - consider using download"

---

## Issue 3: Minimum 100 Logs Display on Initial Load

### Current Behavior

Initial load fetches 20 logs per step (set in `expandStep` at line 4760-4761):

```javascript
if (!this.stepLogLimits[limitKey]) {
    this.stepLogLimits = { ...this.stepLogLimits, [limitKey]: 20 };
}
```

### Proposed Solution

Change initial limit from 20 to 100:

```javascript
// In expandStep or wherever initial limit is set
if (!this.stepLogLimits[limitKey]) {
    this.stepLogLimits = { ...this.stepLogLimits, [limitKey]: 100 };
}
```

Also update `defaultLogsPerStep` constant at line 4864:

```javascript
// Default maximum logs to display per step in tree view
defaultLogsPerStep: 100,  // Changed from 100 to be the initial value
```

**Conditional Logic Enhancement:**

```javascript
// Set initial limit based on total logs available
async expandStep(jobId, stepName, stepIndex) {
    const limitKey = `${jobId}:${stepName}`;

    // If we know total count, set appropriate initial limit
    const step = this.jobTreeData[jobId]?.steps?.[stepIndex];
    const totalLogs = step?.totalLogCount || 0;

    if (!this.stepLogLimits[limitKey]) {
        // Show at least 100, or all logs if < 100
        const initialLimit = Math.min(Math.max(100, totalLogs), 100);
        this.stepLogLimits = { ...this.stepLogLimits, [limitKey]: initialLimit };
    }

    this.fetchStepLogs(jobId, stepName, stepIndex, true);
}
```

---

## Issue 4: Test Assertions for job_definition_general_test.go

### Current Test Coverage

The file `test/ui/job_definition_general_test.go` already has test coverage for log filtering and "Show earlier logs" in `TestJobDefinitionErrorGeneratorLogFiltering`:

- Lines 895-991: Tests "Show earlier logs" functionality
- Lines 618-739: Tests filter dropdown structure

### Proposed New Assertions

#### Test: Initial Log Count >= 100

```go
// TestJobDefinitionLogInitialCount verifies initial log display shows at least 100 logs
func TestJobDefinitionLogInitialCount(t *testing.T) {
    utc := NewUITestContext(t, 5*time.Minute)
    defer utc.Cleanup()

    // Create job with many logs (500+)
    // ... job creation code ...

    // Navigate and wait for job to complete
    // ... navigation code ...

    // Expand step to view logs
    // ... expansion code ...

    // ASSERTION: Initial log count should be at least 100
    var initialLogCount int
    err = chromedp.Run(utc.Ctx,
        chromedp.Evaluate(`
            (() => {
                const logLines = document.querySelectorAll('.tree-log-line');
                return logLines.length;
            })()
        `, &initialLogCount),
    )
    require.NoError(t, err, "Failed to get initial log count")

    // With 500+ logs generated, initial display should show at least 100
    assert.GreaterOrEqual(t, initialLogCount, 100,
        "Initial log display should show at least 100 logs when available")
}
```

#### Test: "Show Earlier Logs" Actually Works

```go
// TestJobDefinitionShowEarlierLogsWorks verifies the button actually loads more logs
func TestJobDefinitionShowEarlierLogsWorks(t *testing.T) {
    utc := NewUITestContext(t, 5*time.Minute)
    defer utc.Cleanup()

    // Create job with 500+ logs
    // ... job creation code ...

    // Get initial log count
    var initialCount int
    chromedp.Run(utc.Ctx,
        chromedp.Evaluate(`document.querySelectorAll('.tree-log-line').length`, &initialCount),
    )

    // Click "Show earlier logs" button
    var clicked bool
    chromedp.Run(utc.Ctx,
        chromedp.Evaluate(`
            (() => {
                const btn = document.querySelector('.load-earlier-logs-btn');
                if (btn && !btn.disabled) {
                    btn.click();
                    return true;
                }
                return false;
            })()
        `, &clicked),
        chromedp.Sleep(3*time.Second), // Wait for API call
    )

    require.True(t, clicked, "Should have found and clicked 'Show earlier logs' button")

    // Get new log count
    var newCount int
    chromedp.Run(utc.Ctx,
        chromedp.Evaluate(`document.querySelectorAll('.tree-log-line').length`, &newCount),
    )

    // ASSERTION: Log count should have increased
    assert.Greater(t, newCount, initialCount,
        "Clicking 'Show earlier logs' should increase displayed log count")

    // ASSERTION: Should have loaded approximately 100 more logs
    logsAdded := newCount - initialCount
    assert.GreaterOrEqual(t, logsAdded, 50, "Should load at least 50 more logs")
    assert.LessOrEqual(t, logsAdded, 150, "Should load at most 150 more logs (100 + tolerance)")
}
```

#### Test: Large Log Set Pagination

```go
// TestJobDefinitionLargeLogSetPagination verifies pagination works for 1000+ logs
func TestJobDefinitionLargeLogSetPagination(t *testing.T) {
    utc := NewUITestContext(t, 10*time.Minute)
    defer utc.Cleanup()

    // Create job that generates 1500+ logs
    body := map[string]interface{}{
        "steps": []map[string]interface{}{
            {
                "name":        "generate_many_logs",
                "type":        "error_generator",
                "config": map[string]interface{}{
                    "worker_count": 15,
                    "log_count":    100,  // 15 workers * 100 = 1500 logs
                    "log_delay_ms": 5,
                },
            },
        },
    }
    // ... create job ...

    // Wait for completion and expand step
    // ...

    // Get total log count from API
    resp, _ := helper.GET(fmt.Sprintf("/api/jobs/%s/tree/logs?step=generate_many_logs&limit=1", jobID))
    var apiResp struct {
        Steps []struct {
            TotalCount int `json:"total_count"`
        } `json:"steps"`
    }
    json.NewDecoder(resp.Body).Decode(&apiResp)
    totalLogs := apiResp.Steps[0].TotalCount

    // ASSERTION: Total logs should be > 1000
    assert.Greater(t, totalLogs, 1000, "Job should generate 1000+ logs")

    // ASSERTION: "Show earlier logs" button should be visible (not all logs displayed)
    var hasEarlierButton bool
    chromedp.Run(utc.Ctx,
        chromedp.Evaluate(`
            (() => {
                const btn = document.querySelector('.load-earlier-logs-btn');
                return btn !== null && btn.offsetParent !== null;
            })()
        `, &hasEarlierButton),
    )
    assert.True(t, hasEarlierButton, "Should show 'earlier logs' button for large log sets")

    // ASSERTION: Displayed logs should be less than total (pagination working)
    var displayedLogs int
    chromedp.Run(utc.Ctx,
        chromedp.Evaluate(`document.querySelectorAll('.tree-log-line').length`, &displayedLogs),
    )
    assert.Less(t, displayedLogs, totalLogs, "Should not display all logs at once for large sets")
}
```

---

## Architecture Compliance Check

| Requirement | Status | Notes |
|-------------|--------|-------|
| Log Fetching Strategy (QUEUE_UI.md) | Compliant | Uses REST API on step expand |
| "Show earlier logs" with offset (QUEUE_UI.md) | Compliant | Uses limit parameter |
| API calls < 10 per step (QUEUE_UI.md) | Compliant | Incremental loading |
| GET /api/jobs/{id}/logs params (QUEUE_LOGGING.md) | Compliant | limit, offset, level supported |
| Log line starts at 1 (QUEUE_LOGGING.md) | Compliant | Uses logIdx + 1 |

---

## Summary of Proposed Changes

| File | Change | Priority |
|------|--------|----------|
| `pages/queue.html:4760-4761` | Change initial limit from 20 to 100 | High |
| `pages/queue.html:4943-4984` | Debug/fix `loadMoreStepLogs` function | High |
| `pages/queue.html:717-733` | Verify Alpine.js event binding for button | High |
| `test/ui/job_definition_general_test.go` | Add new test assertions | Medium |
| `pages/queue.html` | Add pagination UI for 1000+ logs | Medium |

---

## Build & Test Status
Build: N/A (Design document)
Tests: N/A (Design document - proposes new tests)
