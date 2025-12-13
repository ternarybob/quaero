# Step 4: Implement recording logic in sidepanel.js

Model: sonnet | Status: ✅

## Done

- Added event listener for recording-toggle checkbox in DOMContentLoaded
- Implemented `loadRecordingState()` - loads state from background on init
- Implemented `toggleRecording()` - handles start/stop via background.js messages
- Implemented `updateRecordingUI(state)` - toggles 'active' class on indicator, updates count
- Implemented `captureCurrentPage()`:
  - Gets current tab and injects content script
  - Sends capturePageWithImages to content.js
  - Posts to /api/documents/capture with auth cookies
  - Tracks capture via addCapturedUrl message
  - Updates capture count display
- Added initialization call in DOMContentLoaded

## Files Changed

- `cmd/quaero-chrome-extension/sidepanel.js` - Added 187 lines (now ~500 lines total)

## Build Check

Build: ✅ | Tests: ⏭️

JavaScript syntax validation passed
