# Task 5: Update queue_test.go TestStepEventsDisplay

Depends: 4 | Critical: no | Model: sonnet

## Addresses User Intent

Test for changes - specifically events showing from `[step]` and `[worker]` with format like `[worker] page download complete url:xxx.com elapsed:xxx.x` - User Intent #6

## Do

1. Update `test/ui/queue_test.go` TestStepEventsDisplay:
   - Add verification for `[step]` and `[worker]` tags visible in UI
   - Check log format matches: `HH:MM:SS [LVL] [context] message`
   - Verify no duplicate log entries
   - Check worker logs show URL and elapsed time

2. Add assertions:
   - Text `[step]` appears in step events panel
   - Text `[worker]` appears for worker-generated logs
   - Level tags `[INF]`, `[WRN]`, `[ERR]` appear instead of emojis

## Accept

- [ ] Test verifies `[step]` tag visible in events panel
- [ ] Test verifies `[worker]` tag visible for worker logs
- [ ] Test checks for text-based level tags
- [ ] Test compiles and runs
