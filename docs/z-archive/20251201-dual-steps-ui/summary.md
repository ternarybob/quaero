# Complete: Fix Dual Steps UI - Child Jobs Display

## Classification
- Type: fix
- Location: ./docs/fix/20251201-dual-steps-ui/

This fix updates the Queue Management UI to display child jobs as separate rows under their parent jobs. Previously, child jobs were only shown as aggregated progress text ("4 pending, 10 running, 6 completed"). Now each child job appears on its own line with individual status, document count, and details. Additionally, documented the existing `filter_tags` configuration option for step-based document filtering.

## Stats
Tasks: 4 | Files: 2 | Duration: ~5min
Models: Planning=opus, Workers=sonnet

## Tasks
- Task 1-2: Added child job row template and updated renderJobs() to iterate through child jobs
- Task 3: Added documentation for filter_tags and other step config filter fields
- Task 4: Validated build and tests pass

## Review: N/A (no critical triggers)
No security, authentication, or architectural changes.

## Changes Made

### pages/queue.html
1. **Added child row template** (lines 209-276): New compact row display for child jobs with:
   - Status icon (pending/running/completed/failed/cancelled)
   - Child index (1/N, 2/N, etc.) for progress tracking
   - Job name and type badge
   - Status badge
   - Document count
   - URL display for crawler children
   - Error display for failed children
   - Info button linking to job details

2. **Updated renderJobs() function** (lines 2086-2103): Now iterates through allJobs to find children for each parent and adds them as separate rows after step rows.

### internal/models/job_definition.go
3. **Added documentation** (lines 74-90): Comprehensive godoc comments for step config filter fields:
   - `filter_tags` ([]string): Filter by document tags
   - `filter_created_after` (string): Filter by creation date
   - `filter_updated_after` (string): Filter by update date
   - `filter_limit` (int): Limit number of documents processed

## Verify
go build PASS | go test PASS (2 tests, 66.963s)
