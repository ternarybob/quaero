# Fix: WebSocket Job Logging Consistency

- Slug: websocket-logging-consistency | Type: fix | Date: 2025-12-03
- Request: "Fix inconsistent logging in queue UI - some logs show [step] but others have no context tag, duplicate log entries appear, and worker logs need [worker] tag. Update log format to [time] [level] [worker/step] message. Replace emoji levels with [INF],[DBG] etc. Update tests to verify."
- Prior: docs\fix\queue_manager\websocket_job_logging.md (investigation)

## User Intent

1. All job logs should have consistent context tags: `[step]` for step manager logs, `[worker]` for worker logs
2. Fix duplicate log entries (e.g., "Starting 2 workers..." and "Step finished successfully" appearing twice)
3. Update log format to: `[time] [level] [context] message` (e.g., `[08:41:03] [INF] [worker] Downloading url: xxx`)
4. Replace emoji log levels with standard tags: `[INF]`, `[DBG]`, `[WRN]`, `[ERR]`
5. Match colors from Service Logs, maintain white/transparent background
6. Update tests to verify WebSocket messages show proper `[step]` and `[worker]` context

## Success Criteria

- [ ] All step manager logs include `[step]` tag
- [ ] All worker logs include `[worker]` tag
- [ ] No duplicate log entries in the Events panel
- [ ] Log format follows: `[time] [level] [context] message`
- [ ] Level indicators use text tags `[INF]`, `[DBG]`, `[WRN]`, `[ERR]` instead of emojis
- [ ] test/api/websocket_job_events_test.go passes with proper context verification
- [ ] test/ui/queue_test.go -> TestStepEventsDisplay passes verifying [step] and [worker] tags
