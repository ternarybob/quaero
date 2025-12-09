# Plan: Remove DevOps Job Type

## Overview
Remove the abstract "devops" job type dispatcher and have each worker directly addressable by its actual function name. The individual workers already exist - we just need to:
1. Remove the dispatcher
2. Remove WorkerTypeDevOps from the codebase
3. Update job definitions to use direct worker types instead of `type = "devops"` with `action = "..."` pattern

## Tasks

### Task 1: Remove DevOps Worker Type from worker_type.go
- Remove `WorkerTypeDevOps` constant
- Remove it from `IsValid()` switch
- Remove it from `AllWorkerTypes()` slice
- File: `internal/models/worker_type.go`

### Task 2: Remove DevOps Worker Registration from app.go
- Remove `devopsWorker` creation and registration (lines ~698-709)
- Individual workers are already registered separately (lines 712-760)
- File: `internal/app/app.go`

### Task 3: Delete devops_worker.go
- Delete the dispatcher file entirely
- File: `internal/queue/workers/devops_worker.go`

### Task 4: Update Job Definition TOML
- Change `type = "devops"` to direct worker type
- Remove `action = "..."` configuration
- Update step types:
  - `action = "extract_structure"` -> `type = "extract_structure"`
  - `action = "analyze_build_system"` -> `type = "analyze_build"`
  - `action = "classify_devops"` -> `type = "classify"`
  - `action = "build_dependency_graph"` -> `type = "dependency_graph"`
  - `action = "aggregate_devops_summary"` -> `type = "aggregate_summary"`
- File: `test/results/ui/*/devops_enrich.toml` (generated)
- File: `test/ui/devops_enrichment_test.go` (TOML generation code)

### Task 5: Update Test File devops_enrichment_test.go
- Update TOML generation to use direct worker types
- Remove any references to "devops" job type
- Update step configurations to not use "action" parameter
- File: `test/ui/devops_enrichment_test.go`

### Task 6: Check for Other References
- Search for any other references to "devops" worker type
- Update any test fixtures or configuration files

### Task 7: Build and Test
- Run `go build ./...` to ensure compilation
- Run tests to verify functionality

## Acceptance Criteria
- [ ] `WorkerTypeDevOps` removed from codebase
- [ ] `devops_worker.go` deleted
- [ ] No `action` configuration pattern in job definitions
- [ ] Each step uses direct worker type (extract_structure, analyze_build, classify, dependency_graph, aggregate_summary)
- [ ] Build passes
- [ ] Tests pass
