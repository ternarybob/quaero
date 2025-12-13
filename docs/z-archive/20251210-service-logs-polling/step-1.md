# Step 1: Add debug logging to LogEventAggregator
Model: sonnet | Skill: go | Status: Done

## Done
- Added Trace-level log when skipping trigger (no pending logs)
- Changed existing Debug log message to be clearer ("triggering refresh")
- Trace level ensures this won't appear in production logs unless explicitly enabled

## Files Changed
- `internal/services/events/log_aggregator.go` - Lines 110-121: Added trace log for skipped triggers

## Skill Compliance (go)
- [x] Use arbor structured logging (Trace level for skip, Debug for trigger)
- [x] Don't pollute production logs (Trace is lowest level)

## Build Check
Build: Pending | Tests: Pending
