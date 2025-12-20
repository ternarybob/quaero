# Task Summary: Job Templates and Variables Refactor

## Task Description

Three sub-tasks:
1. Move variables from `{exe}/variables/variables.toml` to `{exe}/variables.toml` (like email.toml and connectors.toml)
2. Create job template system for stock investigation jobs with variable replacement
3. Create job template tests in `test/api/` and `test/ui/`

## Status: ✅ COMPLETE

Build: **PASSING**

## Changes Summary

### Task 1: Variables File Location

| File | Change |
|------|--------|
| `internal/storage/badger/load_variables.go` | Extended to support root-level `variables.toml` |
| `internal/common/config.go` | Changed default from `./variables` to `./` |
| `bin/variables.toml` | Moved from `bin/variables/variables.toml` |

**Behavior:**
- First checks for `{exe}/variables.toml` (new pattern)
- Then checks `{exe}/variables/*.toml` (backward compatibility)

### Task 2: Job Template System

| File | Purpose |
|------|---------|
| `internal/models/worker_type.go` | Added `WorkerTypeJobTemplate = "job_template"` |
| `internal/queue/workers/job_template_worker.go` | New worker for template execution |
| `internal/app/app.go` | Registration of new worker |
| `bin/job-templates/asx-stock-analysis.toml` | Sample ASX stock analysis template |
| `bin/job-definitions/asx-stocks-daily.toml` | Example usage of template |

**Template Variable Syntax:**
```toml
# In template file:
id = "web-search-asx-{stock:ticker_lower}"
name = "ASX:{stock:ticker} Investment Analysis"
asx_code = "{stock:ticker}"

# In job definition:
[step.run_templates]
type = "job_template"
template = "asx-stock-analysis"
variables = [
    { ticker = "CBA", name = "Commonwealth Bank", industry = "banking" },
    { ticker = "WES", name = "Wesfarmers", industry = "retail" },
]
```

### Task 3: Tests

| File | Purpose |
|------|---------|
| `test/api/job_template_test.go` | API tests for job template creation and worker type |
| `test/ui/job_template_test.go` | UI tests for job template visibility |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      Job Template System                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  bin/job-definitions/asx-stocks-daily.toml                      │
│  ┌─────────────────────────────────────────┐                    │
│  │ [step.run_templates]                    │                    │
│  │ type = "job_template"                   │                    │
│  │ template = "asx-stock-analysis"         │                    │
│  │ variables = [{ ticker = "CBA", ... }]   │                    │
│  └──────────────────┬──────────────────────┘                    │
│                     │                                            │
│                     ▼                                            │
│  ┌─────────────────────────────────────────┐                    │
│  │        JobTemplateWorker                 │                    │
│  │  1. Load template from job-templates/    │                    │
│  │  2. For each variable set:               │                    │
│  │     - Substitute {stock:key} placeholders│                    │
│  │     - Create ephemeral job definition    │                    │
│  │     - Execute via Orchestrator           │                    │
│  └──────────────────┬──────────────────────┘                    │
│                     │                                            │
│                     ▼                                            │
│  bin/job-templates/asx-stock-analysis.toml                      │
│  ┌─────────────────────────────────────────┐                    │
│  │ id = "web-search-asx-{stock:ticker_lower}│                   │
│  │ [step.fetch_stock_data]                  │                    │
│  │ asx_code = "{stock:ticker}"              │                    │
│  │ ...                                      │                    │
│  └─────────────────────────────────────────┘                    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Before vs After

### Variables Loading

**Before:**
```
bin/
├── variables/
│   └── variables.toml    ← Loaded from subdirectory
├── email.toml
└── connectors.toml
```

**After:**
```
bin/
├── variables.toml        ← Now at root (like email.toml)
├── email.toml
├── connectors.toml
└── job-templates/        ← New directory for templates
    └── asx-stock-analysis.toml
```

### Stock Investigation Jobs

**Before:**
```
bin/job-definitions/
├── web-search-asx-cba.toml   ← 5 near-identical files
├── web-search-asx-exr.toml   ← Only ticker changes
├── web-search-asx-srl.toml
├── web-search-asx-wes.toml
└── web-search-asx.toml
```

**After:**
```
bin/
├── job-templates/
│   └── asx-stock-analysis.toml   ← One template
└── job-definitions/
    └── asx-stocks-daily.toml     ← Orchestrates all stocks
```

## Verification

- ✅ Build passes
- ✅ Variables loaded from root directory
- ✅ Backward compatibility with variables/ subdirectory
- ✅ JobTemplateWorker registered and recognized
- ✅ Template syntax distinct from KV substitution
- ✅ Tests created for both API and UI
- ✅ **API Tests Pass:**
  - `TestJobTemplate_JobDefinitionCreation` - PASS
  - `TestJobTemplate_WorkerTypeValidation` - PASS
  - `TestVariablesFile_LoadFromRoot` - PASS

## Bug Fix (Iteration)

**Issue:** Job execution failed with "invalid job definition type: job_template"

**Root Cause:** Added `job_template` to `WorkerType` but forgot to add it to `JobDefinitionType`

**Fix:** Added `JobDefinitionTypeJobTemplate` to:
- `internal/models/job_definition.go` - constants
- `IsValidJobDefinitionType()` - validation function
- Error message string

## Files Modified/Created

| File | Action |
|------|--------|
| `internal/storage/badger/load_variables.go` | Modified |
| `internal/common/config.go` | Modified |
| `internal/models/worker_type.go` | Modified |
| `internal/app/app.go` | Modified |
| `internal/queue/workers/job_template_worker.go` | Created |
| `bin/variables.toml` | Moved |
| `bin/job-templates/asx-stock-analysis.toml` | Created |
| `bin/job-definitions/asx-stocks-daily.toml` | Created |
| `test/api/job_template_test.go` | Created |
| `test/ui/job_template_test.go` | Created |
