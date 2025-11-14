I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The settings page CSS already contains the correct structure for a vertical menu layout with two-column grid. The `.settings-menu` class has `flex-direction: column` set at line 1721, and the `.settings-layout` uses CSS Grid with `grid-template-columns: 250px 1fr` at line 1701. However, the user reports the menu displays horizontally, suggesting **CSS specificity conflicts** with the Spectre CSS framework (loaded from CDN in `pages/partials/head.html`).

**Root Cause Analysis:**
- Spectre CSS framework loads before custom styles (`quaero.css`)
- Spectre may have default button or nav styles with higher specificity
- The `<nav class="settings-menu">` element might inherit Spectre's navigation styles
- Button elements inside may have framework display properties overriding flex behavior

**Key Files:**
- `pages/static/quaero.css` (lines 1695-1813) - Settings layout styles
- `pages/settings.html` (lines 24-79) - HTML structure with correct classes
- `pages/partials/head.html` - Spectre CSS loaded before custom styles

**Evidence:**
- HTML structure uses correct class names (`settings-layout`, `settings-sidebar`, `settings-menu`, `settings-menu-item`)
- CSS rules are properly defined with vertical flex direction
- No conflicting `.settings-menu` rules found in codebase
- Spectre CSS framework is the only external CSS that could override styles


### Approach

Strengthen CSS specificity and add explicit overrides to ensure the vertical menu layout takes precedence over Spectre CSS framework defaults. Use targeted selectors, explicit property declarations, and strategic `!important` flags where necessary to force the intended layout behavior.

**Strategy:**
1. **Increase specificity** for `.settings-menu` by adding parent selectors or using more specific combinators
2. **Add explicit overrides** for properties that Spectre might set (display, flex-direction, flex-wrap)
3. **Ensure button elements** don't inherit conflicting display properties from Spectre's button styles
4. **Add defensive CSS** to prevent framework interference with grid layout
5. **Maintain responsive behavior** while fixing desktop layout issues

**Trade-offs:**
- Using `!important` reduces maintainability but guarantees override of framework styles
- Higher specificity selectors are more verbose but provide better control
- Adding redundant properties (like explicit `display: flex`) improves clarity and prevents edge cases


### Reasoning

Listed the repository structure to understand the project layout. Read `pages/static/quaero.css` (lines 1695-1813) to examine existing settings page styles and confirmed correct vertical menu CSS. Read `pages/settings.html` to verify HTML structure matches CSS class expectations. Searched for conflicting `.settings-menu` rules and found none. Read `pages/partials/head.html` to identify Spectre CSS framework as potential source of style conflicts. Analyzed the CSS specificity issue where framework defaults may override custom styles.


## Proposed File Changes

### pages\static\quaero.css(MODIFY)

References: 

- pages\settings.html
- pages\partials\head.html

**Strengthen CSS specificity and add explicit overrides for settings menu vertical layout (lines 1695-1813):**

1. **Update `.settings-layout` selector (around line 1699):**
   - Add explicit `display: grid !important` to prevent Spectre CSS from changing display mode
   - Ensure `grid-template-columns: 250px 1fr` is preserved
   - Add `align-items: start` to prevent unwanted stretching

2. **Update `.settings-sidebar` selector (around line 1708):**
   - Add more specific selector: `.settings-layout .settings-sidebar` to increase specificity
   - Ensure `position: sticky` and `top: 80px` are maintained
   - Add `max-width: 250px` to prevent expansion

3. **Update `.settings-menu` selector (around line 1719):**
   - Change to more specific: `.settings-sidebar .settings-menu` or `.settings-layout .settings-sidebar .settings-menu`
   - Add `display: flex !important` to override any Spectre nav defaults
   - Add `flex-direction: column !important` to force vertical layout
   - Add `flex-wrap: nowrap` to prevent wrapping to horizontal
   - Add `width: 100%` to ensure full sidebar width
   - Keep existing `gap: 0.5rem`, `list-style: none`, `padding: 0`, `margin: 0`

4. **Update `.settings-menu-item` selector (around line 1729):**
   - Change to: `.settings-menu .settings-menu-item` for higher specificity
   - Add `display: flex !important` to override Spectre button defaults
   - Add `flex-direction: row` explicitly (for icon + text horizontal alignment)
   - Add `justify-content: flex-start` to left-align content
   - Ensure `width: 100%` is set to fill sidebar width
   - Keep existing properties: `align-items: center`, `padding: 0.75rem 1rem`, `border: none`, etc.

5. **Update `.settings-menu-item:hover` selector (around line 1755):**
   - Change to: `.settings-menu .settings-menu-item:hover` for consistency
   - Keep existing hover effects

6. **Update `.settings-menu-item.active` selector (around line 1761):**
   - Change to: `.settings-menu .settings-menu-item.active` for consistency
   - Keep existing active state styles

7. **Update `.settings-content` selector (around line 1769):**
   - Add more specific: `.settings-layout .settings-content`
   - Ensure `min-height: 400px` and `overflow-y: auto` are maintained
   - Add `flex: 1` to ensure it takes remaining space

8. **Update responsive media query (around line 1787):**
   - Ensure mobile breakpoint `@media (max-width: 768px)` maintains vertical menu
   - Update selectors to match new specificity (e.g., `.settings-layout .settings-sidebar .settings-menu`)
   - Keep `grid-template-columns: 1fr` for stacked layout
   - Maintain `position: static` for sidebar on mobile

**Rationale:**
- Spectre CSS framework (loaded in `pages/partials/head.html`) may have default styles for `<nav>` elements and buttons that override custom styles
- Increasing specificity ensures custom styles take precedence without modifying framework files
- Using `!important` on critical layout properties (display, flex-direction) guarantees the vertical menu layout
- More specific selectors (parent-child combinations) naturally override framework defaults
- Explicit property declarations prevent ambiguity and edge cases

**Testing verification:**
- Menu items should stack vertically in left sidebar (not horizontally)
- Clicking menu items should load content in right panel
- Layout should remain two-column on desktop (>768px)
- Layout should stack vertically on mobile (<768px)
- No horizontal scrolling or layout breaks on window resize