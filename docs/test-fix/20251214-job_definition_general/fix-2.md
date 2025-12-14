# Fix 2

Iteration: 2

## Failures Addressed

| Test | Root Cause | Fix |
|------|------------|-----|
| Assertion 0: Progressive logs | Logs stored as "info" but queried as "INF" | Normalize level on storage |
| Assertion 3: Steps have no logs | Same - level format mismatch | Same fix |
| Assertion 3b: Completed steps no logs | Same - level format mismatch | Same fix |
| Assertion 4: No step logs found | Same - level format mismatch | Same fix |

## Architecture Compliance

| Doc | Requirement | How Fix Complies |
|-----|-------------|------------------|
| QUEUE_LOGGING.md | Log levels must be consistent | Fix normalizes levels to 3-letter format (INF/WRN/ERR/DBG) |
| QUEUE_UI.md | Logs must display in tree view | Fix enables logs to be retrieved by level filter |

## Changes Made

| File | Change |
|------|--------|
| `internal/storage/badger/log_storage.go` | Added `entry.Level = normalizeLevel(entry.Level)` in AppendLog function |

## NOT Changed (tests are spec)

- `test/ui/job_definition_general_test.go` - Tests define requirements, not modified

## Technical Details

**Before:** Logs were stored with lowercase level names (e.g., `"info"`)
**After:** Logs are normalized to 3-letter uppercase format (e.g., `"INF"`)

This matches what `GetLogsByLevel` expects when querying.

The frontend `normalizeLogLevel` function already handles both formats, so existing UI code will work correctly.
