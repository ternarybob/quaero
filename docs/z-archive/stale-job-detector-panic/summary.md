# Complete: Fix stale job detector panic

Fixed a panic in the stale job detector loop caused by BadgerHold's `IsNil()` query method when comparing nil pointer fields (`LastHeartbeat *time.Time`). The panic occurred with error `reflect: call of reflect.Value.Interface on zero Value`. The fix replaces the problematic two-query approach with a single query for all running jobs followed by in-memory filtering for stale conditions, which safely handles nil pointer comparisons without reflection issues.

## Stats
Tasks: 1 | Files: 1 | Duration: ~5 minutes
Models: Planning=opus, Workers=1×sonnet, Review=N/A (not critical)

## Tasks
- Task 1: Fixed `GetStaleJobs()` in `queue_storage.go` to use in-memory filtering instead of `IsNil()` query

## Changes Made
- `internal/storage/badger/queue_storage.go`:
  - Replaced two separate queries with single query for running jobs
  - Added in-memory filtering for stale conditions:
    - Jobs with `LastHeartbeat != nil && LastHeartbeat.Before(threshold)`
    - Jobs with `LastHeartbeat == nil && StartedAt != nil && StartedAt.Before(threshold)`
  - Added code comment explaining why `IsNil()` is avoided

## Verify
go build ✅
