# Refactor: Queue Manager Separation of Concerns

- Slug: queue-manager-refactor | Type: refactor | Date: 2025-12-02 | Status: ✅ COMPLETE
- Request: "Review internal\queue and update the docs\architecture\manager_worker_architecture.md documentation to align... separation of concerns..."
- Prior: docs/fix/20251202-step-events-architecture

## User Intent

The user identified that `internal/queue/manager.go` was violating separation of concerns by handling both job persistence/lifecycle AND step worker routing. Additionally, logging was tightly coupled and content-specific.

The goal is to:
1.  **Separate Services**: Split `Manager` into:
    -   `JobManager`: Handles job persistence, status updates, and lifecycle (Queue Domain).
    -   `StepManager`: Handles worker registration and step routing (Queue Domain, but distinct responsibility).
    -   `Orchestrator`: Handles job definition execution coordination.
2.  **Decouple Logging**: Ensure logging is context-aware but not hardcoded with "magic" context resolution in the persistence layer.
3.  **Update Documentation**: Align architecture docs with the new structure.

## Changes

### 1. New `StepManager` Service
-   Created `internal/queue/step_manager.go`.
-   Moves `RegisterWorker`, `HasWorker`, `GetWorker`, and `Execute` logic out of the main Manager.
-   Implements `interfaces.StepManager`.

### 2. New `Orchestrator` Service
-   Created `internal/queue/orchestrator.go`.
-   Extracted `ExecuteJobDefinition` logic from Manager.
-   Coordinates job definition execution using StepManager for routing.

### 3. Refactored `JobManager` (formerly `Manager`)
-   Removed worker registry and routing logic from `internal/queue/job_manager.go`.
-   Focusing purely on:
    -   `CreateJob` / `CreateJobRecord`
    -   `GetJob` / `ListJobs`
    -   `UpdateJobStatus` / `UpdateJobProgress`
    -   `AddJobLog`

### 4. New `ContextLogger`
-   Created `internal/queue/logging/context_logger.go`.
-   Context-aware logging wrapper for arbor.ILogger.
-   Logs to both system log and job-specific log.

### 5. Interface Updates
-   Added `StepManager` interface to `internal/interfaces/job_interfaces.go`.

### 6. Documentation Updates
-   Updated `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md` with new architecture.
-   Created Mermaid diagram showing component relationships.

## Verification Plan

-   [x] Verify `StepManager` correctly registers and routes workers.
-   [x] Verify `JobManager` still handles job creation and status updates correctly.
-   [x] Verify `Orchestrator` uses `StepManager` instead of `Manager` for step execution.
-   [x] Verify logs still appear in the UI with correct step context.
-   [x] All unit tests pass (`internal/queue/step_manager_test.go`).
-   [x] All worker tests pass (`internal/queue/workers`).

## Related Documents

- `01_implementation.md` - Initial implementation plan
- `02_work.md` - Completed work summary (separation of concerns)
- `03_logging_architecture_review.md` - Logging pub/sub architecture review (pending implementation)
