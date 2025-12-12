# Fix 1
Iteration: 1

## Failures Addressed
| Test | Root Cause | Fix |
|------|------------|-----|
| Step icon mismatch (pending=fa-circle) | Line 645 uses `fa-circle` for pending status, but parent job uses `fa-clock`. Also `getStatusIcon` function (line 2677) has same issue. | Change `fa-circle` to `fa-clock` in both locations |
| Log line numbering starts at 4439 | `getStepLogStartIndex` returns `totalCount - displayedCount` which shows offset-based numbers. For step-specific logs, lines should always start at 1. | Change `getStepLogStartIndex` to always return 0 for step logs (line numbers will be `0 + logIdx + 1 = 1, 2, 3...`) |
| import_files step doesn't auto-expand | Auto-expand only happens when step status changes to 'running' (line 4043). If step completes before tree data loads, it misses expansion. Need to also auto-expand 'completed' steps during initial load. | Add `completed` status to auto-expand conditions |

## Architecture Compliance
| Doc | Requirement | How Fix Complies |
|-----|-------------|------------------|
| QUEUE_UI.md | "pending: `fa-clock` - Waiting to start" | Changing `fa-circle` to `fa-clock` matches standard |
| QUEUE_LOGGING.md | "Log lines MUST: Start at line 1 (not 0, not 5)" | Returning 0 from getStepLogStartIndex means line 1 starts at `0 + 0 + 1 = 1` |
| QUEUE_UI.md | "ALL steps should auto-expand when they start running" | Adding 'completed' to auto-expand handles fast-completing steps |

## Changes Made
| File | Change |
|------|--------|
| `pages/queue.html:645` | Change `'fa-circle': step.status === 'pending'` to `'fa-clock': step.status === 'pending'` |
| `pages/queue.html:2677` | Change `'pending': 'fa-circle'` to `'pending': 'fa-clock'` |
| `pages/queue.html:4376` | Change `return totalCount - displayedCount;` to `return 0;` to start line numbers at 1 |
| `pages/queue.html:4043` | Add `'completed'` to the status check so fast-completing steps auto-expand |

## NOT Changed (tests are spec)
- test/ui/job_definition_codebase_classify_test.go - Tests define requirements, not modified
