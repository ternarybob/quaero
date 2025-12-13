# Plan: Revert Settings Page to Standard Spectre CSS

## Problem Analysis

The current settings page (`pages/settings.html`) uses custom classes and CSS that deviate from Spectre CSS standards:

1. **Custom Navigation**: Uses custom `.settings-sidebar`, `.settings-menu`, `.settings-menu-item` classes instead of standard Spectre nav components
2. **Custom Grid Layout**: Uses custom CSS grid (`.settings-layout`) instead of Spectre's responsive grid system (`.container`, `.columns`, `.column`)
3. **Custom CSS Section**: Lines 1695-1822 in `quaero.css` contain extensive custom styling for the two-column layout

## Spectre CSS Standards to Implement

**Navigation Component:**
- `<ul class="nav">` - Parent container
- `<li class="nav-item">` - Each navigation item
- `<li class="nav-item active">` - Active item
- Simple, clean vertical nav

**Responsive Grid:**
- `<div class="container">` - Main container
- `<div class="columns">` - Row wrapper
- `<div class="column col-3">` - Sidebar (25% width)
- `<div class="column col-9">` - Content (75% width)
- Responsive breakpoints: `col-xs-12`, `col-sm-12`, `col-md-3`, etc.

## Steps

### 1. Update settings.html to use standard Spectre components
   - Skill: @none
   - Files: `pages/settings.html`
   - User decision: no
   - Replace custom sidebar with `<ul class="nav">` structure
   - Replace custom grid with Spectre `.container > .columns > .column` structure
   - Update Alpine.js bindings to work with new structure
   - Keep existing functionality (dynamic loading, active states)

### 2. Remove custom CSS classes from quaero.css
   - Skill: @none
   - Files: `pages/static/quaero.css`
   - User decision: no
   - Remove lines 1695-1822 (section 16: SETTINGS PAGE TWO-COLUMN LAYOUT)
   - Keep all other custom styling intact
   - Verify no other files reference removed classes

### 3. Test settings page rendering and functionality
   - Skill: @test-writer
   - Files: `test/ui/settings_test.go` (verify existing tests pass)
   - User decision: no
   - Verify page loads without console errors
   - Verify navigation works (section switching)
   - Verify responsive layout on mobile/tablet breakpoints
   - Take screenshots to document visual changes

## Success Criteria
- `pages/settings.html` uses only standard Spectre CSS classes (`nav`, `nav-item`, `container`, `columns`, `column`)
- Custom section 16 (lines 1695-1822) removed from `quaero.css`
- No references to removed classes (`.settings-layout`, `.settings-sidebar`, `.settings-menu`, `.settings-menu-item`, `.settings-content`)
- Settings page functionality preserved (dynamic loading, active states, section switching)
- Existing UI tests pass
- Page responsive on mobile/tablet (stacks vertically)
- No console errors or warnings
