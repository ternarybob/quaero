# Summary: Rename Step/StepWorker to Worker Terminology

## Objective
Remove confusing "step" terminology and replace with clearer "worker" terminology throughout the codebase.

## What Changed

### Terminology Mapping
| Before | After |
|--------|-------|
| `StepType` | `WorkerType` |
| `StepWorker` | `DefinitionWorker` |
| `StepManager` | Removed (unused) |
| `ValidateStep()` | `ValidateConfig()` |
| `step_type.go` | `worker_type.go` |

### Constants Renamed
- `StepTypeAgent` → `WorkerTypeAgent`
- `StepTypeCrawler` → `WorkerTypeCrawler`
- `StepTypePlacesSearch` → `WorkerTypePlacesSearch`
- `StepTypeWebSearch` → `WorkerTypeWebSearch`
- `StepTypeGitHubRepo` → `WorkerTypeGitHubRepo`
- `StepTypeGitHubActions` → `WorkerTypeGitHubActions`
- `StepTypeTransform` → `WorkerTypeTransform`
- `StepTypeReindex` → `WorkerTypeReindex`
- `StepTypeDatabaseMaintenance` → `WorkerTypeDatabaseMaintenance`

## Files Modified

### Core Changes (14 source files)
- `internal/models/worker_type.go` (renamed)
- `internal/models/job_definition.go`
- `internal/interfaces/job_interfaces.go`
- `internal/queue/manager.go`
- `internal/queue/workers/*.go` (6 workers)
- `internal/handlers/*.go` (2 handlers)
- `internal/jobs/service.go`
- `internal/services/validation/toml_validation_service.go`

### Test Files (3)
- `internal/models/job_definition_test.go`
- `internal/jobs/service_test.go`
- `internal/common/replacement_integration_test.go`

## Validation
- Build: PASS
- Tests: PASS (relevant tests)

## Impact
- Internal API only - no external impact
- Compile-time safe changes
- Clearer, more consistent terminology
