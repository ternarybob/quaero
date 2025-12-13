# Plan: Filter Verbose Logs from Step Events UI

Type: fix | Workdir: ./docs/fix/20251201-step-events-filter-verbose/

## Summary

The Step Events panel in the queue UI shows too many verbose logs (DEBUG level). Two sources of logs need filtering:
1. **Real-time WebSocket events** - filtered via `AddJobLogWithEvent` (done in tasks 1-2)
2. **Historical API fetch** - needs API default change to filter debug logs

## Tasks

| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Add log level filter in AddJobLogWithEvent - skip UI publish for debug/trace | - | no | sonnet |
| 2 | Add PublishToUI option to JobLogOptions for override control | 1 | no | sonnet |
| 3 | Change API /api/jobs/{id}/logs default to return INFO+ unless level=all | 2 | no | sonnet |
| 4 | Verify with UI test - check step events show only INFO+ | 3 | no | sonnet |

## Order

[1] → [2] → [3] → [4]

## Key Design Decisions

1. **Filter at publish time**: Don't publish EventJobLog for levels below INFO (real-time)
2. **Filter API default**: Return INFO+ logs by default, debug only if level=all explicitly
3. **DB unaffected**: Continue storing all logs in the job_logs table
4. **Override option**: Allow workers to override with PublishToUI in JobLogOptions
