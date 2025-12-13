# Step 1: Fix EventDocumentUpdated Count Increment
Model: opus | Status: ✅

## Done
- Removed `IncrementDocumentCount()` call from `EventDocumentUpdated` handler
- Added detailed comment explaining the design decision
- Kept logging for debugging (now logs "count not incremented" message)

## Files Changed
- `internal/queue/state/monitor.go` - Modified EventDocumentUpdated handler (lines 515-545)
  - Removed call to `m.jobMgr.IncrementDocumentCount(ctx, parentJobID)`
  - Added comment block explaining: document updates modify existing documents, not create new ones
  - Document count should reflect UNIQUE documents, not total operations
  - Example: 20 docs created + 20 docs updated = 20 (not 40)

## Verify
Build: pending | Tests: pending
