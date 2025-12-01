# Step 1: Update Job Models

Model: opus | Status: ✅

## Done

- Added `JobTypeManager` and `JobTypeStep` constants in `crawler_job.go`
- Added `ManagerID *string` field to `QueueJob` struct
- Added `ManagerID *string` field to `QueueJobState` struct
- Updated `NewQueueJobState()` to copy ManagerID
- Updated `ToQueueJob()` to copy ManagerID
- Updated `Clone()` to copy ManagerID
- Added new constructors:
  - `NewQueueManager()` - creates manager job (depth 0)
  - `NewQueueStep()` - creates step job (depth 1)
  - `NewQueueJobForStep()` - creates job under step (depth 2)
- Added helper methods:
  - `IsManager()`, `IsStep()`, `IsJob()` - type checks
  - `GetManagerID()` - returns manager ID or empty string

## Files Changed

- `internal/models/crawler_job.go` - Added JobTypeManager, JobTypeStep constants
- `internal/models/job_model.go` - Added ManagerID field, constructors, helper methods

## Verify

Build: ✅ | Tests: ⏭️
