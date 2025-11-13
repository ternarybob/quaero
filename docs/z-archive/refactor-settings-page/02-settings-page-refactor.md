I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The current `settings.html` uses server-side template includes to embed partial components directly into the page. The user wants to convert this to a dynamic accordion pattern where:
- Partials are loaded via AJAX when accordion items are expanded
- URL query parameters track open accordions (e.g., `?a=auth-cookies,danger`)
- Multiple accordions can be open simultaneously
- All accordions start collapsed by default
- State is restored from URL on page load

All five partial files are already created and self-contained with Alpine.js `x-data` attributes. The Alpine.js components (`settingsStatus`, `settingsConfig`, `authCookies`, `authApiKeys`, `settingsDanger`) are registered in `common.js`. The user provided a clear example pattern using checkboxes with `@change` events and a `loadContent()` function.

### Approach

Replace the static template includes in `settings.html` with an accordion structure using hidden checkboxes for state management. Create an Alpine.js component `settingsAccordion` in `common.js` to handle dynamic partial loading via fetch API, URL state synchronization, and accordion restoration on page load. Use checkbox `@change` events to trigger content loading and URL updates. Implement alphabetical ordering of accordion sections: auth-apikeys, auth-cookies, config, danger, status.

### Reasoning

Listed the repository structure, read `settings.html` and `common.js` to understand the current implementation, examined the partial files to confirm they are self-contained with Alpine.js components, and reviewed the user's requirements for accordion behavior and URL state management.

## Mermaid Diagram

sequenceDiagram
    participant User
    participant Browser
    participant settingsAccordion
    participant Server
    participant Alpine

    Note over User,Alpine: Page Load Sequence
    User->>Browser: Navigate to /settings?a=status,danger
    Browser->>settingsAccordion: init()
    settingsAccordion->>settingsAccordion: Parse URL params (status, danger)
    settingsAccordion->>Browser: Check checkboxes for status, danger
    settingsAccordion->>Server: GET /partials/settings/status.html
    Server-->>settingsAccordion: HTML content
    settingsAccordion->>Alpine: Set content['status'] = HTML
    settingsAccordion->>Server: GET /partials/settings/danger.html
    Server-->>settingsAccordion: HTML content
    settingsAccordion->>Alpine: Set content['danger'] = HTML
    Alpine->>Browser: Render accordions (status & danger expanded)

    Note over User,Alpine: User Interaction Sequence
    User->>Browser: Click "Configuration" accordion
    Browser->>settingsAccordion: @change event (config, true)
    settingsAccordion->>settingsAccordion: Check if already loaded
    settingsAccordion->>Alpine: Set loading['config'] = true
    settingsAccordion->>Server: GET /partials/settings/config.html
    Server-->>settingsAccordion: HTML content
    settingsAccordion->>Alpine: Set content['config'] = HTML
    settingsAccordion->>Alpine: Set loading['config'] = false
    settingsAccordion->>settingsAccordion: updateUrl('config', true)
    settingsAccordion->>Browser: replaceState with ?a=status,danger,config
    Alpine->>Browser: Render config accordion content

    Note over User,Alpine: Close Accordion Sequence
    User->>Browser: Click "Status" accordion (to close)
    Browser->>settingsAccordion: @change event (status, false)
    settingsAccordion->>settingsAccordion: updateUrl('status', false)
    settingsAccordion->>Browser: replaceState with ?a=danger,config
    Browser->>Browser: Accordion collapses (CSS-driven)

## Proposed File Changes

### pages\settings.html(MODIFY)

References: 

- pages\static\common.js(MODIFY)

Replace the entire content section (lines 21-38) with an accordion structure wrapped in an Alpine.js component `x-data="settingsAccordion"`. Remove all static template includes (`{{template "partials/settings-status.html" .}}`, etc.).

Create five accordion items in alphabetical order:
1. **API Keys** (id: `auth-apikeys`, loads: `settings/auth-apikeys.html`)
2. **Authentication** (id: `auth-cookies`, loads: `settings/auth-cookies.html`)
3. **Configuration** (id: `config`, loads: `settings/config.html`)
4. **Danger Zone** (id: `danger`, loads: `settings/danger.html`)
5. **Service Status** (id: `status`, loads: `settings/status.html`)

Each accordion item should follow this structure:
- Hidden checkbox input with unique id and `@change` event calling `loadContent(id, url, checked)`
- Label with `.accordion-header` class containing an icon and section title
- Div with `.accordion-body` class containing:
  - Loading indicator shown when `loading[id]` is true
  - Content container with `x-html="content[id]"` shown when not loading

The checkbox pattern allows CSS-based expand/collapse without JavaScript state, while the `@change` event triggers AJAX loading and URL updates.

Keep the Service Logs section (line 36-38) below the accordion as a separate static section - it should remain a server-side template include since it's not part of the accordion.

Remove the inline script tag (lines 46-48) as component logic will be in `common.js`.

### pages\static\common.js(MODIFY)

References: 

- pages\settings.html(MODIFY)

Add a new Alpine.js component `settingsAccordion` within the `document.addEventListener('alpine:init', ...)` block (after line 24, before existing components).

**Component Properties:**
- `content` - Object to store loaded HTML content for each accordion section (keyed by section id)
- `loading` - Object to track loading state for each section (keyed by section id)
- `loadedSections` - Set to track which sections have been loaded to prevent duplicate fetches

**Component Methods:**

1. **init()** - Called on component initialization:
   - Parse URL query parameter `a` to get comma-separated list of open accordion ids
   - For each accordion id in the URL, programmatically check the corresponding checkbox to expand it
   - Call `loadContent()` for each accordion that should be open on page load

2. **loadContent(sectionId, partialUrl, isChecked)** - Called when accordion checkbox changes:
   - If `isChecked` is false (accordion closing), update URL by removing section from query parameter and return early (don't load content)
   - If section already loaded (check `loadedSections` Set), skip fetch and just update URL
   - Set `loading[sectionId] = true`
   - Fetch partial HTML from `/partials/{partialUrl}` using fetch API
   - On success: store HTML in `content[sectionId]`, add to `loadedSections`, set `loading[sectionId] = false`
   - On error: log error, show notification via `showNotification()`, set `loading[sectionId] = false`
   - Call `updateUrl(sectionId, isChecked)` to sync URL state

3. **updateUrl(sectionId, isOpen)** - Updates URL query parameter:
   - Get current URLSearchParams from `window.location.search`
   - Get current `a` parameter value (comma-separated accordion ids)
   - Split into array, add/remove `sectionId` based on `isOpen` boolean
   - Sort array alphabetically for consistent URLs
   - Update `a` parameter with new comma-separated list (or remove parameter if empty)
   - Use `window.history.replaceState()` to update URL without page reload

4. **getOpenAccordions()** - Helper to get array of currently open accordion ids from URL:
   - Parse `a` query parameter
   - Return array of section ids (empty array if parameter not present)

**Implementation Notes:**
- Use `Alpine.reactive()` for `content` and `loading` objects to ensure reactivity
- The checkbox state is managed by the browser (checked/unchecked), Alpine only handles side effects
- Content is cached after first load - subsequent opens don't re-fetch
- URL updates use `replaceState` not `pushState` to avoid polluting browser history
- Error handling should use the existing `showNotification()` function for user feedback