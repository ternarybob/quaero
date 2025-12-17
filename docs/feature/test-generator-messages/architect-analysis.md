# Architect Analysis: Test Job Generator Message Clarity

## Issue Description

The test job generator produces messages like:
- `[WRN] Warning at item 219: resource usage high`
- `[ERR] Error at item 220: operation failed`

User asks: Are these real or simulated?

## Finding: Messages are SIMULATED

**Location:** `internal/queue/workers/test_job_generator_worker.go:114-129`

```go
// Random log level distribution: 80% INFO, 15% WARN, 5% ERROR
randVal := rand.Float64()
var level, message string
if randVal < 0.80 {
    level = "info"
    infoCount++
    message = fmt.Sprintf("Processing item %d/%d", i+1, logCount)
} else if randVal < 0.95 {
    level = "warn"
    warnCount++
    message = fmt.Sprintf("Warning at item %d: resource usage high", i+1)
} else {
    level = "error"
    errorCount++
    message = fmt.Sprintf("Error at item %d: operation failed", i+1)
}
```

**Explanation:**
- These are **completely simulated** messages for testing purposes
- 80% chance of INFO, 15% chance of WARN, 5% chance of ERROR
- The "resource usage high" and "operation failed" messages are fake
- They exist to test log filtering, UI display, and error tolerance features

## Proposed Changes

### Task 1: Add [SIMULATED] prefix
**File:** `internal/queue/workers/test_job_generator_worker.go`
- Change message format to clearly indicate these are test messages
- Prefix: `[SIMULATED]` to make it obvious

### Task 2: Add job/step identification to messages
**File:** `internal/queue/workers/test_job_generator_worker.go`
- Include job name and step name in log messages
- Format: `[SIMULATED] {step_name}/{job_name}: Processing item X/Y`

## Analysis Summary

| Change | Type | File |
|--------|------|------|
| Add [SIMULATED] prefix | MODIFY | `test_job_generator_worker.go` |
| Add job/step context | MODIFY | `test_job_generator_worker.go` |

## Anti-Creation Check
- No new files needed
- All changes modify existing code
