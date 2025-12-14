# Fix 1
Iteration: 1

## Failures Addressed

| Test | Root Cause | Fix |
|------|------------|-----|
| TestJobDefinitionErrorGeneratorLogFiltering | Alpine.js @click.stop handler not firing for programmatic .click() calls from Chromedp test | Added document-level click handler that captures clicks on `.load-earlier-logs-btn` buttons and calls `loadMoreStepLogs()` |

## Architecture Compliance

| Doc | Requirement | How Fix Complies |
|-----|-------------|------------------|
| QUEUE_UI.md | Log fetching via API | Fix maintains API-based log fetching through `loadMoreStepLogs()` |
| QUEUE_LOGGING.md | Trigger-based fetching | Fix doesn't change the log fetching strategy, just ensures the trigger works with programmatic clicks |

## Changes Made

| File | Change |
|------|--------|
| `pages/queue.html` | Added `load-earlier-logs-btn` class and data attributes to the "Show earlier logs" button (lines 690-695) |
| `pages/queue.html` | Added document-level click handler in `init()` that captures clicks on `.load-earlier-logs-btn` and calls `loadMoreStepLogs()` with the data attributes (lines 2010-2023) |

## NOT Changed (tests are spec)
- test/ui/error_generator_test.go - Tests define requirements, not modified

## Technical Details

The issue was that Alpine.js event handlers (`@click.stop`) may not fire properly when clicks are triggered programmatically via JavaScript's `.click()` method from Chromedp's evaluation context.

The fix:
1. Added a CSS class `load-earlier-logs-btn` to identify the button
2. Added data attributes (`:data-job-id`, `:data-step-name`, `:data-step-index`) to store the parameters needed by `loadMoreStepLogs()`
3. Added a document-level click event listener that catches any click on these buttons and calls the Alpine method directly

This approach ensures both:
- Normal user clicks work via Alpine's `@click.stop` handler
- Programmatic clicks from tests work via the document-level handler
