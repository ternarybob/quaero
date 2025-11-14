# Plan: Transform Settings Page from Accordion to Two-Column Menu Layout

## Overview
Refactor the settings page from an accordion-based layout to a modern two-column interface with a fixed vertical menu sidebar and dynamic content panel. Preserve all existing functionality including lazy loading, URL tracking, and Alpine.js components.

## Steps

### 1. **Restructure HTML to Two-Column Grid Layout**
- Skill: @go-coder
- Files: `pages/settings.html`
- User decision: no
- **What:** Replace accordion structure with CSS Grid layout containing left sidebar menu and right content panel
- **Why:** Transform UI from vertical accordion to horizontal split-pane layout

### 2. **Refactor Alpine.js Component from Accordion to Navigation**
- Skill: @go-coder
- Files: `pages/static/common.js`
- User decision: no
- **What:** Rename `settingsAccordion` to `settingsNavigation`, replace checkbox state with `activeSection` property, add `selectSection()` method
- **Why:** Change state management from checkbox-driven to reactive property-driven navigation

### 3. **Add CSS Styles for Two-Column Settings Layout**
- Skill: @go-coder
- Files: `pages/static/quaero.css`
- User decision: no
- **What:** Add grid layout styles, sidebar menu styles, active states, responsive breakpoints
- **Why:** Provide visual styling for new two-column layout with menu interactions

## Success Criteria
- ✅ Settings page displays two-column layout (sidebar + content panel)
- ✅ Vertical menu shows all 5 sections with icons and labels
- ✅ Clicking menu item loads content in right panel
- ✅ Active menu item highlighted with `.active` class
- ✅ Only one section active at a time
- ✅ Lazy loading preserved (content fetched once)
- ✅ URL tracking works (`?a=section-id` format)
- ✅ Service Logs remain full-width at bottom
- ✅ Responsive: stacks vertically on mobile (<768px)
- ✅ All existing Alpine.js components (authApiKeys, etc.) work unchanged
- ✅ Full codebase compiles without errors
- ✅ No console errors on page load or navigation

## Design Decisions (Auto-Resolved)
- **Grid columns:** 250px sidebar + 1fr content (standard two-column split)
- **State management:** Single `activeSection` string property (one active section at a time)
- **URL format:** `?a=section-id` (single section, not comma-separated)
- **Default section:** `auth-apikeys` (first menu item)
- **Responsive strategy:** Vertical stack below 768px (mobile-first approach)

## Notes
- No changes needed to partial HTML files (`pages/partials/settings-*.html`)
- No changes needed to individual Alpine.js components (`authApiKeys`, `authCookies`, etc.)
- Service Logs section remains unchanged (full-width at bottom)
- Existing test suite will need updates (accordion selectors → menu button selectors)
