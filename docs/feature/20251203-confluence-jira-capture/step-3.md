# Step 3: Add recording toggle UI to sidepanel.html

Model: sonnet | Status: ✅

## Done

- Added new "Recording" section above Actions section
- Created iOS-style toggle switch with id="recording-toggle"
- Added recording indicator dot with pulsing animation when active
- Added capture counter display (id="capture-count")
- Added CSS styles:
  - Toggle switch: 48x26px, blue theme (#2980b9)
  - Indicator dot: gray (stopped) → green (#27ae60) with pulse animation
  - Consistent styling with existing UI

## Files Changed

- `cmd/quaero-chrome-extension/sidepanel.html` - Added 119 lines (now 345 lines total)

## Build Check

Build: ✅ | Tests: ⏭️ (HTML validation passed)

Valid HTML5 structure, all IDs present and unique
