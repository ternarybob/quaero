# Task 1: Update timestamp format to include milliseconds
Depends: - | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent
Events not able to be ordered when jobs complete under 1 second because timestamp only has second precision.

## Skill Patterns to Apply
- Structured logging with arbor
- Context everywhere
- Wrap errors with context

## Do
1. Update `consumer.go` to use RFC3339Nano format for `FullTimestamp`
2. Update display timestamp `Timestamp` to include milliseconds: "15:04:05.000"
3. Update `JobLogEntry` model documentation to reflect new format

## Accept
- [ ] FullTimestamp uses RFC3339Nano format with nanosecond/millisecond precision
- [ ] Display timestamp shows milliseconds
- [ ] Build succeeds
