# ARCHITECT Analysis - Log Ordering and Total Number Mismatch

## Screenshot Analysis
The screenshot shows:
- `high_volume_generator` step showing `logs: 100/1159` in badge
- But the step only has 100 logs displayed
- Line numbers on left: 1, 2, 3... going down correctly
- Log messages show "Processing item 1173/1200" etc. - the actual logged content

## Root Cause

In `pages/queue.html` at line 4898, the SSE handler incorrectly assigns the **job's total log count** to each **step's totalLogCount**:

```javascript
newSteps[stepIdx] = {
    ...currentStep,
    logs: mergedLogs,
    totalLogCount: data.meta?.total_count || mergedLogs.length  // ← BUG HERE
};
```

**The problem:**
- `data.meta?.total_count` is the ENTIRE JOB's log count (from `CountAggregatedLogs`)
- This value is assigned to each step's `totalLogCount`
- The display shows `logs: 100/1159` where 1159 is the job total, not the step total
- This makes it appear like there are 1159 logs for the step when there are only 100

## Fix

**MODIFY** `pages/queue.html:4898` - Don't use job's total count for step's totalLogCount.

For streaming SSE updates, we don't have per-step totals from the server. The correct behavior is to just track what we've received:

```javascript
// DON'T use job total for step total - they're different concepts
// For SSE streaming, we track the step's actual log count
totalLogCount: mergedLogs.length
```

## Files to Modify
1. `pages/queue.html` - Line 4898: Remove `data.meta?.total_count ||` - just use `mergedLogs.length`

## Note
The `data.meta.total_count` from SSE is the JOB's total count, which is useful for job-level displays but NOT for individual step log counts.
