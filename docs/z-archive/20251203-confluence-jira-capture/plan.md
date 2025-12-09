# Plan: Confluence/Jira Session Recording Capture

Type: feature | Workdir: ./docs/feature/20251203-confluence-jira-capture/

## User Intent (from manifest)

Enable capturing content from Confluence and Jira (JavaScript-rendered, authentication-required enterprise wiki pages) where:
1. Headless browser access is blocked by the platform
2. User authentication/cookies are required
3. JavaScript rendering is mandatory for content visibility
4. Embedded images need to be captured (not just links)

The captured content should be queryable/summarizable by AI for knowledge management purposes.

## Architecture Decision

**Selected: Option 2 - Session Recording Mode**

This approach adds a "recording mode" toggle to the extension sidepanel. When enabled:
- Each page navigation automatically captures the rendered HTML
- Images are converted to base64 data URIs for inline embedding
- Content is sent to backend `/api/documents/capture` endpoint
- Recording state persists via `chrome.storage`

## Tasks

| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Add recording state management to background.js | - | no | sonnet |
| 2 | Enhance content.js with image-to-base64 conversion | - | no | sonnet |
| 3 | Add recording toggle UI to sidepanel.html | - | no | sonnet |
| 4 | Implement recording logic in sidepanel.js | 1,2,3 | no | sonnet |
| 5 | Add tab navigation listener for auto-capture | 1,4 | no | sonnet |
| 6 | Add capture history display in sidepanel | 4 | no | sonnet |
| 7 | Update manifest.json for webNavigation permission | - | no | sonnet |

## Order

[1,2,3,7] → [4] → [5,6]

## Technical Details

### Recording State (Task 1)
- Store in `chrome.storage.local`: `{ recording: boolean, sessionId: string, capturedUrls: [] }`
- Session ID generated on recording start
- Capture history persisted per session

### Image Conversion (Task 2)
- Find all `<img>` elements in DOM
- Fetch image data via canvas or fetch API
- Convert to base64 data URI
- Replace `src` attribute with data URI
- Handle CORS by using `fetch` with credentials

### UI Toggle (Task 3)
- Add toggle switch in sidepanel above actions
- Visual indicator: green dot when recording
- Show captured page count

### Auto-Capture (Task 5)
- Listen to `chrome.tabs.onUpdated` for `complete` status
- Filter to only capture when recording enabled
- Debounce to prevent duplicate captures
- Skip chrome://, extension://, about: URLs
