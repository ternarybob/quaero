# Architect Analysis: Job Log UI Cleanup

## Tasks

### Task 1: Reduce job log entry font size to 0.7rem
**Location:** `pages/static/quaero.css` line 1529
**Current:** `font-size: 0.8rem`
**Target:** `font-size: 0.7rem`

### Task 2: Remove log display count, show only total
**Location:** `pages/queue.html` lines 676-683
**Current:** `'logs: ' + getFilteredTreeLogs(...).length + '/' + total`
**Target:** `total` (just the number)

### Task 3: Update tests
**Location:** `test/ui/job_definition_general_test.go`
**Files to modify:**
- Assertion 5 in `TestJobDefinitionTestJobGeneratorLogFiltering` (lines 1035-1079)
- `assertLogCountDisplayFormat` function (lines 1418-1540+)

Tests currently expect format `logs: X/Y` - need to change to just expect a total number.

## Changes Summary

| Task | File | Type |
|------|------|------|
| Font size | `pages/static/quaero.css` | MODIFY |
| Log count display | `pages/queue.html` | MODIFY |
| Tests | `test/ui/job_definition_general_test.go` | MODIFY |

## Anti-Creation Check
- No new files needed
- All changes modify existing code
