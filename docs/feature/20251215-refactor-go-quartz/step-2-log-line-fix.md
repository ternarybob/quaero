# Step 2: Fix Log Line Number Counter

## Problem
Log line numbers were out of order and duplicated when multiple workers logged to the same step.

**Screenshot Evidence:**
```
3  [INF] Processing item 1/50
24 [INF] Child job f01b44b5 → running...
1  [INF] Status changed: running
2  [INF] Test job generator starting...
5  [INF] Processing item 3/50
5  [INF] Processing item 3/50  <-- DUPLICATE
```

## Root Cause
The line number counter was keyed by `jobID`, but multiple workers under the same step each have different job IDs. Each worker got its own counter starting at 1, so when logs were aggregated by step, the line numbers interleaved incorrectly.

**Before:**
```go
var jobLineCounters sync.Map  // Key: jobID

entry.LineNumber = s.getNextLineNumber(ctx, jobID)
```

Worker A (jobID: abc123) → counter 1, 2, 3...
Worker B (jobID: def456) → counter 1, 2, 3...
Result when aggregated: 1, 1, 2, 2, 3, 3...

## Solution
Changed the counter key from `jobID` to `step_id` (from entry.Context). All workers logging to the same step now share one atomic counter.

**After:**
```go
var stepLineCounters sync.Map  // Key: step_id

counterKey := entry.GetContext(models.LogCtxStepID)
if counterKey == "" {
    counterKey = jobID  // fallback
}
entry.LineNumber = s.getNextLineNumber(ctx, counterKey)
```

Worker A (step_id: step123) → uses shared counter
Worker B (step_id: step123) → uses shared counter
Result: 1, 2, 3, 4, 5... (interleaved by actual write order)

## Files Modified

1. **`internal/storage/badger/log_storage.go`**
   - Renamed `jobLineCounters` → `stepLineCounters`
   - Modified `getNextLineNumber()` to use step_id as counter key
   - Modified `AppendLog()` to extract step_id from entry.Context
   - Updated sort functions to compare by LineNumber only for same-step logs
   - Added `ClearStepLineCounter()` helper function

## Sort Logic Update

When sorting logs:
- **Same step**: Sort by LineNumber (1, 2, 3...)
- **Different steps**: Sort by Sequence (timestamp-based global ordering)

```go
stepI := logs[i].GetContext(models.LogCtxStepID)
stepJ := logs[j].GetContext(models.LogCtxStepID)
if stepI != "" && stepI == stepJ && logs[i].LineNumber > 0 && logs[j].LineNumber > 0 {
    return logs[i].LineNumber < logs[j].LineNumber
}
// Fall back to Sequence for cross-step
```

## Build Verification
```
Build passed: quaero.exe (v0.1.1969)
```

## Expected Result
For a step with 50 log items from multiple concurrent workers:
```
1  [INF] Status changed: running
2  [INF] Test job generator starting...
3  [INF] Processing item 1/50
4  [INF] Processing item 2/50
5  [INF] Processing item 3/50
...
50 [INF] Processing item 50/50
```
Line numbers are now sequential (1 to N) based on actual write order, regardless of which worker wrote them.
