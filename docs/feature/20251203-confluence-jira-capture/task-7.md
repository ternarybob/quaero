# Task 7: Update manifest.json for webNavigation permission

Depends: - | Critical: no | Model: sonnet

## Addresses User Intent

Ensures the extension has proper permissions to monitor page navigations for auto-capture functionality.

## Do

1. Add `webNavigation` to permissions array in manifest.json
2. Verify existing permissions are sufficient:
   - `cookies` - for auth capture
   - `activeTab` - for current tab access
   - `tabs` - for tab queries
   - `storage` - for state persistence
   - `scripting` - for content script injection

## Accept

- [ ] `webNavigation` permission added
- [ ] Manifest.json remains valid JSON
- [ ] Extension can be loaded without errors
