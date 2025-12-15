# Step 2: Implementation
Iteration: 2 | Status: complete

## Changes Made

| File | Action | Description |
|------|--------|-------------|
| `pages/queue.html:4758-4761` | modified | Changed initial log limit from 20 to 100 |
| `pages/queue.html:4942-5006` | modified | Enhanced `loadMoreStepLogs` function with better debug logging, validation, and error handling |
| `test/ui/job_definition_general_test.go` | modified | Added two new test functions: `TestJobDefinitionLogInitialCount` and `TestJobDefinitionShowEarlierLogsWorks` |

## Detailed Changes

### 1. Initial Log Limit Change (queue.html:4758-4761)

**Before:**
```javascript
// Load initial logs with a small window so the DOM can show progressive growth.
const limitKey = `${jobId}:${step.name}`;
if (!this.stepLogLimits[limitKey]) {
    this.stepLogLimits = { ...this.stepLogLimits, [limitKey]: 20 };
}
```

**After:**
```javascript
// Load initial logs - show at least 100 logs for better visibility
const limitKey = `${jobId}:${step.name}`;
if (!this.stepLogLimits[limitKey]) {
    this.stepLogLimits = { ...this.stepLogLimits, [limitKey]: 100 };
}
```

### 2. Enhanced loadMoreStepLogs Function (queue.html:4942-5006)

Added the following improvements:
- Entry debug logging with truncated jobId for readability
- Validation check for required parameters (jobId, stepName)
- Duplicate call prevention (skip if already loading)
- URL logging before fetch
- Response validation with step count logging
- Old vs new log count comparison in success logs
- Better error handling with API error status codes
- Completion logging

Key additions:
```javascript
console.log('[Queue] loadMoreStepLogs called:', { jobId: jobId?.substring(0, 8), stepName, stepIndex });

if (!jobId || stepName === undefined) {
    console.error('[Queue] loadMoreStepLogs: Missing required parameters');
    return;
}

// Prevent duplicate calls while loading
if (this.stepLogLoading[key]) {
    console.log('[Queue] loadMoreStepLogs: Already loading, skipping');
    return;
}
```

### 3. New Test Functions (job_definition_general_test.go)

#### TestJobDefinitionLogInitialCount
- Creates job with 750+ logs (5 workers * 150 logs)
- Verifies initial display shows at least 80 logs when total > 100
- Verifies at least 50 logs displayed when "Show earlier logs" button is visible

#### TestJobDefinitionShowEarlierLogsWorks
- Creates job with 1000 logs (10 workers * 100 logs)
- Clicks "Show earlier logs" button
- Verifies log count increases after click
- Verifies at least 20 more logs are loaded

## Build & Test

Build: Not verified (Go not available in environment)
Tests: Added 2 new test functions, compilation not verified

**Note:** Build verification requires Go to be installed. The code changes are syntactically correct based on manual review.

## Architecture Compliance (self-check)

- [x] Log Fetching Strategy (QUEUE_UI.md) - Uses REST API `/api/jobs/{id}/tree/logs` on step expand
- [x] "Show earlier logs" with offset (QUEUE_UI.md) - Uses limit parameter, increases by 100 per click
- [x] API calls < 10 per step (QUEUE_UI.md) - Incremental loading with 100-log chunks
- [x] GET /api/jobs/{id}/logs params (QUEUE_LOGGING.md) - Uses limit, level parameters correctly
- [x] Log line starts at 1 (QUEUE_LOGGING.md) - Maintained via `logIdx + 1` pattern

## Files Modified

1. **pages/queue.html**
   - Line 4758-4761: Initial limit 20 → 100
   - Line 4942-5006: Enhanced loadMoreStepLogs with debug logging and validation

2. **test/ui/job_definition_general_test.go**
   - Lines 1581-1908: Added TestJobDefinitionLogInitialCount and TestJobDefinitionShowEarlierLogsWorks
