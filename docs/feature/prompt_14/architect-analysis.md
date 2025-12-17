# Architect Analysis: Prompt 14 Issues

## Issues from prompt_14.md

1. **"Show earlier logs" link has NEVER worked** - Remove it
2. **Completed jobs show all logs, then revert after refresh** - Fix inconsistency
3. **Total log count not updating consistently** - Verify/fix

## Analysis

### Issue 1: "Show earlier logs" Never Worked

**Location:** `pages/queue.html` lines 709-722

The button exists but the `loadMoreStepLogs` function (line 5126) has issues:
- It increases the limit but the API may not return more logs
- The `hasStepEarlierLogs` check compares `totalCount > shownCount` but both may be equal after initial load

**Action:** REMOVE the "Show earlier logs" button and related functions per user request.

### Issue 2: Completed Jobs Logs Revert After Refresh

**Root Cause:** In `getFilteredTreeLogs` (line 5081-5091):
```javascript
if (!isTerminalJob) {
    // Limit applied only to non-terminal jobs
    filteredLogs = filteredLogs.slice(-limit);
}
```

For terminal jobs, NO limit is applied - all logs show. But after page refresh:
- The API fetches logs with a default limit
- The `step.logs` array only contains what was fetched
- Even though no display limit is applied, the data isn't there

**Fix:** For completed jobs, the API should fetch ALL logs, not just the default limit.

### Issue 3: Total Log Count Inconsistent

**Current Implementation:** Line 681 shows:
```javascript
step.unfilteredLogCount || step.totalLogCount || step.logs.length
```

The `totalLogCount` is updated via:
1. SSE handler (line 4904): Uses `maxLineNumber` from logs
2. API fetch (line 4368, 4682, etc.): Uses API response `total_count`

**Problem:** The display falls back to `step.logs.length` which is only the in-memory count, not the true total.

## Changes Required

| File | Change |
|------|--------|
| `pages/queue.html` | Remove "Show earlier logs" button (lines 709-722) |
| `pages/queue.html` | Remove related functions: `hasStepEarlierLogs`, `getStepEarlierLogsCount`, `loadMoreStepLogs` |
| `pages/queue.html` | Fix log count display to always show `totalLogCount` |
| `test/ui/*.go` | Update tests that reference "Show earlier logs" |

## Anti-Creation Check
- All changes MODIFY existing code
- No new files needed
