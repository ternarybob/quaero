# Step 10: Fix Step totalLogCount from SSE Data

## Problem
The SSE handler was incorrectly assigning the JOB's total log count to each STEP's totalLogCount.

**Before:** `logs: 100/1159` - where 1159 is the job total, not the step's count
**After:** `logs: 100/100` - shows the step's actual log count

## Change Made

**`pages/queue.html:4898`**

```javascript
// Before (BUG)
totalLogCount: data.meta?.total_count || mergedLogs.length

// After (FIXED)
// Use step's actual log count, not job's total count
// data.meta.total_count is the JOB total, not per-step
totalLogCount: mergedLogs.length
```

## Explanation
- `data.meta.total_count` comes from `CountAggregatedLogs(ctx, jobID, ...)` on the server
- This is the ENTIRE JOB's log count, which aggregates all steps
- Each step has its own logs, so `mergedLogs.length` is the correct count for the step
- The display formula `logs: X/Y` now correctly shows step's displayed/total counts

## Build Status
✅ Build passes
