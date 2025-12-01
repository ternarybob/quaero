# Step 1: Verify and document expected behavior
Model: sonnet | Status: ✅

## Done
- Verified WebSocket failures: All "Failed to send...to client" messages in `internal/handlers/websocket.go` use `h.logger.Warn()` (lines 320, 578, 612, 646, 680, 714, 749, 1063, 1125, 1217) - correctly at WARN level
- Verified Statistics vs UI:
  - `/api/jobs/stats` counts ALL jobs (parents + children) using `CountJobs()` and `CountJobsByStatus()`
  - Queue UI filters with `!job.parent_id` to show only parent jobs
  - This is BY DESIGN - statistics show total activity, UI shows manageable parent list

## Findings
**No bug exists** - the behavior is working as designed:

1. **Job Statistics** (1001 completed) = 1 parent + 1000 child jobs all completed
2. **Job Queue UI** shows 1 parent job (children hidden until expanded)
3. **WebSocket warnings** are already logged at WARN level - they appear in Service Logs panel
4. **Job execution** worked correctly - 1000 child jobs completed successfully

The GitHub repo collector with 1000 files creates:
- 1 parent job (orchestrator)
- 1000 child jobs (one per file to fetch)

## Files Changed
- None - no code changes needed

## Verify
Build: ✅ | Tests: ⏭️
