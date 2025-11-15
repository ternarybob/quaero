# Plan: Remove Content Left Spacing on Settings Page

## Problem Analysis
From the screenshot, the settings page has excessive left padding/margin on the content area. The `.page-container` class currently has `padding: 0 1.5rem;` which creates unwanted whitespace on both sides. The user wants to move the contents to the right with 0 padding and 0 margin while keeping everything within the page-container.

## Current State
- `.page-container` has `padding: 0 1.5rem;` (line 323-327 in quaero.css)
- This creates ~1.5rem whitespace on both left and right sides
- The settings page uses `.page-container` as the main wrapper (line 15 in settings.html)

## Desired State
- Remove left and right padding from `.page-container` to allow content to extend to edges
- Content should align flush to viewport edges
- All functionality preserved

## Steps

1. **Remove horizontal padding from `.page-container`**
   - Skill: @none
   - Files: `pages/static/quaero.css`
   - User decision: no
   - Change `.page-container` padding from `0 1.5rem` to `0` (or keep vertical padding if needed)

2. **Verify settings page layout**
   - Skill: @none
   - Files: `pages/settings.html`
   - User decision: no
   - Review HTML structure to ensure no inline styles or other padding/margin sources

## Success Criteria
- `.page-container` has 0 horizontal padding
- Content extends to left and right edges of viewport
- All Alpine.js functionality preserved
- No console errors
- Responsive behavior maintained
