# Task 2: Enhance content.js with image-to-base64 conversion

Depends: - | Critical: no | Model: sonnet

## Addresses User Intent

Captures images as embedded data (not just links) so Confluence/Jira page content including diagrams, screenshots, and attachments is preserved for AI analysis.

## Do

1. Add `convertImagesToBase64()` async function that:
   - Finds all `<img>` elements with src attributes
   - Fetches image data using canvas drawImage approach
   - Converts to base64 data URI format
   - Replaces original src with data URI
   - Handles errors gracefully (skip failed images)
2. Add new message handler `capturePageWithImages` that:
   - Calls `convertImagesToBase64()` first
   - Then captures full HTML (with embedded images)
   - Returns HTML + metadata
3. Handle CORS limitations:
   - Use canvas approach for same-origin images
   - Skip or use placeholder for cross-origin images that fail

## Accept

- [ ] Images converted to base64 data URIs in captured HTML
- [ ] Cross-origin images handled gracefully (skip or placeholder)
- [ ] Original page DOM not permanently modified (clone or restore)
- [ ] New message handler `capturePageWithImages` works
