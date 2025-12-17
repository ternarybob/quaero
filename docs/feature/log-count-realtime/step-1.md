# Step 1: Worker Implementation

## Problem

Screenshot showed log line numbers at 214-221, but the total count badge showed "8". The count was not updating in real-time as logs streamed in.

## Root Cause

In `pages/queue.html` line 4900, the SSE handler was setting:
```javascript
totalLogCount: mergedLogs.length
```

This only reflected logs in memory, not the actual total. During real-time streaming, the UI may only have a subset of logs loaded.

## Solution

Use the **highest line_number** from logs as the total count, since line numbers are server-assigned, sequential, and start at 1.

**File:** `pages/queue.html` (lines 4893-4904)

```javascript
// Before
totalLogCount: mergedLogs.length

// After
const maxLineNumber = Math.max(...mergedLogs.map(l => l.line_number || 0));
const realTimeTotal = Math.max(maxLineNumber, currentStep.totalLogCount || 0, mergedLogs.length);
totalLogCount: realTimeTotal
```

## Test Update

**File:** `test/ui/job_definition_general_test.go`

Updated `assertLogCountDisplayFormat` to:
1. Extract highest visible line number from DOM
2. Verify total count >= highest visible line number
3. Fail if count is less (indicates real-time update issue)

## Build Status
**PASS** - Both main build and test package compile successfully

## Files Modified
- `pages/queue.html`
- `test/ui/job_definition_general_test.go`
