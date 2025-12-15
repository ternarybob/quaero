# Step 1: Rename error_generator to test_job_generator

## Implementation Summary

### Files Modified

1. **internal/queue/workers/test_job_generator_worker.go** (NEW - renamed from error_generator_worker.go)
   - Renamed `ErrorGeneratorWorker` struct to `TestJobGeneratorWorker`
   - Renamed `NewErrorGeneratorWorker` to `NewTestJobGeneratorWorker`
   - Updated `GetWorkerType()` to return `"test_job_generator"`
   - Updated `GetType()` to return `models.WorkerTypeTestJobGenerator`
   - All internal references updated (queue job type, log messages, etc.)
   - Added helper functions `getConfigIntWithDefault` and `getConfigFloatWithDefault`

2. **internal/models/worker_type.go**
   - Changed `WorkerTypeErrorGenerator` to `WorkerTypeTestJobGenerator`
   - Value changed from `"error_generator"` to `"test_job_generator"`

3. **internal/app/app.go** (lines 741-750)
   - Updated worker registration to use `NewTestJobGeneratorWorker`
   - Updated comment to reflect new worker name

### Job Definition Files

4. **bin/job-definitions/test_job_generator.toml** (NEW - renamed from error_generator.toml)
   - Single generator step with standard configuration
   - type = "test_job_generator"
   - worker_count = 10, log_count = 100, log_delay_ms = 50

5. **test/bin/job-definitions/test_job_generator.toml** (NEW)
   - Same as bin version

6. **test/config/job-definitions/test_job_generator.toml** (NEW - Enhanced)
   - Multiple generator steps for comprehensive testing:
     - `fast_generator`: Quick test (5 workers, 50 logs, 10ms delay)
     - `high_volume_generator`: 1000+ logs (3 workers, 1200 logs, 5ms delay) = 3600+ total logs
     - `slow_generator`: 2+ minutes (2 workers, 300 logs, 500ms delay = 150+ seconds)
     - `recursive_generator`: Child job hierarchy testing (3 workers, 20 logs, child_count=2, depth=2)

### Test Files

7. **test/api/test_job_generator_test.go** (NEW - renamed from error_generator_test.go)
   - All test functions renamed from `TestErrorGenerator*` to `TestTestJobGenerator*`
   - All step types changed from `error_generator` to `test_job_generator`

8. **test/ui/job_definition_general_test.go**
   - Updated header comment
   - Renamed all test functions from `TestJobDefinitionErrorGenerator*` to `TestJobDefinitionTestJobGenerator*`
   - All step types changed from `error_generator` to `test_job_generator`
   - Updated all log messages and comments

### Files Deleted
- internal/queue/workers/error_generator_worker.go
- bin/job-definitions/error_generator.toml
- test/bin/job-definitions/error_generator.toml
- test/config/job-definitions/error_generator.toml
- test/api/error_generator_test.go

## Architecture Compliance

| Requirement | Satisfied |
|-------------|-----------|
| Workers implement GetType() | ✓ Returns `models.WorkerTypeTestJobGenerator` |
| Workers implement DefinitionWorker | ✓ Init, CreateJobs, ValidateConfig implemented |
| Workers implement JobWorker | ✓ Execute, Validate implemented |
| Worker registration with StepManager | ✓ Updated in app.go |
| Worker registration with JobProcessor | ✓ Updated in app.go |
| AddJobLog variants for logging | ✓ All log calls use w.jobMgr.AddJobLog |

## Test Job Generator Configuration

The enhanced test job definition in `test/config/job-definitions/test_job_generator.toml` provides:

### Step 1: fast_generator
- Purpose: Quick execution for basic testing
- Config: 5 workers × 50 logs × 10ms = ~2.5 seconds
- Total logs: ~250

### Step 2: high_volume_generator
- Purpose: 1000+ logs for pagination testing
- Config: 3 workers × 1200 logs × 5ms = ~18 seconds per worker
- Total logs: 3600+ logs
- Satisfies: "1 generator logging to random 1000+"

### Step 3: slow_generator
- Purpose: Long-running job testing (2+ minutes)
- Config: 2 workers × 300 logs × 500ms = 150 seconds per worker
- Total execution: 2.5+ minutes
- Satisfies: "slow down 1 generator to take +2 minutes"

### Step 4: recursive_generator
- Purpose: Tests child job hierarchy
- Config: 3 workers, child_count=2, recursion_depth=2
- Creates nested child jobs for hierarchy testing

## Build Verification

```bash
go build ./...
# Success - no errors

go test -v -timeout 2m ./test/api/... -run TestTestJobGeneratorJobDefinitionCreation
# PASS
```
