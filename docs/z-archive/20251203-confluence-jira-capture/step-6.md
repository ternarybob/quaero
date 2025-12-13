# Step 6: Add capture history display in sidepanel

Model: sonnet | Status: ✅

## Done

**In sidepanel.html:**
- Added "Captured Pages" section after Recording section
- Created scrollable list container (max-height: 200px, overflow-y: auto)
- Added CSS styles for list items, titles, timestamps, empty state
- Container has id="capture-history-list"

**In sidepanel.js:**
- Implemented `formatRelativeTime(timestamp)` - "Just now", "X min ago", etc.
- Implemented `updateCaptureHistory(capturedUrls)`:
  - Renders list items with title and relative timestamp
  - Shows empty state when no captures
  - Adds tooltips for truncated titles
- Modified `loadRecordingState()` to call updateCaptureHistory

## Files Changed

- `cmd/quaero-chrome-extension/sidepanel.html` - Added CSS and HTML section (now 394 lines)
- `cmd/quaero-chrome-extension/sidepanel.js` - Added history functions (now 572 lines)

## Build Check

Build: ✅ | Tests: ⏭️

HTML valid, JavaScript syntax validation passed
