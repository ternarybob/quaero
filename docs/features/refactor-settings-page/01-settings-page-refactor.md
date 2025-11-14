I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current Architecture

The settings page uses an **accordion-based layout** with the following characteristics:

1. **HTML Structure** (`pages/settings.html`):
   - 5 accordion sections: API Keys, Authentication, Configuration, Danger Zone, Service Status
   - Each section uses a checkbox + label pattern for expand/collapse
   - Content loaded lazily via `x-html` directive
   - Service Logs displayed at bottom (full-width)

2. **Alpine.js Component** (`pages/static/common.js`, lines 74-191):
   - `settingsAccordion` component manages content loading
   - Lazy loading from `/settings/*.html` endpoints (actually `/partials/settings-*.html`)
   - URL parameter tracking (`?a=section1,section2`) for deep linking
   - `loadedSections` Set prevents duplicate API calls

3. **Partial Files** (`pages/partials/settings-*.html`):
   - Each partial is self-contained with its own Alpine.js data component
   - Components: `authApiKeys`, `authCookies`, `settingsConfig`, `settingsDanger`, `settingsStatus`
   - No changes needed to these files

4. **CSS Framework**:
   - Uses Bulma/Spectre CSS with custom variables
   - Existing patterns: `.page-container`, `.card`, `.navbar`
   - Responsive breakpoint at 768px

## User Requirements

Transform the accordion layout into a **modern settings interface** with:
- **Left vertical menu** - Fixed sidebar with all section links visible
- **Right content panel** - Dynamic content area for selected section
- **Preserve functionality** - Lazy loading, URL tracking, Alpine.js components
- **Responsive design** - Stack vertically on mobile devices
- **Service Logs** - Remain full-width at bottom

Reference image shows a clean two-column layout similar to VS Code/GitHub settings pages.

### Approach

## Implementation Strategy

### 1. HTML Restructure (settings.html)
Replace the accordion structure with a **CSS Grid two-column layout**:
- Left column: Fixed-width sidebar (250px) with vertical menu
- Right column: Flexible content area (1fr)
- Service Logs: Full-width section below the grid

### 2. Alpine.js Component Refactor (common.js)
Rename and refactor `settingsAccordion` to `settingsNavigation`:
- Replace checkbox-based state with `activeSection` property
- Add `selectSection(sectionId)` method for menu clicks
- Preserve lazy loading logic and cache (`loadedSections`)
- Maintain URL parameter tracking for deep linking
- Auto-select first section or URL parameter on init

### 3. CSS Styling (quaero.css)
Add new styles for the settings layout:
- `.settings-layout` - Grid container (2 columns)
- `.settings-sidebar` - Left menu styling
- `.settings-menu-item` - Menu item with hover/active states
- `.settings-content` - Right panel with smooth transitions
- Responsive: Stack vertically below 768px with collapsible menu

### 4. Preserve Existing Behavior
- No changes to partial HTML files
- No changes to Alpine.js data components (authApiKeys, etc.)
- Maintain lazy loading from `/settings/*.html` endpoints
- Keep URL parameter format (`?a=section-id`)
- Preserve component state cache to prevent duplicate API calls

### Reasoning

I explored the codebase structure by listing directories and reading the relevant files. I examined `pages/settings.html` to understand the accordion structure, `pages/static/common.js` to analyze the Alpine.js component logic, `pages/static/quaero.css` to understand the CSS framework, and sample partial files to verify the content structure. I also reviewed the existing navigation patterns and responsive design breakpoints to ensure consistency with the application's design system.

## Mermaid Diagram

sequenceDiagram
    participant User
    participant Browser
    participant SettingsHTML as settings.html
    participant NavigationComponent as settingsNavigation (Alpine.js)
    participant Server as /settings/*.html

    User->>Browser: Navigate to /settings
    Browser->>SettingsHTML: Load page
    SettingsHTML->>NavigationComponent: Initialize x-data="settingsNavigation"
    
    NavigationComponent->>NavigationComponent: Parse URL (?a=section-id)
    NavigationComponent->>NavigationComponent: Set activeSection (URL or default)
    NavigationComponent->>NavigationComponent: selectSection(activeSection)
    
    NavigationComponent->>NavigationComponent: Check loadedSections cache
    alt Content not cached
        NavigationComponent->>Server: Fetch /settings/{section-id}.html
        Server-->>NavigationComponent: Return partial HTML
        NavigationComponent->>NavigationComponent: Store in content[section-id]
        NavigationComponent->>NavigationComponent: Add to loadedSections
    else Content cached
        NavigationComponent->>NavigationComponent: Use cached content
    end
    
    NavigationComponent->>Browser: Update URL (?a=section-id)
    NavigationComponent->>SettingsHTML: Render content via x-html
    SettingsHTML->>Browser: Display left menu + right content
    
    User->>Browser: Click different menu item
    Browser->>NavigationComponent: selectSection(new-section-id)
    NavigationComponent->>NavigationComponent: Set activeSection = new-section-id
    NavigationComponent->>NavigationComponent: Check cache & load if needed
    NavigationComponent->>Browser: Update URL & render new content
    Browser->>User: Show new section with smooth transition

## Proposed File Changes

### pages\settings.html(MODIFY)

References: 

- pages\partials\settings-auth-apikeys.html
- pages\partials\settings-auth-cookies.html
- pages\partials\settings-config.html
- pages\partials\settings-danger.html
- pages\partials\settings-status.html

**Replace the accordion structure with a two-column grid layout:**

1. **Remove the accordion wrapper** (lines 23-101):
   - Delete the `.accordion` div and all `.accordion-item` structures
   - Remove all checkbox inputs and accordion labels
   - Remove the `@change` event handlers on checkboxes

2. **Add new two-column grid structure** after the page title (line 21):
   - Create a `<div class="settings-layout">` container with two children:
     - **Left sidebar**: `<aside class="settings-sidebar">` containing a vertical menu
     - **Right content panel**: `<main class="settings-content">` for dynamic content

3. **Build the vertical menu** inside `.settings-sidebar`:
   - Create a `<nav>` element with class `settings-menu`
   - Add 5 menu items as `<button>` elements with class `settings-menu-item`:
     - API Keys (id: `auth-apikeys`, icon: `fa-key`)
     - Authentication (id: `auth-cookies`, icon: `fa-lock`)
     - Configuration (id: `config`, icon: `fa-cog`)
     - Danger Zone (id: `danger`, icon: `fa-exclamation-triangle`)
     - Service Status (id: `status`, icon: `fa-server`)
   - Each button should have:
     - `@click="selectSection('section-id')"` event handler
     - `:class="{ 'active': activeSection === 'section-id' }"` for active state
     - Font Awesome icon + text label

4. **Build the content panel** inside `.settings-content`:
   - Add loading state: `<div x-show="loading[activeSection]">` with spinner
   - Add content area: `<div x-show="!loading[activeSection]" x-html="content[activeSection]"></div>`
   - This replaces the individual accordion body sections

5. **Update Alpine.js directive** on the container:
   - Change `x-data="settingsAccordion"` to `x-data="settingsNavigation"`

6. **Keep Service Logs section unchanged** (lines 103-106):
   - This remains full-width below the settings layout

**Key structural changes:**
- Accordion: Vertical stack of expandable sections → Grid: Side-by-side menu + content
- State management: Checkbox checked/unchecked → Alpine.js `activeSection` property
- Content loading: Triggered by checkbox change → Triggered by menu button click
- Multiple sections open: Possible → Single section active: Always one selected

### pages\static\common.js(MODIFY)

References: 

- pages\settings.html(MODIFY)

**Refactor the `settingsAccordion` component to `settingsNavigation`** (lines 74-191):

1. **Rename the component**:
   - Change `Alpine.data('settingsAccordion', ...)` to `Alpine.data('settingsNavigation', ...)`

2. **Update component state properties**:
   - Keep: `content: {}` - Stores loaded HTML content by section ID
   - Keep: `loading: {}` - Tracks loading state by section ID
   - Keep: `loadedSections: new Set()` - Prevents duplicate API calls
   - **Add**: `activeSection: null` - Tracks currently selected menu item
   - **Add**: `defaultSection: 'auth-apikeys'` - Default section to show on load

3. **Refactor `init()` method** (lines 80-97):
   - Parse URL parameter `?a=section-id` (single section, not comma-separated list)
   - Set `activeSection` to URL parameter or `defaultSection`
   - Call `selectSection(activeSection)` to load initial content
   - Remove checkbox manipulation logic (no longer needed)

4. **Add new `selectSection(sectionId)` method**:
   - Set `activeSection = sectionId`
   - Determine partial URL: `/settings/${sectionId}.html`
   - Call `loadContent(sectionId, partialUrl)`
   - Update URL parameter via `updateUrl(sectionId)`
   - This replaces the checkbox `@change` event handler

5. **Refactor `loadContent(sectionId, partialUrl)` method** (lines 99-145):
   - **Remove**: `isChecked` parameter (no longer needed)
   - **Remove**: Closing accordion logic (lines 103-106)
   - Keep: Check if already loaded via `loadedSections.has(sectionId)`
   - Keep: Fetch logic with loading state management
   - Keep: Store content in `content[sectionId]`
   - Keep: Add to `loadedSections` Set
   - Simplify: Always load when called (no conditional based on checkbox state)

6. **Refactor `updateUrl(sectionId)` method** (lines 147-179):
   - **Remove**: `isOpen` parameter
   - **Simplify**: Set URL parameter `?a=${sectionId}` (single value, not array)
   - Remove: Array manipulation logic (lines 152-172)
   - Keep: `window.history.replaceState()` for URL update
   - Update debug log message

7. **Refactor `getOpenAccordions()` method** (lines 181-190):
   - **Rename** to `getActiveSection()`
   - Return single section ID string (not array)
   - Parse `?a=section-id` parameter
   - Return `null` if parameter missing

**Behavioral changes:**
- Old: Multiple sections can be open simultaneously → New: Single active section
- Old: Checkbox state drives UI → New: Alpine.js reactive property drives UI
- Old: URL tracks multiple sections (`?a=sec1,sec2`) → New: URL tracks single section (`?a=sec1`)
- Preserved: Lazy loading, content caching, loading states, error handling

### pages\static\quaero.css(MODIFY)

References: 

- pages\settings.html(MODIFY)

**Add new CSS styles for the settings page layout** (append to end of file):

1. **Settings Layout Container** (`.settings-layout`):
   - Use CSS Grid: `display: grid`
   - Two columns: `grid-template-columns: 250px 1fr`
   - Gap between columns: `gap: 2rem`
   - Margin bottom: `margin-bottom: 2rem` (space before Service Logs)
   - Min height: `min-height: 600px` (prevent layout shift)

2. **Settings Sidebar** (`.settings-sidebar`):
   - Background: `background-color: var(--card-bg)` (match card styling)
   - Border: `border: 1px solid var(--border-color)`
   - Border radius: `border-radius: var(--border-radius)`
   - Padding: `padding: 1rem`
   - Height: `align-self: start` (don't stretch to content height)
   - Sticky positioning: `position: sticky; top: 80px` (stick below header)

3. **Settings Menu** (`.settings-menu`):
   - Display: `display: flex; flex-direction: column`
   - Gap: `gap: 0.5rem` (space between menu items)
   - List style: `list-style: none; padding: 0; margin: 0`

4. **Settings Menu Item** (`.settings-menu-item`):
   - Display: `display: flex; align-items: center`
   - Padding: `padding: 0.75rem 1rem`
   - Border: `border: none; border-radius: var(--border-radius)`
   - Background: `background-color: transparent`
   - Color: `color: var(--text-primary)`
   - Font: `font-size: 0.9rem; font-weight: 500; text-align: left`
   - Cursor: `cursor: pointer`
   - Transition: `transition: all 0.2s ease`
   - Width: `width: 100%`
   - Icon spacing: `i { margin-right: 0.75rem; width: 1.25rem; text-align: center; }`

5. **Menu Item Hover State** (`.settings-menu-item:hover`):
   - Background: `background-color: rgba(255, 255, 255, 0.05)`
   - Transform: `transform: translateX(2px)` (subtle slide effect)

6. **Menu Item Active State** (`.settings-menu-item.active`):
   - Background: `background-color: var(--color-primary)`
   - Color: `color: white`
   - Font weight: `font-weight: 600`
   - Box shadow: `box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2)`

7. **Settings Content Panel** (`.settings-content`):
   - Background: `background-color: var(--card-bg)`
   - Border: `border: 1px solid var(--border-color)`
   - Border radius: `border-radius: var(--border-radius)`
   - Padding: `padding: 1.5rem`
   - Min height: `min-height: 400px`
   - Overflow: `overflow-y: auto` (scroll if content is tall)
   - Transition: `transition: opacity 0.2s ease` (smooth content swap)

8. **Content Loading State** (`.settings-content .loading-state`):
   - Text align: `text-align: center`
   - Padding: `padding: 3rem`
   - Color: `color: var(--text-secondary)`

9. **Responsive Design** (media query `@media (max-width: 768px)`):
   - **Layout**: Change to single column: `grid-template-columns: 1fr`
   - **Sidebar**: Full width, not sticky: `position: static`
   - **Menu**: Horizontal scroll or collapsible accordion pattern
   - **Alternative**: Use a mobile-friendly tab bar at top instead of sidebar
   - **Content**: Full width with adequate padding

10. **Service Logs Section** (ensure compatibility):
    - Verify `.settings-layout` + Service Logs section stack correctly
    - Service Logs should remain full-width below the grid
    - No changes needed to existing Service Logs styles

**Design considerations:**
- Match existing color scheme using CSS variables (`var(--color-primary)`, etc.)
- Consistent border radius and spacing with existing cards
- Smooth transitions for better UX
- Sticky sidebar for easy navigation on long content
- Mobile-first responsive approach