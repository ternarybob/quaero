# Plan: Fix TestNewsCrawlerCrash Deadlock

## Classification
- Type: fix
- Workdir: ./docs/fix/20251129-test-queue-crash/

## Analysis

### Root Cause
The test fails with a deadlock in `verifyServiceConnectivity()` at `test/ui/main_test.go:99`. The issue is:

1. `chromedp.Cancel(browserCtx)` is called in a deferred function
2. The browser context may already be cancelled due to the timeout context
3. `chromedp.Cancel()` attempts to execute a command on an already-closed browser, causing a deadlock

The call to `chromedp.Cancel(browserCtx)` at line 99 blocks forever because:
- The browser context's internal channels are already closed
- The `execute()` function waits indefinitely on a select that will never receive

### Solution
Remove the explicit `chromedp.Cancel()` call. The `cancelBrowser()` function already properly cleans up the context. Calling `chromedp.Cancel()` explicitly before `cancelBrowser()` is redundant and causes the deadlock.

### Risks
- Low risk: This is a straightforward cleanup pattern fix
- The change only affects the pre-test connectivity check, not the actual test code

## Groups
| Task | Desc | Depends | Critical | Complexity | Model |
|------|------|---------|----------|------------|-------|
| 1 | Remove chromedp.Cancel call in verifyServiceConnectivity | none | no | low | sonnet |
| 2 | Run test to verify fix | 1 | no | low | sonnet |

## Order
Sequential: [1] → [2]
