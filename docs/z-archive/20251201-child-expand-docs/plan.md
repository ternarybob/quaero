# Plan: Fix Child Expand and Document Count Issues
Type: fix | Workdir: ./docs/fix/20251201-child-expand-docs/

## Problem Analysis

From the screenshot and code analysis, there are **3 distinct issues**:

### Issue 1: Children Expand Button Not Working
The "20 children" button is visible, but clicking it doesn't expand the child jobs. The chevron icon toggles (fa-chevron-right/fa-chevron-down) but children aren't appearing.

**Root Cause:** Looking at the code:
- `toggleParentExpand(parentId)` at line 1920 toggles `this.expandedParents[parentId]`
- `renderJobs()` at line 2248 checks `this.isParentExpanded(parentJob.id)`
- Child jobs are fetched but the rendering loop (line 2251) is correct

The actual issue: `allJobs` might not contain child jobs because they're fetched separately. Need to verify the child fetch happens and children are stored in `allJobs`.

### Issue 2: Document Count Shows 24 Instead of 20
The parent job shows "24 Documents" but only 20 unique documents were created.

**Root Cause:** From `internal/queue/state/monitor.go`:
- Line 495: `IncrementDocumentCount()` on `EventDocumentSaved` (creates 20)
- Line 540: `IncrementDocumentCount()` on `EventDocumentUpdated` (adds 4 more)

The agent step (Step 2) updates the SAME 20 documents for keyword extraction, but each update triggers `EventDocumentUpdated` which increments the count again. This results in double-counting.

**Solution:** The `EventDocumentUpdated` handler should NOT increment document_count for the PARENT job. Updates to existing documents don't create new documents.

### Issue 3: Child Jobs Need Document Count
Child job rows display but don't show their individual document counts. The child template at line 250 shows:
```html
<span x-text="getDocumentsCount(item.job)"></span> docs
```
This is already present but may not be working because child jobs don't have `document_count` in their metadata.

## Tasks
| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Fix EventDocumentUpdated to not increment parent count | - | yes:data-integrity | opus |
| 2 | Debug and fix child expand functionality | - | no | sonnet |
| 3 | Ensure child jobs have document_count populated | 1 | no | sonnet |
| 4 | Validate build and test | 1,2,3 | no | sonnet |

## Order
[1] → [2,3] → [4]

## Key Files
- `internal/queue/state/monitor.go` - EventDocumentUpdated handler (lines 517-558)
- `pages/queue.html` - Child expand logic (lines 1920-1928, 2245-2266)
- `internal/queue/state/progress.go` - IncrementDocumentCount implementation
