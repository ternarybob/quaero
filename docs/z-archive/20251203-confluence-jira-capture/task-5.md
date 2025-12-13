# Task 5: Add tab navigation listener for auto-capture

Depends: 1,4 | Critical: no | Model: sonnet

## Addresses User Intent

Automatically captures each page as the user browses Confluence/Jira, enabling the "record session" workflow without manual intervention.

## Do

1. Add `chrome.tabs.onUpdated` listener in background.js
2. Filter events:
   - Only trigger on `status === 'complete'`
   - Only when recording is enabled
   - Skip chrome://, extension://, about:, file:// URLs
3. Implement auto-capture logic:
   - Debounce captures (500ms) to prevent duplicates
   - Skip if URL already captured in current session
   - Inject content script and trigger capture
   - Send to backend server
   - Update captured URLs list
4. Add optional URL pattern filter (for focusing on specific domains)

## Accept

- [ ] Pages auto-captured when recording enabled
- [ ] No duplicate captures for same URL
- [ ] Internal browser URLs skipped
- [ ] Debounce prevents rapid-fire captures
- [ ] Works with Confluence page navigations
