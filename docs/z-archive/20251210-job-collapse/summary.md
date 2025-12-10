# Complete: Job Steps Collapse Toggle

Type: fix | Tasks: 5 | Files: 1

## User Request

"Update, when user clicks on the job (in queue) collapses/hides the steps"

## Result

Added collapse/expand functionality for step rows under parent jobs in the queue page. Clicking on the metadata area (where timestamps are shown) now toggles step visibility for multi-step jobs. A chevron icon indicates the current state.

## Skills Used

none

## Validation: ✅ MATCHES

All success criteria met:
- Metadata area clickable to toggle steps
- State persists during session
- No navigation to job details on click
- Visual chevron indicator shows state

## Review: N/A

No critical triggers (security, auth, etc.)

## Verify

Build: ✅ | Tests: ⏭️ (frontend UI change - manual verification)

## Changes Made

**pages/queue.html**:
1. Line 1808: Added `collapsedJobs: {}` state object
2. Lines 1960-1970: Added `toggleJobStepsCollapse()` method and `isJobStepsCollapsed()` helper
3. Lines 367-376: Made metadata div clickable with chevron indicator for multi-step jobs
4. Lines 2607-2610: Added check in `renderJobs()` to skip steps when collapsed
