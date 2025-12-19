# UI Testing & Monitoring Skill for Quaero

**Prerequisite:** Read `.claude/skills/refactoring/SKILL.md` before any code changes.

## Purpose

Patterns for UI testing with browser automation, screenshots, monitoring, and result data output.
Reference implementation: `test/ui/job_definition_general_test.go`

## Project Context

- **Test Framework:** Go testing with testify (assert/require)
- **Browser Automation:** chromedp (Chrome DevTools Protocol)
- **Location:** `test/ui/` for UI tests, `test/api/` for API tests
- **Results:** Screenshots and data saved to `test/results/{run-timestamp}/`

## UITestContext - Core Test Infrastructure

### Creating Test Context

```go
// Always use UITestContext for UI tests
func TestMyFeature(t *testing.T) {
    utc := NewUITestContext(t, 5*time.Minute)  // Set appropriate timeout
    defer utc.Cleanup()  // ALWAYS defer cleanup

    // Test code here
}
```

### Available URLs

```go
utc.BaseURL     // Base server URL
utc.JobsURL     // /jobs page
utc.QueueURL    // /queue page
utc.DocsURL     // /documents page
utc.SettingsURL // /settings page
```

## Screenshots

### Sequential Screenshot Naming

Screenshots are auto-numbered sequentially (01_, 02_, etc.) for clear ordering:

```go
// Basic screenshot (auto-numbered, full page)
utc.Screenshot("queue_page")        // → 01_queue_page.png

// Full page screenshot (same as Screenshot)
utc.FullScreenshot("expanded_view") // → 02_expanded_view.png

// Refresh page then screenshot
utc.RefreshAndScreenshot("final_state")
```

### Screenshot Timing Patterns

```go
// Take screenshots at key moments:
utc.Screenshot("initial_state")

// After navigation
err = utc.Navigate(utc.QueueURL)
utc.Screenshot("after_navigation")

// After action
err = utc.Click(".button")
utc.Screenshot("after_click")

// On error conditions
if err != nil {
    utc.Screenshot("error_state_" + sanitizeName(errorType))
}

// During monitoring (automatic every 30s in MonitorJob)
utc.Screenshot("status_" + currentStatus)
```

## Logging

### Structured Test Logging

All logs go to both console and `test.log` file in results:

```go
// Simple log
utc.Log("Starting test for job: %s", jobName)

// Status changes
utc.Log("Status change: %s -> %s (at %v)", lastStatus, currentStatus, elapsed)

// Assertions passed
utc.Log("✓ ASSERTION PASSED: Filter dropdown with correct structure")

// Warnings (non-fatal issues)
utc.Log("⚠ WARNING: Feature not implemented yet")

// Test completion
utc.Log("Test completed with status: %s", finalStatus)
```

## Saving Result Data

### Save Arbitrary Data to Results

```go
// Save text content to results directory
utc.SaveToResults("captured_data.json", jsonString)
utc.SaveToResults("api_response.txt", responseBody)

// Copy job definition to results
utc.CopyJobDefinitionToResults("job_definitions/my_job.toml")
```

## Navigation & Interaction

### Page Navigation

```go
// Navigate with automatic wait for page load
err := utc.Navigate(utc.QueueURL)
require.NoError(t, err, "Failed to navigate to Queue page")

// Wait for specific element
err = utc.WaitForElement(".job-card", 10*time.Second)
```

### Element Interaction

```go
// Click element
err := utc.Click(".run-button")

// Get text content
text, err := utc.GetText(".status-badge")
```

## Job Monitoring Patterns

### Trigger and Monitor Job

```go
// Trigger job via UI
err := utc.TriggerJob("My Job Name")
require.NoError(t, err, "Failed to trigger job")

// Monitor with options
opts := MonitorJobOptions{
    Timeout:         5 * time.Minute,
    ExpectDocuments: true,
    AllowFailure:    false,  // Set true if job failure is acceptable
}
err = utc.MonitorJob("My Job Name", opts)

// Or use convenience method
err = utc.TriggerAndMonitorJob("My Job Name", 5*time.Minute)
```

### Custom Polling Loop (for complex monitoring)

```go
startTime := time.Now()
jobTimeout := 2 * time.Minute
var finalStatus string

for {
    if time.Since(startTime) > jobTimeout {
        utc.Screenshot("timeout_state")
        break
    }

    // Get status via JavaScript evaluation
    var currentStatus string
    err := chromedp.Run(utc.Ctx,
        chromedp.Evaluate(`
            (() => {
                const card = document.querySelector('.job-card');
                const badge = card?.querySelector('[data-status]');
                return badge?.getAttribute('data-status') || '';
            })()
        `, &currentStatus),
    )

    if currentStatus != "" && currentStatus != finalStatus {
        utc.Log("Status change: %s -> %s", finalStatus, currentStatus)
        utc.Screenshot("status_" + currentStatus)
        finalStatus = currentStatus
    }

    if currentStatus == "completed" || currentStatus == "failed" {
        utc.Log("Job reached terminal state: %s", currentStatus)
        break
    }

    time.Sleep(2 * time.Second)
}
```

## WebSocket & API Tracking

### Track WebSocket Messages

```go
wsTracker := NewWebSocketMessageTracker()

chromedp.ListenTarget(utc.Ctx, func(ev interface{}) {
    switch e := ev.(type) {
    case *network.EventWebSocketFrameReceived:
        payload := e.Response.PayloadData
        if strings.Contains(payload, "refresh_logs") {
            var msg struct {
                Type    string                 `json:"type"`
                Payload map[string]interface{} `json:"payload"`
            }
            if err := json.Unmarshal([]byte(payload), &msg); err == nil {
                wsTracker.AddRefreshLogs(msg.Payload, time.Now())
            }
        }
    }
})

// Enable network tracking
chromedp.Run(utc.Ctx, network.Enable())
```

### Track API Calls

```go
apiTracker := NewAPICallTracker()

chromedp.ListenTarget(utc.Ctx, func(ev interface{}) {
    switch e := ev.(type) {
    case *network.EventRequestWillBeSent:
        apiTracker.AddRequest(e.Request.URL, time.Now())
    }
})

// Later: assert API calls are gated by WebSocket triggers
jobLogs := apiTracker.GetJobLogsCalls()
serviceLogs := apiTracker.GetServiceLogsCalls()
```

## Assertions

### DOM-Based Assertions

```go
var result map[string]interface{}
err := chromedp.Run(utc.Ctx,
    chromedp.Evaluate(`
        (() => {
            return {
                hasElement: document.querySelector('.my-element') !== null,
                elementText: document.querySelector('.my-element')?.textContent || '',
                itemCount: document.querySelectorAll('.list-item').length
            };
        })()
    `, &result),
)
require.NoError(t, err, "Failed to evaluate DOM")

assert.True(t, result["hasElement"].(bool), "Element should exist")
assert.Equal(t, "Expected Text", result["elementText"].(string))
utc.Log("Found %d items", int(result["itemCount"].(float64)))
```

### Step Expansion Tracking

```go
tracker := NewStepExpansionTracker()

// During monitoring, check expansion state
func checkStepExpansionState(utc *UITestContext, tracker *StepExpansionTracker) {
    chromedp.Run(utc.Ctx,
        chromedp.Evaluate(`
            (() => {
                const expanded = [];
                document.querySelectorAll('.tree-step').forEach(step => {
                    const logs = step.querySelector('.tree-step-logs');
                    if (logs) {
                        const name = step.querySelector('.tree-step-name')?.textContent;
                        if (name) expanded.push(name.trim());
                    }
                });
                return expanded;
            })()
        `, &expandedSteps),
    )

    for _, stepName := range expandedSteps {
        tracker.RecordExpansion(stepName)
    }
}

// Assert expansion order
order := tracker.GetExpansionOrder()
utc.Log("Steps expanded in order: %v", order)
```

## Test File Structure

```go
// test/ui/my_feature_test.go
package ui

import (
    "testing"
    "time"

    "github.com/chromedp/chromedp"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMyFeature(t *testing.T) {
    utc := NewUITestContext(t, 5*time.Minute)
    defer utc.Cleanup()

    utc.Log("--- Testing My Feature ---")

    // 1. Setup (create test data via API)
    helper := utc.Env.NewHTTPTestHelper(t)
    // ... create test data ...
    defer helper.DELETE("/api/cleanup/...")

    // 2. Navigate and take initial screenshot
    err := utc.Navigate(utc.QueueURL)
    require.NoError(t, err)
    utc.Screenshot("initial_state")

    // 3. Perform actions
    // ... interact with UI ...

    // 4. Assert results
    var result map[string]interface{}
    chromedp.Run(utc.Ctx, chromedp.Evaluate(`...`, &result))

    assert.True(t, result["expected"].(bool), "Assertion message")
    utc.Log("✓ Test passed")

    // 5. Final screenshot
    utc.Screenshot("final_state")
}
```

## Anti-Patterns (AUTO-FAIL)

```go
// ❌ Missing cleanup
utc := NewUITestContext(t, timeout)
// Missing: defer utc.Cleanup()

// ❌ Hardcoded waits without purpose
time.Sleep(5 * time.Second)  // Why 5 seconds?

// ❌ No screenshots on failure
if err != nil {
    t.Fatal(err)  // Should screenshot first!
}

// ❌ Silent failures in evaluation
chromedp.Evaluate(`...`, nil)  // Ignoring result

// ❌ Missing error checks
utc.Navigate(url)  // Should check error!
```

## Rules Summary

1. **Always defer Cleanup** - `defer utc.Cleanup()` immediately after context creation
2. **Screenshot key moments** - Initial, after actions, on errors, final state
3. **Use structured logging** - `utc.Log()` for all test output
4. **Check all errors** - Every chromedp operation can fail
5. **Use helper methods** - Don't reinvent TriggerJob, MonitorJob, etc.
6. **Save relevant data** - Use SaveToResults for captured data
7. **Follow timeout patterns** - Set appropriate test and job timeouts
