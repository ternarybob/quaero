# Task 1: Add log level filter in AddJobLogWithEvent

Depends: - | Critical: no | Model: sonnet

## Do

1. In `internal/queue/manager.go`, modify `AddJobLogWithEvent` to skip publishing `EventJobLog` for log levels below INFO
2. Define log level priority constants if not already present
3. Only publish event if level is: info, warn, warning, error, fatal, panic

## Accept

- [ ] AddJobLogWithEvent skips WebSocket publish for debug/trace levels
- [ ] DB storage (AppendLog) still receives all logs regardless of level
- [ ] INFO, WARN, ERROR, FATAL logs still published to UI
