# Complete: Fix Child Expand and Document Count Issues
Type: fix | Tasks: 4 | Files: 2

## Result
Fixed three UI issues in the Queue Management page: (1) children expand button now properly toggles child job visibility by using Alpine.js object spread pattern for reactivity, (2) document count now shows unique documents only by removing the increment for `EventDocumentUpdated` (updates to existing documents don't create new ones), and (3) child jobs now display their own document counts by tracking per-child document_count in the event handler.

## Changes Summary

### Issue 1: Document Count Double-Counting (CRITICAL)
**Root Cause:** Both `EventDocumentSaved` AND `EventDocumentUpdated` were incrementing the parent's document_count. When Step 2 (agent) updated documents created by Step 1, the count was incremented again.

**Fix:** Removed `IncrementDocumentCount()` call from `EventDocumentUpdated` handler. Updates modify existing documents - they don't create new ones.

### Issue 2: Children Expand Button Not Working
**Root Cause:** Alpine.js reactivity wasn't triggered when adding new keys to the `expandedParents` object.

**Fix:** Changed `toggleParentExpand()` to use object spread: `this.expandedParents = { ...this.expandedParents, [parentId]: newState }`

### Issue 3: Child Jobs Missing Document Count
**Root Cause:** Only parent jobs had `document_count` incremented. Child jobs had no way to display their individual counts.

**Fix:** Added code in `EventDocumentSaved` handler to also increment the child job's `document_count`.

## Files Changed
- `internal/queue/state/monitor.go`
  - Modified `EventDocumentSaved` handler to increment both parent AND child document counts
  - Modified `EventDocumentUpdated` handler to NOT increment parent count (updates don't create docs)
- `pages/queue.html`
  - Fixed `toggleParentExpand()` to use object spread for Alpine.js reactivity

## Review: N/A (no critical triggers matched)

## Verify
Build: ✅ | Tests: ✅
