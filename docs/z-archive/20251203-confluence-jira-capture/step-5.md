# Step 5: Add tab navigation listener for auto-capture

Model: sonnet | Status: ✅

## Done

- Added `chrome.tabs.onUpdated` listener for status === 'complete'
- Implemented URL filtering:
  - Skips chrome://, extension://, about:, file://, edge:// URLs
  - Via `shouldSkipUrl(url)` helper
- Implemented debouncing:
  - Uses Map (`lastCaptureTime`) to track per-tab timing
  - 1000ms threshold prevents duplicate captures
- Implemented duplicate detection:
  - `isUrlAlreadyCaptured(url)` checks session's capturedUrls
- Implemented `performAutoCapture(tabId, url)`:
  - Verifies recording state
  - Injects content script
  - Triggers capturePageWithImages
  - Sends to backend /api/documents/capture
  - Updates session state via addCapturedUrl
- Added helper functions:
  - `getServerUrl()` - retrieves from chrome.storage.sync
  - `sendCaptureToBackend()` - handles backend POST
  - `captureAuthDataForUrl()` - captures cookies for URL

## Files Changed

- `cmd/quaero-chrome-extension/background.js` - Added 224 lines (now ~504 lines total)

## Build Check

Build: ✅ | Tests: ⏭️

JavaScript syntax validation passed
