I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The codebase currently uses **Cirrus UI v0.7.1** with extensive customizations in \`quaero.css\` (630+ lines). The application consists of:

- **15 HTML files**: 7 main pages (index, config, chat, sources, jobs, documents, auth) + 5 partials (head, navbar, footer, snackbar, service-logs, service-status) + 3 additional files (settings, style.html in docs)
- **Alpine.js integration**: Used for reactive components (serviceLogs, sourceManagement, appStatus, configPage, authPage)
- **WebSocket integration**: Managed via \`websocket-manager.js\` singleton for real-time updates
- **Custom JavaScript**: \`common.js\` contains Alpine.js component definitions
- **Highlight.js**: Used for code syntax highlighting in job/document detail views

**Current Cirrus UI patterns identified:**
- Navigation: \`.header\`, \`.header-nav\`, \`.nav-item\`, \`.nav-left\`, \`.nav-center\`, \`.nav-right\`
- Containers: \`.frame\`, \`.frame__header\`, \`.frame__body\`, \`.tile\`, \`.tile__container\`, \`.tile__buttons\`
- Buttons: \`.btn\`, \`.btn--sm\`, \`.btn-primary\`, \`.btn-success\`, \`.btn-danger\`, \`.btn-info\`, \`.outline\`
- Tags: \`.tag\`, \`.tag--success\`, \`.tag--danger\`, \`.tag--warning\`, \`.tag--info\`, \`.tag--primary\`
- Tables: \`.table\` with custom styling
- Forms: \`.field\`, \`.control\`, \`.input\`, \`.select\`, \`.checkbox\`, \`.label\`, \`.help\`
- Modals: \`.modal\`, \`.modal-card\`, \`.modal-card-head\`, \`.modal-card-body\`, \`.modal-card-foot\`
- Utilities: Custom spacing (\`.q-*\`), flex (\`.q-d-flex\`), text (\`.q-text-*\`)

**Metro UI v5 component mappings discovered:**
- AppBar: \`data-role=\"appbar\"\` with \`data-expand-point=\"md\"\` for responsive
- Panels/Cards: \`.panel\`, \`.card\` with \`.panel-header\`/\`.card-header\`, \`.panel-content\`/\`.card-content\`
- Buttons: \`.button\` with modifiers (\`.primary\`, \`.success\`, \`.danger\`, \`.outline\`, \`.small\`)
- Tags/Badges: \`.tag\` with \`.title\` and \`.action\`, \`.badge\` with color utilities
- Tables: \`.table\` with modifiers (\`.striped\`, \`.hovered\`, \`.bordered\`)
- Forms: \`data-role=\"input\"\`, \`data-role=\"select\"\`, \`data-role=\"textarea\"\`, \`.checkbox\` wrapper
- Dialogs: \`data-role=\"dialog\"\` or JS API \`Metro.dialog.create()\`
- Toast/Notify: \`Metro.toast.create()\` and \`Metro.notify.create()\` APIs
- Grid: \`.container\`, \`.grid\`, \`.row\`, \`.cells{N}\`, \`.cell\`, \`.colspan{N}\`

**Key integration points:**
- WebSocket status updates in navbar (\`.status-text\` selector)
- Alpine.js DOM bindings (x-data, x-show, x-text, @click)
- Modal visibility controlled by Alpine state
- Table rendering via JavaScript innerHTML
- Pagination button selectors
- Snackbar global function \`window.showNotification\`

### Approach

**Migration Strategy:**

1. **Framework Replacement**: Replace Cirrus UI CDN with Metro UI v5 CDN in \`head.html\`
2. **CSS Minimization**: Strip \`quaero.css\` to only color theming using CSS variables that Metro UI can consume
3. **Component-by-Component Migration**: Refactor each partial and page to use Metro UI default components with minimal custom classes
4. **JavaScript Adaptation**: Update DOM selectors and class references in Alpine.js components and custom scripts
5. **Testing Points**: Ensure Alpine.js reactivity, WebSocket updates, modal interactions, and table pagination continue working

**Color Theming Approach:**
- Define semantic color tokens in \`quaero.css\` using CSS variables
- Map to Metro UI's color system (primary, success, danger, warning, info)
- Use Metro's built-in color utilities (\`bg-*\`, \`fg-*\`) where possible
- Keep only essential overrides for brand colors

**Component Migration Priority:**
1. Framework setup (head.html)
2. Core layout (navbar, footer)
3. Shared components (snackbar, service-logs, service-status)
4. Simple pages (index, config)
5. Complex pages (chat, sources, jobs, documents, auth)

### Reasoning

I explored the codebase by reading all 15 HTML files and the CSS/JS assets. I identified the current Cirrus UI patterns and customizations. I searched the web for Metro UI v5 documentation to understand CDN links, component structure, class naming conventions, and JavaScript APIs. I mapped Cirrus UI patterns to their Metro UI equivalents and identified integration points with Alpine.js and WebSocket functionality.

## Proposed File Changes

### pages\\partials\\head.html(MODIFY)

Replace Cirrus UI CDN link (line 8) with Metro UI v5 CDN:
- Remove: \`<link rel=\"stylesheet\" href=\"https://unpkg.com/cirrus-ui@0.7.1/dist/cirrus.min.css\">\`
- Add: \`<link rel=\"stylesheet\" href=\"https://cdn.metroui.org.ua/current/metro.css\">\`
- Add Metro UI icons: \`<link rel=\"stylesheet\" href=\"https://cdn.metroui.org.ua/current/icons.css\">\`
- Add Metro UI JavaScript before Alpine.js (after line 25): \`<script src=\"https://cdn.metroui.org.ua/current/metro.js\"></script>\`

Update comment on line 10 to reflect Metro UI instead of Cirrus UI.

Keep Font Awesome, Highlight.js, websocket-manager.js, common.js, and Alpine.js as-is since they are framework-agnostic.

### pages\\static\\quaero.css(MODIFY)

**Strip down to minimal color theming only.** Remove all sections except color variables.

**Keep only:**
- Root color variables (lines 7-16) but update to Metro UI semantic naming:
  - \`--color-primary: #0757ba;\` (software blue)
  - \`--color-secondary: #6c757d;\` (neutral gray)
  - \`--color-success: #1f883d;\` (green)
  - \`--color-danger: #d1242f;\` (red)
  - \`--color-warning: #ffdd57;\` (yellow)
  - \`--color-info: #0757ba;\` (blue)

**Remove entirely:**
- All spacing variables (lines 22-34)
- Navbar height variable (lines 36-38)
- Body & layout styles (lines 41-61)
- Navbar overrides (lines 63-108)
- Footer overrides (lines 120-127)
- Border utilities (lines 129-137)
- Typography overrides (lines 139-162)
- Mobile responsiveness (lines 163-207)
- Component overrides for frames, tables, logs (lines 209-384)
- Snackbar styles (lines 386-427)
- Modal styles (lines 429-529)
- Form element styles (lines 531-620)
- Button overrides (lines 622-631)

**Add Metro UI color integration:**
Create CSS rules that apply the color variables to Metro UI components:
\`\`\`css
.button.primary { background: var(--color-primary); }
.button.success { background: var(--color-success); }
.button.danger { background: var(--color-danger); }
.tag.primary { background: var(--color-primary); color: white; }
.tag.success { background: var(--color-success); color: white; }
.tag.danger { background: var(--color-danger); color: white; }
\`\`\`

Final file should be ~50-80 lines maximum (down from 631 lines).

### pages\\partials\\navbar.html(MODIFY)

References: 

- pages\\static\\websocket-manager.js

Replace Cirrus UI navbar with Metro UI AppBar component.

**Replace entire structure (lines 1-44):**
- Remove: \`<nav class=\"header header-fixed bg-white q-border-bottom\">\`
- Add: \`<nav data-role=\"appbar\" data-expand-point=\"md\" class=\"pos-fixed\">\`

**Brand section:**
- Replace \`<div class=\"nav-item\"><a href=\"/\" class=\"q-brand\">Quaero</a></div>\` with \`<a href=\"/\" class=\"brand no-hover\"><span class=\"text-bold\">Quaero</span></a>\`

**Navigation menu:**
- Replace \`<div class=\"nav-center\">\` with \`<ul class=\"app-bar-menu\">\`
- Replace each \`<div class=\"nav-item\"><a href=\"/\">HOME</a></div>\` with \`<li><a href=\"/\">HOME</a></li>\`
- Keep Go template conditionals for active state: \`{{if eq .Page \"home\"}}class=\"active\"{{end}}\`
- Close with \`</ul>\`

**Status indicator (right side):**
- Replace \`<div class=\"nav-right\">\` with \`<div class=\"app-bar-item place-right\">\`
- Replace \`<span class=\"tag tag--success status-text\">ONLINE</span>\` with \`<span class=\"badge success status-text\">ONLINE</span>\`

**Mobile toggle:**
- Remove custom hamburger button (lines 8-12) - Metro UI AppBar handles this automatically with \`data-expand-point=\"md\"\`

**JavaScript updates (lines 46-82):**
- Remove mobile menu toggle logic (lines 48-56) - Metro UI handles this
- Keep WebSocket status update function (lines 58-72) but update selectors:
  - Change \`statusText.classList.remove('tag--danger')\` to \`statusText.classList.remove('danger')\`
  - Change \`statusText.classList.add('tag--success')\` to \`statusText.classList.add('success')\`
  - Keep \`.status-text\` selector
- Keep WebSocket subscription logic (lines 74-80)

Remove all custom CSS classes (\`.q-border-bottom\`, \`.q-brand\`) as Metro UI provides default styling.

### pages\\partials\\footer.html(MODIFY)

Replace Cirrus UI footer with Metro UI footer styling.

**Replace footer element (line 1):**
- Remove: \`<footer class=\"bg-white u-text-center q-p-3 q-border-top\">\`
- Add: \`<footer class=\"container-fluid text-center p-4 border-top\">\`

**Keep version display div (lines 2-5) unchanged** - just the container ID and content.

**JavaScript (lines 7-20) - no changes needed** - API call and DOM manipulation remain the same.

Remove custom utility classes (\`.q-p-3\`, \`.q-border-top\`) and use Metro UI equivalents (\`.p-4\`, \`.border-top\`).

### pages\\partials\\snackbar.html(MODIFY)

References: 

- pages\\partials\\head.html(MODIFY)

Replace custom snackbar implementation with Metro UI Toast API.

**Remove entire custom HTML structure (line 1):**
- Delete: \`<div id=\"snackbar\" style=\"position: fixed; bottom: 20px; right: 20px; z-index: 9999; display: none;\"></div>\`

**Replace JavaScript function (lines 3-19):**
- Remove custom \`showSnackbar\` function
- Replace with Metro UI Toast wrapper:
\`\`\`javascript
function showSnackbar(message, type = 'info') {
    const typeMap = {
        'info': 'bg-blue fg-white',
        'success': 'bg-green fg-white',
        'warning': 'bg-yellow fg-dark',
        'error': 'bg-red fg-white',
        'danger': 'bg-red fg-white'
    };
    const classes = typeMap[type] || 'bg-blue fg-white';
    Metro.toast.create(message, null, 3000, classes);
}

// Keep alias for backwards compatibility
window.showNotification = showSnackbar;
\`\`\`

**Note:** Metro UI Toast requires \`metro.js\` to be loaded (already added in \`head.html\`). The Toast will auto-position at bottom-right by default.

### pages\\partials\\service-logs.html(MODIFY)

References: 

- pages\\static\\common.js(MODIFY)

Replace Cirrus UI frame/tile structure with Metro UI panel.

**Replace outer container (line 1):**
- Remove: \`<div class=\"frame\" x-data=\"serviceLogs\">\`
- Add: \`<div class=\"panel\" x-data=\"serviceLogs\">\`

**Replace header section (lines 3-25):**
- Remove: \`<div class=\"frame__header\">\`
- Add: \`<div class=\"panel-header\">\`
- Remove tile structure (lines 5-24)
- Replace with simpler header layout:
\`\`\`html
<div class=\"d-flex flex-justify-between flex-align-center\">
    <h6 class=\"m-0\">Service Logs</h6>
    <div class=\"d-flex flex-gap-2\">
        <button class=\"button small outline\" ...>...</button>
        <button class=\"button small outline\" ...>...</button>
        <button class=\"button small outline danger\" ...>...</button>
    </div>
</div>
\`\`\`

**Replace body section (lines 27-31):**
- Remove: \`<div class=\"frame__body\">\`
- Add: \`<div class=\"panel-content\">\`
- Keep \`<pre class=\"log-container\">\` but add Metro UI styling class: \`<pre class=\"log-container bg-light p-4\" style=\"max-height: 500px; overflow-y: auto;\">\`

**Update button classes:**
- Replace \`.btn\` with \`.button\`
- Replace \`.btn--sm\` with \`.small\`
- Replace \`.btn-danger\` with \`.danger\`
- Keep \`.outline\` as-is (Metro UI supports it)

**Keep Alpine.js bindings unchanged** (x-ref, @click, :title, :class, x-text, x-for, x-if) - these are framework-agnostic.

**Note:** Log styling (colors, scrollbar) will be removed from \`quaero.css\`, so add inline styles or minimal Metro UI classes for the log container appearance.

### pages\\partials\\service-status.html(MODIFY)

Replace Cirrus UI frame structure with Metro UI panel.

**Replace outer container (line 1):**
- Remove: \`<div class=\"frame\">\`
- Add: \`<div class=\"panel\">\`

**Replace header (lines 3-5):**
- Remove: \`<div class=\"frame__header\">\`
- Add: \`<div class=\"panel-header\">\`
- Keep heading: \`<h6 class=\"m-0\">Quick Actions</h6>\`

**Replace body (lines 7-33):**
- Remove: \`<div class=\"frame__body\">\`
- Add: \`<div class=\"panel-content\">\`

**Update button container (line 9):**
- Remove: \`<div class=\"q-d-flex q-gap-2\">\`
- Add: \`<div class=\"d-flex flex-gap-2 mb-4\">\`

**Update button classes (lines 10-25):**
- Replace \`.btn\` with \`.button\`
- Replace \`.btn-primary\` with \`.primary\`
- Replace \`.btn-info\` with \`.info\` (or use default)
- Replace \`.btn-success\` with \`.success\`
- Replace \`.btn--sm\` with \`.small\`
- Keep \`.outline\` as-is

**Update text styling (lines 28-31):**
- Replace \`.text-md\` with \`.text-small\`
- Remove \`.q-text-muted\` and use Metro UI's \`.text-muted\` or \`.fg-gray\`

**Keep all href links and icon markup unchanged** - Font Awesome icons work with Metro UI.

### pages\\index.html(MODIFY)

References: 

- pages\\partials\\navbar.html(MODIFY)
- pages\\partials\\footer.html(MODIFY)
- pages\\partials\\service-status.html(MODIFY)
- pages\\partials\\service-logs.html(MODIFY)

Migrate index page to Metro UI components.

**Update body class (line 9):**
- Remove: \`<body class=\"has-navbar-fixed-top\">\`
- Add: \`<body class=\"h-100\">\` with inline style \`<body style=\"padding-top: 60px;\">\` to account for fixed AppBar

**Update content container (line 13):**
- Keep: \`<div class=\"content\">\` but add Metro UI container class
- Change to: \`<div class=\"container-fluid mt-4\">\`

**Update heading section (lines 16-21):**
- Remove custom padding class \`.pl-2\`
- Wrap in Metro UI grid:
\`\`\`html
<div class=\"row mb-4\">
    <div class=\"cell\">
        <h1>Quaero</h1>
        <p class=\"text-leader\">Unified Data Collection and Analysis Platform</p>
    </div>
</div>
\`\`\`
- Replace \`.title\` with \`<h1>\` (Metro UI default)
- Replace \`.subtitle\` with \`.text-leader\` (Metro UI subheading class)

**Keep partial includes (lines 23, 25) unchanged** - they will be migrated separately:
- \`{{template \"service-status.html\" .}}\`
- \`{{template \"service-logs.html\" .}}\`

**Keep footer and snackbar includes (lines 29, 31) unchanged.**

No JavaScript changes needed - this is a simple layout page.

### pages\\config.html(MODIFY)

References: 

- pages\\partials\\service-logs.html(MODIFY)

Migrate config page to Metro UI components.

**Update body and container (lines 9, 13):**
- Same changes as \`index.html\`: remove \`.has-navbar-fixed-top\`, add padding-top, change \`.content\` to \`.container-fluid mt-4\`

**Update heading section (lines 16-21):**
- Same grid wrapper as \`index.html\`

**Update Service Status panel (lines 24-40):**
- Replace \`.frame\` with \`.panel\`
- Replace \`.frame__header\` with \`.panel-header\`
- Replace \`.frame__body\` with \`.panel-content\`
- Update header layout:
\`\`\`html
<div class=\"panel-header\">
    <div class=\"d-flex flex-justify-between flex-align-center\">
        <h6 class=\"m-0\">Service Status</h6>
        <span :class=\"'badge ' + (isOnline ? 'success' : 'danger')\" id=\"service-status\">
            <span x-text=\"isOnline ? 'Online' : 'Offline'\"></span>
        </span>
    </div>
</div>
\`\`\`
- Replace \`.tag\` with \`.badge\`
- Replace \`.tag--success\` with \`.success\`, \`.tag--danger\` with \`.danger\`
- Remove custom flex classes (\`.q-d-flex\`, \`.q-align-center\`) and use Metro UI equivalents (\`.d-flex\`, \`.flex-align-center\`)

**Update Configuration Details panel (lines 42-49):**
- Same panel structure changes
- Update pre element styling:
  - Remove custom classes (\`.q-bg-light\`, \`.q-p-3\`, \`.q-rounded\`, \`.q-overflow-auto\`, \`.q-max-h-300\`)
  - Add Metro UI classes: \`<pre class=\"bg-light p-4 border\" style=\"max-height: 300px; overflow: auto;\">\`

**Keep Alpine.js component (lines 59-100) unchanged** - only DOM/class references need updating, logic stays the same.

**Keep service-logs partial include (line 51) unchanged.**

### pages\\chat.html(MODIFY)

References: 

- pages\\partials\\service-logs.html(MODIFY)

Migrate chat page to Metro UI components.

**Update body and container (lines 9, 13):**
- Same changes as other pages

**Update heading section (lines 16-21):**
- Same grid wrapper

**Update chat panel (lines 23-71):**
- Replace \`.frame\` with \`.panel\`
- Replace \`.frame__header\` with \`.panel-header\`
- Replace \`.frame__body\` with \`.panel-content\`

**Update message input field (lines 34-38):**
- Replace \`.field\` with \`.form-group\`
- Replace \`.control\` with no wrapper (Metro UI doesn't need it)
- Update textarea: \`<textarea id=\"user-message\" data-role=\"textarea\" rows=\"3\" placeholder=\"Type your message...\" style=\"min-height: 5rem; resize: vertical;\"></textarea>\`
- Remove \`.input\` class, add \`data-role=\"textarea\"\`

**Update controls section (lines 41-58):**
- Replace custom flex classes with Metro UI:
  - \`.q-d-flex\` → \`.d-flex\`
  - \`.q-align-center\` → \`.flex-align-center\`
  - \`.q-justify-between\` → \`.flex-justify-between\`
  - \`.q-gap-2\` → \`.flex-gap-2\`
  - \`.q-flex-wrap\` → \`.flex-wrap\`

**Update checkbox (lines 43-46):**
- Wrap in Metro UI checkbox structure:
\`\`\`html
<label class=\"checkbox\">
    <input type=\"checkbox\" id=\"rag-enabled\" checked>
    <span class=\"check\"></span>
    <span class=\"caption\">Enable RAG (Document Retrieval) - Requires embedding support</span>
</label>
\`\`\`

**Update buttons (lines 49-56):**
- Replace \`.btn\` with \`.button\`
- Replace \`.btn-primary\` with \`.primary\`
- Replace \`.btn-danger\` with \`.danger\`
- Replace \`.btn--sm\` with \`.small\`
- Keep \`.outline\`

**Update status section (lines 61-69):**
- Replace custom text classes with Metro UI equivalents
- Remove \`.q-mt-3\`, \`.q-text-muted\`, \`.q-mb-2\` and use \`.mt-4\`, \`.text-muted\`, \`.mb-2\`

**JavaScript (lines 81-430) - minimal changes:**
- Update class references in \`addTechnicalMetadata\` function (lines 115-150):
  - Replace \`.q-bg-dark\` with \`.bg-dark\`
  - Replace \`.q-mt-2\` with \`.mt-2\`
- Update status HTML generation (lines 362-404):
  - Replace \`.q-d-flex\`, \`.q-flex-wrap\`, \`.q-mr-3\`, \`.q-align-center\`, \`.q-gap-1\` with Metro UI equivalents
  - Replace \`.q-text-success\`, \`.q-text-danger\`, \`.q-text-warning\` with \`.fg-green\`, \`.fg-red\`, \`.fg-orange\`
- Keep all logic, API calls, and event handlers unchanged

### pages\\sources.html(MODIFY)

References: 

- pages\\static\\common.js(MODIFY)
- pages\\partials\\service-logs.html(MODIFY)

Migrate sources page to Metro UI components.

**Update body and container (lines 9, 13):**
- Same changes as other pages

**Update heading section (lines 16-21):**
- Same grid wrapper

**Update Application Status panel (lines 24-41):**
- Replace \`.frame\` with \`.panel\`
- Replace \`.frame__header\` with \`.panel-header\`, \`.frame__body\` with \`.panel-content\`
- Update header flex layout:
  - Replace \`.u-flex\` with \`.d-flex\`
  - Replace \`.u-items-center\` with \`.flex-align-center\`
  - Replace \`.u-justify-space-between\` with \`.flex-justify-between\`
- Replace \`.tag\` with \`.badge\`
- Update \`getStatusColor\` function to return Metro UI badge classes: \`'success'\`, \`'danger'\`, \`'info'\` instead of \`'is-success'\`, etc.

**Update Sources List panel (lines 46-121):**
- Same panel structure changes
- Update header button group (lines 52-60):
  - Replace \`.u-flex\`, \`.u-items-center\`, \`.u-gap-2\` with Metro UI equivalents
  - Replace \`.btn\` with \`.button\`, \`.btn--sm\` with \`.small\`, \`.btn-info\` with \`.info\`

**Update table (lines 65-119):**
- Add Metro UI table classes: \`<table class=\"table striped hovered\">\`
- Update table header alignment: replace \`.q-text-center\` with \`.text-center\`
- Update tag classes in table body:
  - Replace \`.tag--info\` with \`.info\`, \`.tag--primary\` with \`.primary\`
  - Replace \`.tag--success\` with \`.success\`
- Update button classes in actions column (lines 109-114):
  - Replace \`.btn\` with \`.button\`, \`.btn--sm\` with \`.small\`
  - Replace \`.btn-danger\` with \`.danger\`
  - Remove \`.q-mr-1\` and use \`.mr-1\`

**Update Create/Edit Modal (lines 124-226):**
- Replace \`.modal\` with Metro UI dialog structure:
\`\`\`html
<div data-role=\"dialog\" :class=\"{'open': showCreateModal || showEditModal}\" data-overlay=\"true\">
    <div class=\"dialog-content\">
        <div class=\"dialog-header\">
            <h6 x-text=\"showEditModal ? 'Edit Source' : 'Add New Source'\"></h6>
            <button class=\"button square small\" @click=\"closeModal()\">
                <span class=\"mif-cross\"></span>
            </button>
        </div>
        <div class=\"dialog-body\">
            <!-- form fields -->
        </div>
        <div class=\"dialog-actions\">
            <button class=\"button success small outline\" @click=\"saveSource()\">Save</button>
            <button class=\"button small outline\" @click=\"closeModal()\">Cancel</button>
        </div>
    </div>
</div>
\`\`\`
- Replace all form fields:
  - \`.field\` → \`.form-group\`
  - \`.label\` → \`<label>\` (Metro UI default)
  - \`.control\` → remove wrapper
  - \`.input\` → add \`data-role=\"input\"\`
  - \`.select\` → add \`data-role=\"select\"\` on \`<select>\`
  - \`.checkbox\` → wrap with Metro UI checkbox structure
- Replace \`.label.is-small\` with \`.text-small\`
- Replace \`.input.is-small\` with \`data-role=\"input\"\` and add \`.small\` class

**Keep Alpine.js component in common.js unchanged** - only update class references in template strings if needed.

### pages\\jobs.html(MODIFY)

References: 

- pages\\partials\\service-logs.html(MODIFY)

Migrate jobs page to Metro UI components. This is the most complex page with statistics, tables, modals, and extensive JavaScript.

**Update body and container (lines 9, 13):**
- Same changes as other pages

**Update heading section (lines 16-21):**
- Same grid wrapper

**Update Job Statistics panel (lines 24-75):**
- Replace \`.frame\` with \`.panel\`
- Replace tile structure in header with simpler flex layout
- Update button classes (\`.btn\` → \`.button\`, \`.btn--sm\` → \`.small\`, \`.btn-info\` → \`.info\`)
- Update statistics display (lines 48-73):
  - Remove custom flex classes and use Metro UI: \`.d-flex\`, \`.flex-justify-around\`, \`.flex-align-center\`, \`.flex-gap-4\`
  - Replace \`.text-md\` with \`.text-small\`
  - Replace \`.q-text-muted\` with \`.text-muted\`
  - Replace \`.title\` with \`.h1\` or keep as-is (Metro UI styles h1-h6)
  - Replace \`.q-text-warning\`, \`.q-text-info\`, \`.q-text-success\`, \`.q-text-danger\` with \`.fg-orange\`, \`.fg-blue\`, \`.fg-green\`, \`.fg-red\`

**Update Default Jobs panel (lines 78-118):**
- Same panel structure changes
- Update table: add \`class=\"table striped hovered\"\`
- Replace \`.q-text-center\` with \`.text-center\`

**Update Crawler Jobs panel (lines 121-218):**
- Same panel structure changes
- Update filter controls (lines 143-187):
  - Replace \`.q-d-flex\`, \`.q-gap-2\`, \`.q-mb-4\` with Metro UI equivalents
  - Replace \`.q-w-25\` with \`.cell-3\` (Metro UI grid cell)
  - Replace \`.field\` with \`.form-group\`
  - Replace \`.select.is-fullwidth\` with \`data-role=\"select\"\` on select element
- Update table: add \`class=\"table striped hovered\"\`
- Update pagination (lines 212-216):
  - Replace custom nav with Metro UI pagination:
\`\`\`html
<div class=\"pagination\">
    <ul>
        <li class=\"prev\"><button id=\"prev-page\" class=\"button small\" onclick=\"previousPage()\">Previous</button></li>
        <li><span id=\"page-info\" class=\"text-muted\">Page 1 of 1</span></li>
        <li class=\"next\"><button id=\"next-page\" class=\"button small\" onclick=\"nextPage()\">Next</button></li>
    </ul>
</div>
\`\`\`

**Update Job Detail panel (lines 221-240):**
- Same panel structure changes
- Update tag: replace \`.tag.tag--info\` with \`.badge.info\`

**Update Create Job Modal (lines 251-310):**
- Replace with Metro UI dialog structure (similar to sources.html modal)
- Update form fields with \`data-role\` attributes
- Update button classes

**JavaScript updates (lines 312-1056):**
- Update class references in \`renderJobs\` function (lines 418-478):
  - Replace \`.q-bg-light\` with \`.bg-light\`
  - Replace \`.q-text-center\` with \`.text-center\`
  - Replace \`.btn\` with \`.button\`, \`.btn--sm\` with \`.small\`
  - Replace \`.btn-warning\` with \`.warning\`, \`.btn-danger\` with \`.danger\`
- Update \`getStatusBadge\` function (lines 481-490):
  - Replace \`.tag.tag--warning\` with \`.badge.warning\`
  - Replace \`.tag.tag--info\` with \`.badge.info\`
  - Replace \`.tag.tag--success\` with \`.badge.success\`
  - Replace \`.tag.tag--danger\` with \`.badge.danger\`
- Update \`renderDefaultJobs\` function (lines 896-948):
  - Same badge class updates
  - Replace \`.q-text-info\` with \`.fg-blue\`
- Keep all logic, API calls, and event handlers unchanged

### pages\\documents.html(MODIFY)

References: 

- pages\\partials\\service-logs.html(MODIFY)

Migrate documents page to Metro UI components. Similar structure to jobs page.

**Update body and container (lines 9, 13):**
- Same changes as other pages

**Update heading section (lines 16-21):**
- Same grid wrapper

**Update Document Statistics panel (lines 24-54):**
- Replace \`.frame\` with \`.panel\`
- Update statistics display (lines 35-52):
  - Replace \`.q-d-flex\`, \`.q-justify-between\`, \`.q-flex-wrap\`, \`.q-gap-3\` with Metro UI equivalents
  - Replace \`.q-text-center\` with \`.text-center\`
  - Replace \`.text-md\` with \`.text-small\`
  - Replace \`.q-text-muted\` with \`.text-muted\`

**Update Documents Table panel (lines 57-127):**
- Same panel structure changes
- Update filter controls (lines 63-97):
  - Replace \`.q-d-flex\`, \`.q-gap-2\`, \`.q-mb-4\`, \`.q-flex-wrap\` with Metro UI equivalents
  - Replace \`.q-w-50\`, \`.q-w-25\` with Metro UI grid cells (\`.cell-6\`, \`.cell-3\`)
  - Replace \`.field\` with \`.form-group\`
  - Add \`data-role=\"input\"\` to search input
  - Add \`data-role=\"select\"\` to select elements
- Update table: add \`class=\"table striped hovered\"\`
- Replace \`.q-text-center\` with \`.text-center\`
- Update pagination: same structure as jobs.html

**Update Document Detail panel (lines 130-141):**
- Same panel structure changes
- Update tag: replace \`.tag.tag--info\` with \`.badge.info\`

**JavaScript updates (lines 151-476):**
- Update class references in \`renderDocuments\` function (lines 263-328):
  - Replace \`.q-bg-light\` with \`.bg-light\`
  - Replace \`.q-text-center\` with \`.text-center\`
  - Replace \`.btn\` with \`.button\`, \`.btn--sm\` with \`.small\`
  - Replace \`.btn-danger\` with \`.danger\`
  - Replace \`.q-mr-1\` with \`.mr-1\`
  - Replace \`.tag.tag--success\` with \`.badge.success\`
  - Replace \`.tag.tag--danger\` with \`.badge.danger\`
- Keep all logic, API calls, and event handlers unchanged

### pages\\auth.html(MODIFY)

Migrate auth page to Metro UI components.

**Update body and container (lines 9, 13):**
- Same changes as other pages

**Update heading section (lines 16-21):**
- Same grid wrapper

**Update Instructions panel (lines 26-43):**
- Replace \`.frame\` with \`.panel\`
- Replace \`.frame__header\` with \`.panel-header\`, \`.frame__body\` with \`.panel-content\`
- Update info box (lines 38-41):
  - Replace \`.q-bg-info\`, \`.q-p-3\`, \`.q-rounded\`, \`.q-mt-4\` with Metro UI classes
  - Use: \`<div class=\"bg-cyan fg-white p-4 border-radius mt-4\">\`

**Update Authentication List panel (lines 46-114):**
- Same panel structure changes
- Update header button (lines 51-53):
  - Replace \`.btn.outline.btn--sm\` with \`.button.small.outline\`
- Update loading state (lines 58-62):
  - Replace \`.q-text-center\`, \`.q-p-5\` with \`.text-center\`, \`.p-5\`
  - Replace \`.q-mt-3\` with \`.mt-3\`
- Update empty state (lines 66-72):
  - Same class replacements
  - Replace \`.q-text-muted\` with \`.text-muted\`
- Update table: add \`class=\"table striped hovered\"\`
- Update tag in table (line 96):
  - Replace \`.tag.tag--primary\` with \`.badge.primary\`
- Update delete button (lines 102-106):
  - Replace \`.btn.btn--sm.btn-danger.outline\` with \`.button.small.danger.outline\`

**Update Associated Sources panel (lines 117-137):**
- Same panel structure changes
- Update button (lines 132-135):
  - Replace \`.btn.btn-info.btn--sm.outline\` with \`.button.info.small.outline\`

**Alpine.js component (lines 145-217) - no changes needed** - only class references in template strings are updated above.

### pages\\static\\common.js(MODIFY)

References: 

- pages\\partials\\snackbar.html(MODIFY)

Update Alpine.js components to work with Metro UI class names. This file contains component definitions that generate HTML dynamically.

**serviceLogs component (lines 6-151) - no changes needed** - the component generates log entries dynamically but doesn't use Cirrus-specific classes in the template strings.

**snackbar component (lines 154-189) - can be removed or simplified:**
- This component is no longer needed since we're using Metro UI Toast API
- However, keep it for backwards compatibility if any code directly uses Alpine's snackbar
- Update \`getClass\` method (lines 180-188) to return Metro UI classes:
\`\`\`javascript
getClass() {
    const typeMap = {
        'success': 'bg-green fg-white',
        'error': 'bg-red fg-white',
        'warning': 'bg-yellow fg-dark',
        'info': 'bg-blue fg-white'
    };
    return typeMap[this.type] || 'bg-blue fg-white';
}
\`\`\`

**appStatus component (lines 192-244):**
- Update \`getStatusColor\` method (lines 229-237) to return Metro UI badge classes:
\`\`\`javascript
getStatusColor(state) {
    const colorMap = {
        'Idle': 'info',
        'Crawling': 'warning',
        'Offline': 'danger',
        'Unknown': 'secondary'
    };
    return colorMap[state] || 'secondary';
}
\`\`\`
- Remove \`'is-'\` prefix from color names

**sourceManagement component (lines 247-379) - no changes needed** - this component uses API calls and Alpine state management, not class generation.

**Global notification function (lines 382-388):**
- Update to use Metro UI Toast instead of Alpine snackbar:
\`\`\`javascript
window.showNotification = function(message, type = 'info') {
    const typeMap = {
        'info': 'bg-blue fg-white',
        'success': 'bg-green fg-white',
        'warning': 'bg-yellow fg-dark',
        'error': 'bg-red fg-white',
        'danger': 'bg-red fg-white'
    };
    const classes = typeMap[type] || 'bg-blue fg-white';
    if (typeof Metro !== 'undefined' && Metro.toast) {
        Metro.toast.create(message, null, 3000, classes);
    } else {
        console.warn('Metro UI not loaded, falling back to console');
        console.log(\`[\${type.toUpperCase()}] \${message}\`);
    }
};
\`\`\`

### pages\\settings.html(MODIFY)

Migrate settings page to Metro UI components (if this file exists and is used).

**Apply same migration patterns as other pages:**
- Update body and container
- Update heading section with grid wrapper
- Replace \`.frame\` with \`.panel\`
- Replace \`.frame__header\` with \`.panel-header\`, \`.frame__body\` with \`.panel-content\`
- Update table: add \`class=\"table striped hovered\"\`
- Update buttons: replace \`.btn\` with \`.button\`, size and color modifiers
- Replace custom utility classes with Metro UI equivalents
- Update form fields with \`data-role\` attributes

**Note:** This file was shown in grep results but not in the initial file list. Verify if it's actively used before migrating.