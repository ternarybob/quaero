# Plan: Fix Settings Menu CSS Specificity Issues

## Problem Statement
The settings page menu is displaying horizontally instead of vertically due to CSS specificity conflicts with the Spectre CSS framework. The custom styles in `quaero.css` are being overridden by framework defaults.

## Root Cause
- Spectre CSS framework loads before custom styles
- Framework may have default button/nav styles with higher specificity
- The `<nav class="settings-menu">` element inherits Spectre's navigation styles
- Button elements inside inherit framework display properties

## Solution Approach
Strengthen CSS specificity and add explicit overrides to ensure the vertical menu layout takes precedence over Spectre CSS framework defaults. Use targeted selectors, explicit property declarations, and strategic `!important` flags.

## Steps

1. **Strengthen Settings Layout CSS Specificity**
   - Skill: @go-coder
   - Files: `pages/static/quaero.css` (lines 1695-1813)
   - User decision: no
   - Changes:
     - Increase specificity for `.settings-menu` by adding parent selectors
     - Add explicit `display: flex !important` and `flex-direction: column !important`
     - Update `.settings-menu-item` with higher specificity selectors
     - Add defensive CSS properties to prevent framework interference
     - Maintain responsive behavior while fixing desktop layout

## Success Criteria
- ✅ Menu items stack vertically in left sidebar (not horizontally)
- ✅ Clicking menu items loads content in right panel
- ✅ Layout remains two-column on desktop (>768px)
- ✅ Layout stacks vertically on mobile (<768px)
- ✅ No horizontal scrolling or layout breaks on window resize
- ✅ Full codebase compiles without errors
