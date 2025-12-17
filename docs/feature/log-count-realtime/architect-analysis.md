# Architect Analysis: Real-time Log Count Fix

## Issue

Screenshot shows log line numbers at 214-221, but the total count badge shows "8". The count should reflect the actual total logs, not just the count of logs in memory.

## Root Cause

**File:** `pages/queue.html` line 4900

```javascript
totalLogCount: mergedLogs.length
```

This sets `totalLogCount` to the number of logs currently in memory, not the actual total. During real-time SSE streaming, the UI may only have a subset of logs loaded while the actual total is much higher.

## Solution

Use the **highest line_number** from the logs as the total count. Since line numbers are server-assigned, sequential, and start at 1, the highest line_number equals the total log count.

**Change from:**
```javascript
totalLogCount: mergedLogs.length
```

**Change to:**
```javascript
totalLogCount: Math.max(...mergedLogs.map(l => l.line_number || 0), currentStep.totalLogCount || 0)
```

This ensures:
1. Real-time updates show correct count based on highest line_number
2. Never decreases (uses max with existing count)
3. Falls back gracefully if line_number not available

## Files to Modify

| File | Change |
|------|--------|
| `pages/queue.html` | Update SSE handler to use max line_number |
| `test/ui/job_definition_general_test.go` | Update assertions for real-time count |

## Anti-Creation Check
- No new files needed
- All changes modify existing code
