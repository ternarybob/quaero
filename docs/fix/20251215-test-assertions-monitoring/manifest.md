# Fix: Test Assertions and Monitoring Screenshots

Date: 2025-12-15
Request: Fix test assertions for log order/count and restore monitoring screenshots every 30 seconds

## User Intent

1. Fix assertions to verify logs show `(latest-100) → latest`, not random/early logs
2. Fix total log count assertion - UI shows 1243 but panel shows ~370 logs
3. Restore screenshots every 30 seconds DURING job monitoring (without page refresh)

## Success Criteria

- [ ] Logs shown in UI must be latest 100 (highest line numbers), not earliest
- [ ] Displayed log count must match the actual number of log lines in panel
- [ ] Screenshots taken every 30s during job execution without page refresh
- [ ] Tests pass with correct assertions

## Applicable Architecture Requirements

| Doc | Section | Requirement |
|-----|---------|-------------|
| QUEUE_UI.md | Log Display | UI fetches logs with order=desc (newest first) |
| QUEUE_UI.md | Log Line Numbering | Logs MUST start at 1 and increment sequentially |
| QUEUE_LOGGING.md | Log Retrieval API | order param: "desc" = newest first |
| QUEUE_LOGGING.md | Log Line Numbering | Lines start at 1, increment sequentially |

## Key Issues Found

1. **Screenshot during monitoring removed**: The test no longer takes periodic screenshots during the monitoring loop without page refresh
2. **Log order assertion wrong**: Should verify showing lines near total count (e.g., 3500-3600 of 3600), not arbitrary sequential order
3. **Count mismatch**: The "total" in label doesn't match actual displayed lines

## Files to Fix

- `test/ui/job_definition_general_test.go` - Fix assertions and restore monitoring screenshots
