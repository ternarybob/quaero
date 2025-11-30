# Step 2: Create Generic StepManager

- Task: task-2.md | Group: 2 | Model: opus

## Actions
1. Added StepWorker interface to `internal/interfaces/job_interfaces.go`
2. Created GenericStepManager in `internal/queue/generic_manager.go`
3. Updated Orchestrator to use GenericStepManager with fallback to legacy routing
4. Added RegisterStepWorker method to Orchestrator
5. Implemented worker registry with type-based lookup

## Files
- `internal/interfaces/job_interfaces.go` - Added StepWorker interface
- `internal/queue/generic_manager.go` - NEW: GenericStepManager implementation
- `internal/queue/orchestrator.go` - Updated to use generic manager

## Decisions
- Placed GenericStepManager in queue package (not managers) to avoid import cycle
- Used arbor.ILogger for consistency with codebase
- Maintained backward compatibility with legacy stepExecutors map
- Try new routing first, fallback to legacy if no worker registered

## Verify
Compile: ✅ | Tests: ✅

## Status: ✅ COMPLETE
