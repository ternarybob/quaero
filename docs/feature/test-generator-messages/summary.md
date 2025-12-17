# Summary: Test Job Generator Message Clarity

## Question Answered

**Q: Are the warning/error messages real or simulated?**

**A: They are completely SIMULATED.** The test job generator creates fake log messages with random distribution (80% INFO, 15% WARN, 5% ERROR) to test:
- Log filtering functionality
- UI display of different log levels
- Error tolerance configuration
- Pagination with high log volumes

## Changes Made

**File:** `internal/queue/workers/test_job_generator_worker.go`

### 1. Added [SIMULATED] Prefix
All log messages from test_job_generator now clearly indicate they are fake:

**Before:**
```
[WRN] Warning at item 219: resource usage high
[ERR] Error at item 220: operation failed
```

**After:**
```
[WRN] [SIMULATED] slow_generator/Test Job Generator Worker 1: Warning at item 219 (simulated resource usage high)
[ERR] [SIMULATED] slow_generator/Test Job Generator Worker 1: Error at item 220 (simulated operation failed)
```

### 2. Added Job/Step Context
Each message now includes:
- **Step name** (from job metadata, e.g., `slow_generator`)
- **Job name** (e.g., `Test Job Generator Worker 1`)

Format: `[SIMULATED] {step_name}/{job_name}: {message}`

## Build Status
**PASS** - Both executables built successfully

## Files Modified
- `internal/queue/workers/test_job_generator_worker.go`

## UI Display Note
The step_name and job_name are already available in the job metadata and context fields. The UI can display these in the log entry's key-value section if desired. The message text now also includes this context for clarity when viewing logs directly.
