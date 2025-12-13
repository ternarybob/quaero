# Task 4: Implement recording logic in sidepanel.js

Depends: 1,2,3 | Critical: no | Model: sonnet

## Addresses User Intent

Connects the UI toggle to the recording state management and content capture, enabling the core "record and browse" workflow.

## Do

1. Add event listener for recording toggle
2. Implement `toggleRecording()` function:
   - Calls background.js startRecording/stopRecording
   - Updates UI state
3. Implement `updateRecordingUI(state)` to reflect current state
4. Implement `captureCurrentPage()` function:
   - Gets current tab
   - Injects content script if needed
   - Sends `capturePageWithImages` message
   - Posts to `/api/documents/capture` endpoint
   - Calls background to track captured URL
5. Load recording state on sidepanel init
6. Update captured count display after each capture

## Accept

- [ ] Toggle starts/stops recording via background.js
- [ ] UI reflects recording state correctly
- [ ] Manual capture works with image embedding
- [ ] Server receives captured content
