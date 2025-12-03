# Task 1: Add recording state management to background.js

Depends: - | Critical: no | Model: sonnet

## Addresses User Intent

Provides persistent recording state management so users can toggle "record mode" and have it persist across page navigations. This is foundational for the session recording feature.

## Do

1. Add recording state variables to background.js
2. Implement `startRecording()` function that:
   - Generates unique session ID
   - Sets recording=true in chrome.storage.local
   - Initializes empty captured URLs array
3. Implement `stopRecording()` function that:
   - Sets recording=false
   - Preserves session history for review
4. Implement `getRecordingState()` function
5. Add message handlers for: `startRecording`, `stopRecording`, `getRecordingState`
6. Add `addCapturedUrl(url, docId)` to track captures

## Accept

- [ ] Recording state persists in chrome.storage.local
- [ ] Session ID is generated when recording starts
- [ ] Message handlers respond to popup/sidepanel requests
- [ ] Captured URLs tracked per session
