# Task 1: Fix GetStaleJobs to avoid IsNil() query

- Group: 1 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: none
- Sandbox: /tmp/3agents/task-1/ | Source: ./ | Output: docs/fixes/stale-job-detector-panic/

## Files
- `internal/storage/badger/queue_storage.go` - fix GetStaleJobs function

## Requirements
Replace the `IsNil()` query which causes reflection panics with in-memory filtering:
1. Query for all running jobs
2. Filter in-memory for:
   - Jobs with `LastHeartbeat < threshold` (existing behavior)
   - Jobs with `LastHeartbeat == nil` AND `StartedAt < threshold`

## Acceptance
- [ ] No `IsNil()` query used
- [ ] Stale jobs with nil heartbeat are still detected
- [ ] Compiles
- [ ] No panic in stale job detector
