# Summary: Fix Log Line Number Ordering

## Issue
Log line numbers were out of order and duplicated when viewed in the queue UI. The screenshot showed: 3, 24, 1, 2, 25, 5, 5, 6, 6...

## Root Cause
The line number counter in `log_storage.go` was keyed by `jobID`. When multiple workers (each with their own jobID) logged to the same step, they each got independent counters. When logs were aggregated by step, these independent counters produced interleaved/duplicate line numbers.

## Solution
Changed the counter key from `jobID` to `step_id` (extracted from log entry context). Now all workers logging to the same step share one atomic counter, producing sequential line numbers (1, 2, 3, 4...) based on actual write order.

## Files Changed
| File | Change |
|------|--------|
| `internal/storage/badger/log_storage.go` | Counter key changed from jobID to step_id |

## Key Changes
1. Renamed `jobLineCounters` → `stepLineCounters`
2. `AppendLog()` now extracts `step_id` from entry.Context as counter key
3. `getNextLineNumber()` uses step_id to find/create counter
4. Sort functions updated to compare by LineNumber only for same-step logs
5. Added `ClearStepLineCounter()` helper

## Verification
- Build: PASS
- Unit tests: PASS (8/8 logs service tests)

## Answer to Original Question
**"Is go-quartz the correct library to manage the queue?"**

**No.** go-quartz is for **scheduling** (cron triggers), not for **queuing** (FIFO execution). The existing `BadgerQueueManager` is appropriate for queue management. The issues were:
1. Log line numbering (FIXED in this change)
2. Statistics accuracy (separate issue - throttling/event timing)

The previous go-quartz refactor to replace `robfig/cron/v3` in the scheduler service was correct for scheduling purposes but did not address the queue/logging issues shown in the screenshot.
