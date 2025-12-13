# Refactor Queue Architecture

The goal is to separate concerns in `internal/queue` by splitting the monolithic `Manager` into distinct services and improving the logging architecture to be context-aware without tight coupling.

## User Review Required

> [!IMPORTANT]
> This refactoring will split `internal/queue/manager.go` into `JobManager` and `StepManager`.
> `JobManager` will handle persistence and status.
> `StepManager` will handle worker registration and routing.
> Logging will be decoupled from the Manager's metadata inspection.

## Proposed Changes

### Queue Domain (`internal/queue/`)

#### [NEW] [step_manager.go](file:///c:/development/quaero/internal/queue/step_manager.go)
- Create `StepManager` struct.
- Move `RegisterWorker`, `HasWorker`, `GetWorker` from `Manager`.
- Implement `Execute` method to route steps to workers (logic currently in `Manager` or missing `GenericStepManager`).

#### [MODIFY] [manager.go](file:///c:/development/quaero/internal/queue/manager.go)
- Remove `workers` map and registration methods.
- Remove `resolveJobContext` magic.
- Focus on `CreateJob`, `GetJob`, `UpdateJobStatus`, `AddJobLog` (but simplified).
- `AddJobLog` should take explicit context or be called by a wrapper that has context.

#### [MODIFY] [workers/job_processor.go](file:///c:/development/quaero/internal/queue/workers/job_processor.go)
- Update to use `StepManager` for worker lookup if applicable (though `JobProcessor` currently has its own `executors` map for `JobWorker` interface, which is different from `DefinitionWorker`).
- Ensure it uses the new logging approach.

### Interfaces (`internal/interfaces/`)

#### [MODIFY] [job_interfaces.go](file:///c:/development/quaero/internal/interfaces/job_interfaces.go)
- Define `StepManager` interface if needed.
- Update `JobStatusManager` or create `JobLogger` interface to support context-aware logging.

### Logging

- Create a helper or wrapper for `arbor.ILogger` that also sends to `JobManager.AddJobLog`.
- This ensures "single line: `log.debug(...)`" works as requested.

## Verification Plan

### Automated Tests
- Run existing tests in `internal/queue/` to ensure no regression.
- Create new test `internal/queue/step_manager_test.go` to verify routing.
- Run `test/ui/queue_test.go` (if exists and relevant) to verify UI updates still work.

### Manual Verification
- Start the application.
- Run a job (e.g., "Nearby Restaurants").
- Verify logs appear in the UI and are correctly attributed to steps.
- Verify "Job Statistics" and "Job Progress" panels update correctly (addressing the other user concern).
