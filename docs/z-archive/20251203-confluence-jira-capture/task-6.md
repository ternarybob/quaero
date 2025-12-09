# Task 6: Add capture history display in sidepanel

Depends: 4 | Critical: no | Model: sonnet

## Addresses User Intent

Shows users what pages have been captured in the current session, providing visibility and confidence that content is being saved.

## Do

1. Add "Captured Pages" section to sidepanel.html
2. Create scrollable list container (max-height with overflow)
3. Display for each captured page:
   - Page title (truncated if long)
   - Timestamp
   - Status indicator (success/pending)
4. Implement `updateCaptureHistory()` in sidepanel.js
5. Load history from background.js session data
6. Update list on each new capture
7. Add "Clear History" button (optional)

## Accept

- [ ] Captured pages list visible in sidepanel
- [ ] List updates as new pages captured
- [ ] Shows page title and timestamp
- [ ] Scrollable when list is long
