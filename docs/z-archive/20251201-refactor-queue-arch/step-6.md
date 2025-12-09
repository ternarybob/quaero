# Step 6: Update GetJobChildStats

Model: opus | Status: ✅

## Done

- Added `StepStats` struct to jobtypes for manager-level step aggregation
- Added `GetStepStats` method to QueueStorage interface
- Added `ListStepJobs` method to QueueStorage interface
- Implemented both methods in BadgerDB storage
- Added wrapper methods in queue/manager.go
- Updated MockJobStorage test mock with new methods

## Key Types Added

```go
// StepStats holds aggregate statistics for step jobs under a manager
type StepStats struct {
    TotalSteps     int // Total step jobs under the manager
    CompletedSteps int // Steps that have finished
    RunningSteps   int // Steps currently running
    PendingSteps   int // Steps waiting to start
    FailedSteps    int // Steps that failed
    CancelledSteps int // Steps that were cancelled
    TotalJobs      int // Total jobs across all steps (leaf job count)
    CompletedJobs  int // Total completed jobs across all steps
    FailedJobs     int // Total failed jobs across all steps
}
```

## New Methods

```go
// QueueStorage interface
GetStepStats(ctx, managerID) (*StepStats, error)
ListStepJobs(ctx, managerID) ([]*QueueJob, error)

// queue.Manager wrapper methods
GetStepStats(ctx, managerID) (*StepStats, error)
ListStepJobs(ctx, managerID) ([]*QueueJob, error)
```

## Usage

- `GetStepStats` - Used by ManagerMonitor to track overall progress
- `ListStepJobs` - Used by UI to display step-level hierarchy

## Files Changed

- `internal/interfaces/jobtypes/jobtypes.go` - Added StepStats struct
- `internal/interfaces/storage.go` - Added StepStats alias, GetStepStats, ListStepJobs
- `internal/storage/badger/queue_storage.go` - Implemented GetStepStats, ListStepJobs
- `internal/queue/manager.go` - Added wrapper methods
- `internal/logs/service_test.go` - Added mock implementations

## Verify

Build: ✅ | Tests: ⏭️
