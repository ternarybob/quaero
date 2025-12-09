# Step 1: Add recording state management to background.js

Model: sonnet | Status: ✅

## Done

- Added recording state variables using chrome.storage.local
- Implemented `generateSessionId()` for unique session IDs (format: `session_{timestamp}_{random}`)
- Implemented `startRecording()` - generates session ID, sets recording=true, initializes capturedUrls
- Implemented `stopRecording()` - sets recording=false, preserves session in sessionHistories
- Implemented `getRecordingState()` - returns current state with session details
- Implemented `addCapturedUrl(url, docId, title)` - tracks captures during active session
- Added message handlers: startRecording, stopRecording, getRecordingState, addCapturedUrl
- Preserved existing captureAuth handler

## Files Changed

- `cmd/quaero-chrome-extension/background.js` - Added 184 lines (now 280 lines total)

## Build Check

Build: ✅ | Tests: ⏭️ (no automated tests for extension JS)

JavaScript syntax validation passed via `node -c background.js`
