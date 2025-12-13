# Step 2: Enhance content.js with image-to-base64 conversion

Model: sonnet | Status: ✅

## Done

- Added `convertImagesToBase64()` async function (126 lines)
  - Clones document to avoid modifying original DOM
  - Finds all `<img>` elements with src attributes
  - Uses canvas-based conversion for same-origin images
  - Two-tier fallback: canvas → fetch with credentials → skip
  - Parallel processing via Promise.allSettled()
  - Returns modified HTML string with embedded base64 images
- Added new message handler `capturePageWithImages`
  - Calls convertImagesToBase64() first
  - Returns HTML + metadata (same format as capturePageContent)
  - Proper async handling with Chrome message API
- Preserved existing `capturePageContent` handler

## Files Changed

- `cmd/quaero-chrome-extension/content.js` - Added 126 lines (now ~160 lines total)

## Build Check

Build: ✅ | Tests: ⏭️ (no automated tests for extension JS)

JavaScript syntax validation passed via `node -c content.js`
