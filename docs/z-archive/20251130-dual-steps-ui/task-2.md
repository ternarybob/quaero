# Task 2: Fix job hang for zero child jobs
- Group: 2 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 1
- Sandbox: /tmp/3agents/task-2/ | Source: ./ | Output: docs/feature/20251130-dual-steps-ui/

## Files
- `internal/queue/state/monitor.go` - add grace period for zero child jobs

## Requirements
Jobs that report `ReturnsChildJobs()=true` but don't actually spawn any children hang indefinitely (30 min timeout). Add a grace period that completes the job if no children appear within 30 seconds.

Changes:
1. Add `noChildrenGracePeriod = 30 * time.Second`
2. Add `hasSeenChildren` flag
3. Create `checkChildJobProgressWithCount()` function
4. Complete job gracefully if grace period exceeded with no children

## Acceptance
- [ ] Grace period logic added
- [ ] Job completes if no children after 30s
- [ ] Compiles
