# Plan: Adjust Settings Page Spacing and Sizing

## Problem Analysis

From the screenshot, I can see:
1. **Left padding issue**: The navigation menu has excessive left padding/margin (indicated by red bracket)
2. **Wide gap**: Large space between the menu and content panel (indicated by red arrow)
3. **Fixed width columns**: Using fixed `col-3` and `col-9` which may not be optimal for content

Current structure:
- Container with default padding (1.5rem on each side from `.page-container`)
- Menu in `col-3` (25% width)
- Content in `col-9` (75% width)
- Default Spectre `.columns` gap (likely 0.4rem per column = 0.8rem total)

## Desired Changes

1. **Remove left padding from navigation**: The `.nav` ul should align flush left or have minimal padding
2. **Reduce gap between columns**: Decrease space from ~2rem to ~0.5-1rem
3. **Auto-size columns**: Use `col-auto` for menu (shrink to content width) and remaining space for content

## Steps

### 1. Add custom CSS for settings page layout adjustments
   - Skill: @none
   - Files: `pages/static/quaero.css`
   - User decision: no
   - Override Spectre default `.nav` padding for settings page context
   - Add tighter column gap for settings layout
   - Implement auto-sizing columns (menu = auto, content = flex)
   - Keep responsive behavior (stack on mobile)

### 2. Update HTML structure with auto-sizing column classes
   - Skill: @none
   - Files: `pages/settings.html`
   - User decision: no
   - Change `col-3` to `col-auto` for menu column
   - Change `col-9` to remaining flex column
   - Verify Alpine.js bindings preserved
   - Maintain mobile responsiveness with `col-sm-12`

## Success Criteria
- Navigation menu has minimal/no left padding (aligns with content above)
- Gap between menu and content reduced from ~2rem to ~0.5-1rem
- Menu column auto-sizes to content width (narrower than 25%)
- Content column takes remaining available space
- Mobile responsive (stacks vertically on small screens)
- All Alpine.js functionality preserved
- No console errors
