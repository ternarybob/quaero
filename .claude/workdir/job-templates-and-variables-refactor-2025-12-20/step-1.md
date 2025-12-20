# WORKER PHASE: Step 1

## Summary

Implemented all three tasks requested:

### Task 1: Move variables.toml to root directory

**Files Modified:**
- `internal/storage/badger/load_variables.go` - Updated to load from `{exe}/variables.toml` first, then `{exe}/variables/` subdirectory for backward compatibility
- `internal/common/config.go` - Changed default `Variables.Dir` from `"./variables"` to `"./"`
- Moved `bin/variables/variables.toml` → `bin/variables.toml`
- Removed empty `bin/variables/` directory

**Pattern:** Now follows the same pattern as `email.toml` and `connectors.toml`

### Task 2: Create Job Template System

**Files Created:**
- `internal/models/worker_type.go` - Added `WorkerTypeJobTemplate = "job_template"`
- `internal/queue/workers/job_template_worker.go` - New worker that:
  - Loads templates from `{exe}/job-templates/`
  - Applies variable substitution with `{variable:key}` syntax
  - Supports modifiers like `{variable:key_lower}` and `{variable:key_upper}`
  - Executes templated jobs via Orchestrator
- `bin/job-templates/asx-stock-analysis.toml` - Sample template with full ASX stock analysis workflow
- `bin/job-definitions/asx-stocks-daily.toml` - Example job definition using the template

**Files Modified:**
- `internal/app/app.go` - Added import and registration of JobTemplateWorker
- `internal/models/worker_type.go` - Added to `IsValid()` and `AllWorkerTypes()`

**Template Variable Syntax:**
- `{stock:ticker}` - Direct variable lookup
- `{stock:ticker_lower}` - Lowercase transformation
- `{stock:ticker_upper}` - Uppercase transformation

### Task 3: Create Tests

**Files Created:**
- `test/api/job_template_test.go` - API tests for:
  - Creating job definitions with job_template steps
  - Worker type validation
  - Variables file loading verification
- `test/ui/job_template_test.go` - UI tests for:
  - Job template visibility in UI
  - Variables loaded from root verification

## Build Status

✅ Build passes

## Files Changed Summary

| File | Action | Purpose |
|------|--------|---------|
| `internal/storage/badger/load_variables.go` | Modified | Support root-level variables.toml |
| `internal/common/config.go` | Modified | Update default path |
| `bin/variables.toml` | Moved | From bin/variables/ to bin/ |
| `internal/models/worker_type.go` | Modified | Add WorkerTypeJobTemplate |
| `internal/queue/workers/job_template_worker.go` | Created | Template orchestration worker |
| `internal/app/app.go` | Modified | Register new worker |
| `bin/job-templates/asx-stock-analysis.toml` | Created | Sample template |
| `bin/job-definitions/asx-stocks-daily.toml` | Created | Example usage |
| `test/api/job_template_test.go` | Created | API tests |
| `test/ui/job_template_test.go` | Created | UI tests |
