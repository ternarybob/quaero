# Task 1: Add debug logging to LogEventAggregator
Depends: - | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent
Helps diagnose whether backend is correctly skipping triggers when no logs are pending. If it still triggers when idle, we need to fix the aggregator. If it correctly skips, the issue is elsewhere.

## Skill Patterns to Apply
- Use arbor structured logging at Debug level
- Don't pollute production logs (Debug level only)

## Do
1. Add Debug log in flushPending() when hasPendingLogs is false (skipping trigger)
2. Verify the existing "periodic trigger" log is at Debug level
3. Ensure the skip log doesn't create log events itself (avoid recursion)

## Accept
- [ ] Debug log shows when trigger is skipped (no pending logs)
- [ ] Debug log shows when trigger fires (has pending logs)
- [ ] No INFO or higher level logs for these events
- [ ] Code compiles
