# Step 7: Update manifest.json for webNavigation permission

Model: sonnet | Status: ✅

## Done

- Added `webNavigation` to permissions array
- Permission enables chrome.webNavigation.onCompleted listener for auto-capture

## Files Changed

- `cmd/quaero-chrome-extension/manifest.json` - Added 1 line (webNavigation permission)

## Build Check

Build: ✅ | Tests: ⏭️

JSON validation passed, manifest is valid
