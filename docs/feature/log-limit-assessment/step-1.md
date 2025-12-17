# Step 1: Worker Implementation

## Assessment Result

**Recommendation: DO NOT remove the log limit entirely.**

Instead, increased `defaultLogsPerStep` from 100 to 500.

## Log Volume Analysis

From `test/config/job-definitions/test_job_generator.toml`:
- **fast_generator:** 5 workers × 50 logs = 250 logs
- **high_volume_generator:** 3 workers × 1200 logs = 3,600 logs
- **slow_generator:** 2 workers × 300 logs = 600 logs
- **recursive_generator:** 3 workers × 20 logs = 60 logs (+children)
- **TOTAL:** ~4,510+ logs

## DOM Impact

Each log line = 4 DOM elements. With no limit:
- 4,510 logs × 4 elements = 18,040 log DOM elements
- With 4 steps expanded: ~72,000 DOM elements
- Estimated memory: ~7MB

**Risk:** Page becomes unresponsive, scroll stutters, real-time updates lag.

## Changes Made

### 1. Increased Default Log Limit
**File:** `pages/queue.html` line 5046-5047
```javascript
// Before
defaultLogsPerStep: 100,

// After
defaultLogsPerStep: 500,
```

**Rationale:**
- High-volume step (3,600 logs) now needs only 7 "Show earlier" clicks instead of 35
- DOM stays manageable (~8,000 log elements with 4 steps)
- Scroll performance remains acceptable

### 2. Added DOM Performance Test
**File:** `test/ui/job_definition_test_generator_test.go`

Added Assertion 5 that verifies:
- Total DOM elements < 50,000 (manageable)
- Logs per step are counted
- Warning if no logs visible

## Build Status
**PASS** - Both main build and test package compile successfully

## Files Modified
- `pages/queue.html`
- `test/ui/job_definition_test_generator_test.go`
