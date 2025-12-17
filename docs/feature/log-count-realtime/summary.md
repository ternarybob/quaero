# Summary: Real-time Log Count Fix

## Issue

Screenshot showed log line numbers at 214-221, but the total count badge displayed "8". The count was not updating in real-time as logs streamed via SSE.

## Root Cause

The SSE handler was setting `totalLogCount: mergedLogs.length` which only reflected logs currently in memory, not the actual total. During real-time streaming, the UI may only have a subset of logs loaded.

## Solution

Use the **highest line_number** from logs as the total count. Since line numbers are:
- Server-assigned
- Sequential starting at 1
- Never skipped

The highest line_number equals the actual total log count.

## Changes Made

### 1. Fix SSE Handler
**File:** `pages/queue.html` lines 4893-4904

```javascript
// Calculate real-time total from highest line_number
const maxLineNumber = Math.max(...mergedLogs.map(l => l.line_number || 0));
const realTimeTotal = Math.max(maxLineNumber, currentStep.totalLogCount || 0, mergedLogs.length);
totalLogCount: realTimeTotal
```

### 2. Update Test Assertion
**File:** `test/ui/job_definition_general_test.go`

- Added extraction of highest visible line number from DOM
- Added assertion: `total >= highestLineNum`
- Test will fail if count badge is less than highest visible line number

## Expected Behavior After Fix

**Before:**
```
Line numbers: 214, 215, 216, 217, 218, 219, 220, 221
Count badge: 8
```

**After:**
```
Line numbers: 214, 215, 216, 217, 218, 219, 220, 221
Count badge: 221 (or higher)
```

## Build Status
**PASS** - All code compiles successfully

## Files Modified
- `pages/queue.html`
- `test/ui/job_definition_general_test.go`
