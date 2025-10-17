I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The project currently uses **Metro UI v5** CSS framework with JavaScript-enhanced components. The migration to **Spectre CSS** requires replacing not only CSS classes but also Metro UI's JavaScript functionality for toasts, pagination, modals, and form enhancements. The template \`docs/style_5.html\` provides visual reference. Critical components requiring custom JavaScript implementation: toast notifications (Metro.toast.create), pagination (Metro.getPlugin), modal dialogs (data-role=\"dialog\"), and form controls (data-role=\"input/select\"). The project has existing UI tests in \`cmd/quaero-test-runner\` that should be used to verify the migration.

### Approach

**Execution Order:** Phase 1 (Foundation) → Phase 2 (Shared Components) → Phase 3 (Page Migration) → Phase 4 (Testing & Validation). Replace Metro UI CDN links with Spectre CSS, update custom styles with GitHub-like theming and interactive component styles (toast, pagination, modal, forms), implement custom JavaScript replacements for Metro UI's interactive components in shared partials and common.js, systematically convert component classes across all 8 HTML pages, and validate with existing UI test suite. Include Git checkpoint for rollback safety.

### Reasoning

Listed repository structure, read all 16 relevant files including the Spectre template reference, analyzed Metro UI component usage patterns and JavaScript dependencies, identified critical interactive components requiring custom implementations (toasts, pagination, modals, forms), examined Alpine.js integration points in \`common.js\`, and reviewed existing test infrastructure in \`cmd/quaero-test-runner\` for validation strategy.

## Mermaid Diagram

sequenceDiagram
    participant Dev as Developer
    participant Git as Git Repository
    participant Head as head.html
    participant CSS as quaero.css
    participant Toast as Toast System
    participant Pagination as Pagination
    participant Modal as Modal System
    participant Forms as Form Controls
    participant Tests as UI Tests
    
    Note over Dev,Tests: Pre-Migration: Git Checkpoint
    Dev->>Git: git checkout -b refactor-spectre-css
    Dev->>Git: git commit -m \"Checkpoint before migration\"
    Dev->>Tests: Run baseline tests
    
    Note over Dev,Tests: Phase 1: Foundation (Execute 1-2)
    Dev->>Head: Replace Metro UI CDN with Spectre CSS
    Dev->>CSS: Rewrite with Spectre styles + custom components
    CSS->>Toast: Add .toast-container, .toast-item styles
    CSS->>Pagination: Add .pagination-container, .pagination-list styles
    CSS->>Modal: Add modal enhancements
    CSS->>Forms: Add .form-group, .form-input styles
    
    Note over Dev,Tests: Phase 2: Shared Components (Execute 3-8)
    Dev->>Toast: Implement custom notification system
    Toast->>Toast: Create DOM elements dynamically
    Toast->>Toast: Add slide-in/out animations
    Toast->>Toast: Auto-dismiss after 3000ms
    
    Dev->>Pagination: Implement custom pagination
    Pagination->>Pagination: Generate page number buttons
    Pagination->>Pagination: Handle page navigation
    Pagination->>Pagination: Update visible page range
    
    Dev->>Modal: Convert to Spectre modal structure
    Modal->>Modal: Add .modal-overlay for backdrop
    Modal->>Modal: Integrate with Alpine.js :class binding
    Modal->>Modal: Handle ESC key and overlay click
    
    Dev->>Forms: Replace data-role with standard HTML
    Forms->>Forms: Apply Spectre form classes
    Forms->>Forms: Update checkbox/select structure
    Forms->>Forms: Maintain Alpine.js x-model bindings
    
    Note over Dev,Tests: Phase 3: Page Migration (Execute 9-16)
    Dev->>Dev: Update index.html
    Dev->>Dev: Update auth.html
    Dev->>Dev: Update config.html
    Dev->>Dev: Update sources.html (with modal/forms)
    Dev->>Dev: Update jobs.html (with pagination)
    Dev->>Dev: Update documents.html (with pagination)
    Dev->>Dev: Update chat.html
    Dev->>Dev: Update settings.html
    
    Note over Dev,Tests: Phase 4: Testing & Validation (Execute 17)
    Dev->>Tests: Test toast notifications
    Dev->>Tests: Test pagination navigation
    Dev->>Tests: Test modal open/close/ESC
    Dev->>Tests: Test form submission
    Dev->>Tests: Test WebSocket updates
    Dev->>Tests: Run UI test suite
    Tests-->>Dev: All tests pass ✓
    
    Dev->>Git: git checkout main
    Dev->>Git: git merge refactor-spectre-css
    Dev->>Git: git tag v1.0.0-spectre

## Proposed File Changes

### README.md(MODIFY)

References: 

- cmd\\quaero-test-runner\\main.go

**Add Migration Checkpoint and Testing Instructions:**

**Before Starting Migration:**
- Add section documenting the Spectre CSS migration
- Include Git checkpoint instructions:
  - \`git checkout -b refactor-spectre-css\`
  - \`git commit -m \"Checkpoint before Spectre CSS migration\"\`
- Document rollback procedure: \`git checkout main\` if issues arise

**Testing Instructions:**
- Add reference to UI test suite: \`cd cmd/quaero-test-runner && go run .\`
- Document manual testing checklist for interactive components
- Note that all existing functionality should remain unchanged

### pages\\partials\\head.html(MODIFY)

References: 

- docs\\style_5.html

**PHASE 1: Foundation - Replace Metro UI CDN links with Spectre CSS**

**Execute First - This file must be updated before any other changes**

- Remove lines 7-9 (Metro UI CSS and icons CDN links)
- Remove lines 28-29 (Metro UI JavaScript CDN link)
- Add Spectre CSS CDN links after line 6:
  - \`https://unpkg.com/spectre.css/dist/spectre.min.css\`
  - \`https://unpkg.com/spectre.css/dist/spectre-exp.min.css\`
  - \`https://unpkg.com/spectre.css/dist/spectre-icons.min.css\`

**Preserve existing dependencies:**
- Keep Font Awesome (line 14-15)
- Keep Highlight.js (lines 17-22)
- Keep custom scripts (lines 24-26)
- Keep Alpine.js (line 32)

**Update comments:**
- Change line 11 comment from \"Metro UI color theming\" to \"Spectre CSS custom theming\"

### pages\\static\\quaero.css(MODIFY)

References: 

- docs\\style_5.html

**PHASE 1: Foundation - Replace entire file with Spectre-compatible custom styles**

**Execute Second - After head.html is updated**

**1. Update CSS Variables (lines 7-15):**
- Keep existing brand color variables but add additional variables from template:
  - \`--header-bg: #24292f\`, \`--page-bg: #f6f8fa\`, \`--content-bg: #ffffff\`
  - \`--text-primary: #1f2328\`, \`--text-secondary: #57606a\`
  - \`--btn-primary: #57606a\`, \`--border-color: #d0d7de\`, \`--border-radius: .375rem\`
  - \`--font-sans: -apple-system, BlinkMacSystemFont, \"Segoe UI\", \"Noto Sans\", Helvetica, Arial, sans-serif\`

**2. Replace Metro UI Button Styles with Spectre Button Styles:**
- Remove \`.button.primary\`, \`.button.success\`, \`.button.danger\` classes (lines 20-46)
- Add Spectre \`.btn\` overrides with custom brand colors:
  - \`.btn { border-radius: var(--border-radius); color: var(--btn-primary); border-color: var(--btn-primary); }\`
  - \`.btn-primary { background-color: var(--color-primary); border-color: var(--color-primary); color: white; }\`
  - \`.btn-success { background-color: var(--color-success); border-color: var(--color-success); color: white; }\`
  - \`.btn-error { background-color: var(--color-danger); border-color: var(--color-danger); color: white; }\`
- Add \`.btn-header-primary\` for navbar buttons (lines 109-120 from template)

**3. Replace Metro UI Badge/Tag Styles with Spectre Label Styles:**
- Remove \`.tag.primary\`, \`.badge.primary\` classes (lines 48-77)
- Add Spectre \`.label\` overrides with brand colors:
  - \`.label-success { background-color: var(--color-success); color: white; }\`
  - \`.label-danger, .label-error { background-color: var(--color-danger); color: white; }\`
  - \`.label-warning { background-color: var(--color-warning); color: #333; }\`
  - \`.label-info, .label-primary { background-color: var(--color-info); color: white; }\`

**4. Add GitHub-like Layout Styles from Template:**
- Add \`body\` styling with \`--page-bg\` and \`padding-top: 64px\` for fixed header (lines 38-44 from template)
- Add \`.app-header\` and \`.app-header-content\` styles (lines 51-120 from template)
- Add \`.app-header-nav\`, \`.nav-links\`, and link hover states
- Add \`.page-container\` (max-width: 1280px, margin: 1.5rem auto, padding: 0 1.5rem)
- Add \`.page-title\` styles (lines 135-142 from template)
- Add \`.content-section\`, \`.section-header\`, \`.section-body\` styles (lines 145-162 from template)

**5. Add Custom Component Styles:**
- Add \`.stats-grid\` (display: grid, grid-template-columns: repeat(4, 1fr), text-align: center)
- Add \`.stat-item\`, \`.stat-label\`, \`.stat-value\` styles (lines 170-198 from template)
- Add \`.terminal\` styles for service logs (lines 229-264 from template):
  - Background: #0d1117, border-radius: 6px, padding: 1rem
  - Font: 'SF Mono', Monaco, Consolas, monospace
  - Max-height: 400px, overflow-y: auto
- Add \`.terminal-line\`, \`.terminal-time\`, \`.terminal-info\`, \`.terminal-warning\`, \`.terminal-error\` color classes
- Add table styling overrides (lines 200-227 from template)
- Add card header enhancements (lines 165-167 from template)

**6. Add Toast Notification Styles:**
- Add \`.toast-container\` (position: fixed, top: 1rem, right: 1rem, z-index: 9999, max-width: 400px)
- Add \`.toast-item\` (background: white, border-radius: 6px, padding: 1rem, margin-bottom: 0.5rem, box-shadow: 0 4px 12px rgba(0,0,0,0.15), animation: slideIn 0.3s ease-out)
- Add color variants: \`.toast-success { background: #d4edda; border-left: 4px solid var(--color-success); }\`, \`.toast-error { background: #f8d7da; border-left: 4px solid var(--color-danger); }\`, \`.toast-warning { background: #fff3cd; border-left: 4px solid var(--color-warning); }\`, \`.toast-info { background: #d1ecf1; border-left: 4px solid var(--color-info); }\`
- Add animations: \`@keyframes slideIn { from { transform: translateX(400px); opacity: 0; } to { transform: translateX(0); opacity: 1; } }\`, \`@keyframes slideOut { from { opacity: 1; transform: translateX(0); } to { opacity: 0; transform: translateX(400px); } }\`
- Add \`.toast-removing { animation: slideOut 0.3s ease-out forwards; }\`

**7. Add Pagination Styles:**
- Add \`.pagination-container { display: flex; justify-content: center; margin-top: 1rem; }\`
- Add \`.pagination-list { display: flex; list-style: none; gap: 0.25rem; padding: 0; margin: 0; }\`
- Add \`.pagination-item button { padding: 0.5rem 0.75rem; border: 1px solid var(--border-color); border-radius: 6px; background: white; cursor: pointer; transition: all 0.2s; }\`
- Add \`.pagination-item button.active { background: var(--color-primary); color: white; border-color: var(--color-primary); }\`
- Add \`.pagination-item button:hover:not(.active):not(:disabled) { background: var(--page-bg); }\`
- Add \`.pagination-item button:disabled { opacity: 0.5; cursor: not-allowed; }\`

**8. Add Modal Enhancements:**
- Add \`.modal.active { display: flex; }\` (Spectre provides base modal styles)
- Add body scroll lock: \`.modal.active ~ body { overflow: hidden; }\` (if needed)

**9. Add Form Control Enhancements:**
- Add \`.form-group { margin-bottom: 1rem; }\`
- Add \`.form-label { display: block; font-weight: 500; margin-bottom: 0.5rem; color: var(--text-primary); }\`
- Add \`.form-input:focus, .form-select:focus { border-color: var(--color-primary); box-shadow: 0 0 0 2px rgba(7, 87, 186, 0.1); }\`
- Add \`.form-input-hint { font-size: 0.875rem; color: var(--text-secondary); margin-top: 0.25rem; }\`
- Add \`.text-error { color: var(--color-danger); }\`

**10. Remove Metro UI Specific Styles:**
- Remove \`.app-bar .brand\` styling (line 80-82)
- Remove \`[data-role=\"appbar\"]\` border styling (line 85-87)
- Remove \`.panel-header\` padding override (line 90-92)

**11. Add Utility Classes:**
- Add color utility classes: \`.text-success { color: var(--color-success); }\`, \`.text-warning { color: var(--color-warning); }\`, \`.text-danger, .text-error { color: var(--color-danger); }\`, \`.text-info { color: var(--color-info); }\`, \`.text-primary { color: var(--color-primary); }\`, \`.text-secondary, .text-muted { color: var(--text-secondary); }\`
- Add spacing utilities if not provided by Spectre: \`.mt-4 { margin-top: 1.5rem; }\`, \`.mb-4 { margin-bottom: 1.5rem; }\`, \`.p-4 { padding: 1.5rem; }\`

### pages\\partials\\snackbar.html(MODIFY)

References: 

- pages\\static\\common.js(MODIFY)

**PHASE 2: Shared Components - Replace Metro UI toast with custom notification system**

**Execute Third - After CSS foundation is in place**

**1. Remove Metro UI Toast Function (lines 2-18):**
- Remove entire \`showSnackbar\` function that uses \`Metro.toast.create\`

**2. Implement Custom Toast System:**

**Create Toast Container on Page Load:**
\`\`\`javascript
document.addEventListener('DOMContentLoaded', function() {
  // Create toast container if it doesn't exist
  if (!document.getElementById('toast-container')) {
    const toastContainer = document.createElement('div');
    toastContainer.className = 'toast-container';
    toastContainer.id = 'toast-container';
    document.body.appendChild(toastContainer);
  }
});
\`\`\`

**Implement showSnackbar Function:**
\`\`\`javascript
function showSnackbar(message, type = 'info') {
  // Type to class mapping
  const typeMap = {
    'info': 'toast-info',
    'success': 'toast-success',
    'warning': 'toast-warning',
    'error': 'toast-error',
    'danger': 'toast-error'
  };
  const toastClass = typeMap[type] || 'toast-info';
  
  // Get or create toast container
  let container = document.getElementById('toast-container');
  if (!container) {
    container = document.createElement('div');
    container.id = 'toast-container';
    container.className = 'toast-container';
    document.body.appendChild(container);
  }
  
  // Create toast element
  const toast = document.createElement('div');
  toast.className = 'toast-item ' + toastClass;
  
  // Add icon based on type
  const icons = {
    'success': 'fa-check-circle',
    'error': 'fa-exclamation-circle',
    'warning': 'fa-exclamation-triangle',
    'info': 'fa-info-circle'
  };
  const iconClass = icons[type] || icons['info'];
  
  toast.innerHTML = \`
    <i class=\"fas \${iconClass}\" style=\"margin-right: 0.5rem;\"></i>
    <span>\${message}</span>
  \`;
  
  // Append to container
  container.appendChild(toast);
  
  // Limit to 5 toasts
  const toasts = container.querySelectorAll('.toast-item');
  if (toasts.length > 5) {
    toasts[0].remove();
  }
  
  // Auto-dismiss after 3000ms
  setTimeout(() => {
    toast.classList.add('toast-removing');
    setTimeout(() => {
      if (toast.parentNode) {
        toast.remove();
      }
    }, 300); // Allow animation to complete
  }, 3000);
}
\`\`\`

**3. Preserve Backwards Compatibility:**
- Keep \`window.showNotification = showSnackbar\` alias
- Ensure function signature matches existing usage: \`showSnackbar(message, type = 'info')\`

**4. Error Handling:**
- Add try-catch to handle cases where toast container doesn't exist
- Fallback to console.log if DOM manipulation fails:
\`\`\`javascript
try {
  // toast creation code
} catch (error) {
  console.warn('Toast notification failed, falling back to console');
  console.log(\`[\${type.toUpperCase()}] \${message}\`);
}
\`\`\`

### pages\\static\\common.js(MODIFY)

References: 

- pages\\partials\\snackbar.html(MODIFY)

**PHASE 2: Shared Components - Update Alpine.js components and global notification function**

**Execute Fourth - After snackbar.html is updated**

**1. Update serviceLogs Component (lines 6-151):**
- Modify \`_getLevelClass\` function (lines 122-131):
  - Change Metro UI color classes to terminal classes:
    - \`'ERROR': 'terminal-error'\` (instead of 'fg-red')
    - \`'WARN': 'terminal-warning', 'WARNING': 'terminal-warning'\` (instead of 'fg-orange')
    - \`'INFO': 'terminal-info'\` (instead of 'fg-blue')
    - \`'DEBUG': 'terminal-time'\` (instead of 'fg-gray')
- Keep all other logic unchanged (WebSocket subscription, log management, auto-scroll)

**2. Update appStatus Component (lines 154-206):**
- Modify \`getStatusColor\` function (lines 191-199):
  - Change Metro UI badge color names to Spectre label color names:
    - \`'Idle': 'label-primary'\` (instead of 'info')
    - \`'Crawling': 'label-warning'\` (instead of 'warning')
    - \`'Offline': 'label-error'\` (instead of 'danger')
    - \`'Unknown': 'label'\` (instead of 'secondary')
- Keep all other logic unchanged (status fetching, WebSocket subscription)

**3. Update sourceManagement Component (lines 209-341):**
- Keep all component logic unchanged
- Ensure modal visibility works with Spectre modal structure:
  - \`showCreateModal\` and \`showEditModal\` flags control \`.active\` class on modal
- Verify form data binding works with standard HTML inputs (no data-role attributes)
- Keep all CRUD operations and API calls unchanged

**4. Replace Global Notification Function (lines 345-361):**

**Remove Metro.toast dependency and implement custom toast (matching snackbar.html):**

\`\`\`javascript
window.showNotification = function(message, type = 'info') {
  // Type to class mapping
  const typeMap = {
    'info': 'toast-info',
    'success': 'toast-success',
    'warning': 'toast-warning',
    'error': 'toast-error',
    'danger': 'toast-error'
  };
  const toastClass = typeMap[type] || 'toast-info';
  
  try {
    // Get or create toast container
    let container = document.getElementById('toast-container');
    if (!container) {
      container = document.createElement('div');
      container.id = 'toast-container';
      container.className = 'toast-container';
      document.body.appendChild(container);
    }
    
    // Create toast element
    const toast = document.createElement('div');
    toast.className = 'toast-item ' + toastClass;
    
    // Add icon based on type
    const icons = {
      'success': 'fa-check-circle',
      'error': 'fa-exclamation-circle',
      'warning': 'fa-exclamation-triangle',
      'info': 'fa-info-circle'
    };
    const iconClass = icons[type] || icons['info'];
    
    toast.innerHTML = \`
      <i class=\"fas \${iconClass}\" style=\"margin-right: 0.5rem;\"></i>
      <span>\${message}</span>
    \`;
    
    // Append to container
    container.appendChild(toast);
    
    // Limit to 5 toasts
    const toasts = container.querySelectorAll('.toast-item');
    if (toasts.length > 5) {
      toasts[0].remove();
    }
    
    // Auto-dismiss after 3000ms
    setTimeout(() => {
      toast.classList.add('toast-removing');
      setTimeout(() => {
        if (toast.parentNode) {
          toast.remove();
        }
      }, 300); // Allow animation to complete
    }, 3000);
  } catch (error) {
    // Fallback to console if DOM manipulation fails
    console.warn('Toast notification failed, falling back to console');
    console.log(\`[\${type.toUpperCase()}] \${message}\`);
  }
};
\`\`\`

**5. Keep Function Signature:**
- Maintain \`window.showNotification(message, type = 'info')\` signature
- Ensure backwards compatibility with all existing calls throughout codebase
- No changes needed to calling code

### pages\\partials\\navbar.html(MODIFY)

References: 

- docs\\style_5.html
- pages\\static\\common.js(MODIFY)

**PHASE 2: Shared Components - Convert Metro UI AppBar to Spectre-based custom header**

**Execute Fifth - After common.js is updated**

**1. Replace AppBar Structure (lines 1-19):**
- Remove \`<nav data-role=\"appbar\" data-expand-point=\"md\" class=\"pos-fixed\">\`
- Implement custom header structure from \`docs/style_5.html\` (lines 277-291):

\`\`\`html
<header class=\"app-header\">
  <div class=\"app-header-content\">
    <nav class=\"app-header-nav\">
      <a href=\"/\" class=\"brand\"><span class=\"text-bold\">Quaero</span></a>
      <div class=\"nav-links\">
        <a href=\"/\" {{if eq .Page \"home\"}}class=\"active\"{{end}}>HOME</a>
        <a href=\"/auth\" {{if eq .Page \"auth\"}}class=\"active\"{{end}}>AUTHENTICATION</a>
        <a href=\"/sources\" {{if eq .Page \"sources\"}}class=\"active\"{{end}}>SOURCES</a>
        <a href=\"/jobs\" {{if eq .Page \"jobs\"}}class=\"active\"{{end}}>JOBS</a>
        <a href=\"/documents\" {{if eq .Page \"documents\"}}class=\"active\"{{end}}>DOCUMENTS</a>
        <a href=\"/chat\" {{if eq .Page \"chat\"}}class=\"active\"{{end}}>CHAT</a>
        <a href=\"/config\" {{if or (eq .Page \"settings\") (eq .Page \"config\")}}class=\"active\"{{end}}>SETTINGS</a>
      </div>
    </nav>
    <span class=\"label label-success status-text\">ONLINE</span>
  </div>
</header>
\`\`\`

**2. Update Status Badge (line 17):**
- Change from \`<span class=\"badge success status-text\">\` to \`<span class=\"label label-success status-text\">\`
- Update JavaScript function \`updateNavbarStatus\` (lines 24-37):
  - Change class toggles from \`badge\` to \`label\`
  - Change \`success\` to \`label-success\`
  - Change \`danger\` to \`label-error\`

**Updated JavaScript:**
\`\`\`javascript
function updateNavbarStatus(online) {
  const statusText = document.querySelector('.status-text');
  if (statusText) {
    if (online) {
      statusText.classList.remove('label-error');
      statusText.classList.add('label-success');
      statusText.textContent = 'ONLINE';
    } else {
      statusText.classList.remove('label-success');
      statusText.classList.add('label-error');
      statusText.textContent = 'OFFLINE';
    }
  }
}
\`\`\`

**3. Update Body Padding Script (lines 47-64):**
- Modify \`updateBodyPadding\` function:
  - Change selector from \`document.querySelector('[data-role=\"appbar\"]')\` to \`document.querySelector('.app-header')\`
  - Keep height calculation and body padding logic unchanged
- Keep debounced resize handler logic

**4. Preserve WebSocket Integration:**
- Keep \`updateNavbarStatus\` function and WebSocketManager subscription (lines 23-46)
- Keep connection status monitoring

### pages\\partials\\footer.html(MODIFY)

References: 

- docs\\style_5.html

**PHASE 2: Shared Components - Update footer styling**

**Execute Sixth - After navbar.html is updated**

**1. Update Container Classes (line 1):**
- Change from \`<footer class=\"container-fluid text-center p-4 border-top\">\`
- To: \`<footer style=\"text-align: center; padding: 2rem; color: var(--text-secondary); font-size: 0.875rem;\">\`
- Or use template footer styling (lines 267-272 from \`docs/style_5.html\`)
- Remove Metro UI utility classes, use inline styles or custom classes

**2. Preserve Functionality:**
- Keep \`#footer-version\` div (line 2-4)
- Keep version loading script (lines 7-20) unchanged
- Ensure fetch API call to \`/api/version\` remains functional
- Keep innerHTML update logic for version display

### pages\\partials\\service-status.html(MODIFY)

References: 

- docs\\style_5.html

**PHASE 2: Shared Components - Convert Metro UI panel to Spectre card**

**Execute Seventh - After footer.html is updated**

**1. Update Panel Structure (lines 1-35):**
- Change \`<div class=\"panel\">\` to \`<div class=\"card\">\`
- Change \`<div class=\"panel-header\">\` to \`<div class=\"card-header\">\`
- Change \`<div class=\"panel-content\">\` to \`<div class=\"card-body\">\`

**2. Update Header Title (line 4):**
- Change \`<h6 class=\"m-0\">\` to \`<h2>\` (Spectre card-title pattern)
- Or use template's section-header pattern with navbar for consistency

**3. Update Button Classes (lines 10-25):**
- Change \`<a class=\"button primary small outline\">\` to \`<a class=\"btn btn-sm btn-primary\">\`
- Change \`<a class=\"button info small outline\">\` to \`<a class=\"btn btn-sm\">\`
- Change \`<a class=\"button success small outline\">\` to \`<a class=\"btn btn-sm btn-success\">\`
- Keep icon structure with Font Awesome classes unchanged
- Keep href attributes and link text unchanged

**4. Update Utility Classes:**
- Replace Metro UI flex utilities:
  - \`d-flex\` → keep or use Spectre equivalent
  - \`flex-gap-2\` → use custom spacing or margin classes
  - \`mb-4\` → use custom margin class
- Update text utilities:
  - \`text-small\` → use custom class or inline style
  - \`text-muted\` → use custom class with \`color: var(--text-secondary)\`

**5. Preserve Links and Content:**
- Keep all href attributes unchanged
- Keep \"Getting Started\" paragraph content
- Keep horizontal rule \`<hr>\`

### pages\\partials\\service-logs.html(MODIFY)

References: 

- docs\\style_5.html
- pages\\static\\common.js(MODIFY)

**PHASE 2: Shared Components - Convert Metro UI panel to Spectre card with template styling**

**Execute Eighth - After service-status.html is updated**

**1. Update Panel Structure (lines 1-25):**
- Change \`<div class=\"panel\" x-data=\"serviceLogs\">\` to \`<div class=\"card\" x-data=\"serviceLogs\">\`
- Change \`<div class=\"panel-header\">\` to \`<div class=\"card-header\">\`
- Change \`<div class=\"panel-content\">\` to \`<div class=\"card-body\">\`

**2. Update Header with Navbar Pattern (lines 3-20):**
- Implement template's card-header with navbar pattern (lines 302-318 from \`docs/style_5.html\`):

\`\`\`html
<div class=\"card-header\">
  <header class=\"navbar\">
    <section class=\"navbar-section\">
      <h2>Service Logs</h2>
    </section>
    <section class=\"navbar-section\">
      <div class=\"btn-group btn-group-block\">
        <button class=\"btn btn-sm\" @click=\"toggleAutoScroll()\" :title=\"autoScroll ? 'Pause Auto-Scroll' : 'Resume Auto-Scroll'\">
          <i :class=\"autoScroll ? 'fas fa-pause' : 'fas fa-play'\"></i>
        </button>
        <button class=\"btn btn-sm\" @click=\"refresh()\" title=\"Refresh Logs\">
          <i class=\"fa-solid fa-rotate-right\"></i>
        </button>
        <button class=\"btn btn-sm btn-error\" @click=\"clearLogs()\" title=\"Clear Logs\">
          <i class=\"fas fa-trash\"></i>
        </button>
      </div>
    </section>
  </header>
</div>
\`\`\`

**3. Update Button Classes (lines 7-17):**
- Change \`<button class=\"button small outline\">\` to \`<button class=\"btn btn-sm\">\`
- Change \`<button class=\"button small outline danger\">\` to \`<button class=\"btn btn-sm btn-error\">\`
- Keep Alpine.js directives (@click, :title, :class) unchanged
- Remove \`<span class=\"icon\">\` wrappers, use icons directly

**4. Update Log Container (line 23):**
- Replace Metro UI classes with terminal styling:
  - Change \`class=\"log-container bg-light p-4\"\` to \`class=\"terminal\"\`
  - Remove inline styles, use \`.terminal\` class from \`quaero.css\`
  - Keep \`x-ref=\"logContainer\"\` for Alpine.js reference
- Keep Alpine.js template logic (x-if, x-for) unchanged
- Update log entry structure to match terminal styling:

\`\`\`html
<div class=\"terminal-line\">
  <span class=\"terminal-time\" x-text=\"\`[\${log.timestamp}]\`\"></span>
  <span :class=\"log.levelClass\" x-text=\"\`[\${log.level}]\`\"></span>
  <span x-text=\"log.message\"></span>
</div>
\`\`\`

**5. Update Log Level Classes in common.js:**
- Already covered in \`common.js\` modification
- Ensure classes match: \`terminal-error\`, \`terminal-warning\`, \`terminal-info\`, \`terminal-time\`

### pages\\index.html(MODIFY)

References: 

- docs\\style_5.html

**PHASE 3: Page Migration - Convert Metro UI grid and components to Spectre**

**Execute Ninth - After all shared components are updated**

**1. Update Container and Grid (lines 13-34):**
- Change \`<div class=\"container-fluid mt-4\">\` to \`<main class=\"page-container\">\`
- Replace Metro UI grid system:
  - Remove \`<div class=\"row\">\` and \`<div class=\"cell-12 p-2\">\`
  - Use simple div structure with custom spacing

**2. Update Heading Section (lines 16-21):**
- Wrap in \`<div class=\"page-title\">\`:

\`\`\`html
<div class=\"page-title\">
  <h1>Quaero</h1>
  <p>Unified Data Collection and Analysis Platform</p>
</div>
\`\`\`

**3. Update Component Sections (lines 23-33):**
- Replace \`<div class=\"row mt-4\">\` with \`<section>\` or \`<div style=\"margin-top: 1.5rem;\">\`
- Remove \`<div class=\"cell-12\">\` wrappers
- Keep template includes unchanged:

\`\`\`html
<section>
  {{template \"service-status.html\" .}}
</section>

<section style=\"margin-top: 1.5rem;\">
  {{template \"service-logs.html\" .}}
</section>
\`\`\`

**4. Update Closing Tags:**
- Change closing \`</div>\` to \`</main>\` for page-container
- Keep footer and snackbar includes unchanged

### pages\\auth.html(MODIFY)

References: 

- docs\\style_5.html

**PHASE 3: Page Migration - Convert Metro UI components to Spectre throughout auth page**

**Execute Tenth - After index.html is updated**

**1. Update Container and Grid (lines 13-149):**
- Change \`<div class=\"container-fluid mt-4\">\` to \`<main class=\"page-container\">\`
- Replace Metro UI grid (\`row\`, \`cell-12\`) with simple div structure
- Wrap heading in \`<div class=\"page-title\">\` (lines 16-21)

**2. Update Panel Components (lines 26-47, 50-121, 124-148):**
- Change all \`<div class=\"panel\">\` to \`<div class=\"card\">\`
- Change all \`<div class=\"panel-header\">\` to \`<div class=\"card-header\">\`
- Change all \`<div class=\"panel-content\">\` to \`<div class=\"card-body\">\`
- Use template's section-header pattern with navbar for headers with buttons

**3. Update Button Classes:**
- Line 56: Change \`<button class=\"button outline small\">\` to \`<button class=\"btn btn-sm\">\`
- Line 107: Change \`<button class=\"button danger small outline\">\` to \`<button class=\"btn btn-sm btn-error\">\`
- Line 141: Change \`<button class=\"button info small outline\">\` to \`<button class=\"btn btn-sm\">\`
- Keep Alpine.js directives (@click, :disabled, :class) unchanged
- Remove \`<span class=\"icon\">\` wrappers, use icons directly

**4. Update Badge Classes (line 101):**
- Change \`<span class=\"badge primary\">\` to \`<span class=\"label label-primary\">\`
- Keep Alpine.js x-text binding unchanged

**5. Update Table (lines 81-116):**
- Keep \`<table class=\"table striped border\">\` structure
- Verify Spectre table classes compatibility
- Update any Metro UI specific table utilities
- Keep Alpine.js template logic (x-for, x-text) unchanged

**6. Update Alert/Info Box (lines 40-43):**
- Change \`<div class=\"bg-cyan p-3 bd-default mt-4\" style=\"border-radius: 4px;\">\`
- To: \`<div class=\"toast toast-primary\" style=\"margin-top: 1rem;\">\`
- Or create custom \`.info-box\` class in \`quaero.css\`
- Keep content unchanged

**7. Preserve Alpine.js Component:**
- Keep \`x-data=\"authPage()\"\` and all Alpine directives unchanged (lines 50-227)
- Keep \`authPage()\` function logic unchanged (lines 157-227)
- Ensure Alpine.js functions work with new class names

### pages\\config.html(MODIFY)

References: 

- docs\\style_5.html

**PHASE 3: Page Migration - Convert Metro UI components to Spectre in config page**

**Execute Eleventh - After auth.html is updated**

**1. Update Container and Layout (lines 13-67):**
- Change \`<div class=\"container-fluid mt-4\">\` to \`<main class=\"page-container\">\`
- Replace Metro UI grid with simple div structure
- Wrap heading in \`<div class=\"page-title\">\` (lines 16-21)

**2. Update Panel Components (lines 24-65):**
- Change \`<div class=\"panel\">\` to \`<div class=\"card\">\`
- Change \`<div class=\"panel-header\">\` to \`<div class=\"card-header\">\`
- Change \`<div class=\"panel-content\">\` to \`<div class=\"card-body\">\`
- Use navbar pattern in card-header for title and status badge alignment

**3. Update Badge Classes (line 32):**
- Change \`:class=\"'badge ' + (isOnline ? 'success' : 'danger')\"\` 
- To: \`:class=\"'label ' + (isOnline ? 'label-success' : 'label-error')\"\`
- Update Alpine.js binding to use Spectre label classes

**4. Update Code Display (line 54):**
- Keep \`<pre>\` and \`<code>\` structure
- Replace Metro UI classes:
  - Change \`class=\"bg-light p-4 border\"\` to \`class=\"terminal\"\` or custom code block class
  - Or use inline styles: \`style=\"background: #f6f8fa; padding: 1rem; border: 1px solid var(--border-color); border-radius: 6px;\"\`
- Keep Alpine.js x-text binding unchanged

**5. Preserve Alpine.js Component:**
- Keep \`x-data=\"configPage()\"\` and all Alpine directives unchanged (lines 24-114)
- Keep \`configPage()\` function logic unchanged (lines 74-114)
- Keep \`formatConfig()\` function unchanged

**6. Update Service Logs Include:**
- Keep template include unchanged (line 63) - will be updated by partial modification

### pages\\sources.html(MODIFY)

References: 

- docs\\style_5.html
- pages\\static\\common.js(MODIFY)

**PHASE 3: Page Migration - Convert Metro UI components to Spectre in sources page with detailed modal and form implementation**

**Execute Twelfth - After config.html is updated**

**1. Update Container and Layout (lines 13-215):**
- Change \`<div class=\"container-fluid mt-4\">\` to \`<main class=\"page-container\">\`
- Replace Metro UI grid with simple div structure
- Wrap heading in \`<div class=\"page-title\">\` (lines 16-21)

**2. Update Panel Components (lines 26-44, 51-125):**
- Change all \`<div class=\"panel\">\` to \`<div class=\"card\">\`
- Change all \`<div class=\"panel-header\">\` to \`<div class=\"card-header\">\`
- Change all \`<div class=\"panel-content\">\` to \`<div class=\"card-body\">\`
- Use navbar pattern in card-headers for aligned titles and buttons

**3. Update Button Classes:**
- Lines 58-64: Change \`<button class=\"button info small outline\">\` to \`<button class=\"btn btn-sm btn-primary\">\`
- Line 62: Change \`<button class=\"button small outline\">\` to \`<button class=\"btn btn-sm\">\`
- Lines 113-118: Change edit/delete buttons to \`<button class=\"btn btn-sm\">\` and \`<button class=\"btn btn-sm btn-error\">\`
- Keep Alpine.js directives (@click) unchanged

**4. Update Badge Classes (lines 94-97, 101-109):**
- Change \`<span class=\"badge info\">\` to \`<span class=\"label label-primary\">\`
- Change \`<span class=\"badge success\">\` to \`<span class=\"label label-success\">\`
- Change \`<span class=\"badge\">\` to \`<span class=\"label\">\`
- Change \`<span class=\"badge warning\">\` to \`<span class=\"label label-warning\">\`
- Keep Alpine.js :class bindings unchanged

**5. Update Table (lines 70-123):**
- Keep \`<table class=\"table striped border\">\` structure
- Verify Spectre table compatibility
- Keep Alpine.js template logic (x-if, x-for) unchanged

**6. Convert Modal Dialog (lines 128-206) - DETAILED IMPLEMENTATION:**

**Remove Metro UI dialog and implement Spectre modal:**

\`\`\`html
<!-- Spectre Modal Structure -->
<div class=\"modal\" :class=\"{'active': showCreateModal || showEditModal}\">
  <a class=\"modal-overlay\" @click=\"closeModal()\" aria-label=\"Close\"></a>
  <div class=\"modal-container\" style=\"max-width: 600px;\">
    <div class=\"modal-header\">
      <a class=\"btn btn-clear float-right\" @click=\"closeModal()\" aria-label=\"Close\"></a>
      <div class=\"modal-title h5\" x-text=\"showEditModal ? 'Edit Source' : 'Add New Source'\"></div>
    </div>
    <div class=\"modal-body\">
      <div class=\"content\">
        <!-- Form fields here -->
      </div>
    </div>
    <div class=\"modal-footer\">
      <button class=\"btn btn-success\" @click=\"saveSource()\">Save</button>
      <button class=\"btn\" @click=\"closeModal()\">Cancel</button>
    </div>
  </div>
</div>
\`\`\`

**Add ESC key handler in JavaScript:**
\`\`\`javascript
// Add to page initialization
document.addEventListener('keydown', function(e) {
  if (e.key === 'Escape') {
    const modal = document.querySelector('.modal.active');
    if (modal) {
      // Trigger Alpine.js closeModal if available
      const closeBtn = modal.querySelector('[\\\\@click*=\"closeModal\"]');
      if (closeBtn) closeBtn.click();
    }
  }
});
\`\`\`

**7. Update Form Controls (lines 131-200) - DETAILED IMPLEMENTATION:**

**Replace Metro UI form controls with standard HTML + Spectre classes:**

\`\`\`html
<!-- Name Field -->
<div class=\"form-group\">
  <label class=\"form-label\">Name</label>
  <input class=\"form-input\" type=\"text\" x-model=\"currentSource.name\" placeholder=\"My Jira Instance\">
</div>

<!-- Type Field -->
<div class=\"form-group\">
  <label class=\"form-label\">Type</label>
  <select class=\"form-select\" x-model=\"currentSource.type\">
    <option value=\"jira\">Jira</option>
    <option value=\"confluence\">Confluence</option>
    <option value=\"github\">GitHub</option>
  </select>
</div>

<!-- Base URL Field -->
<div class=\"form-group\">
  <label class=\"form-label\">Base URL</label>
  <input class=\"form-input\" type=\"url\" x-model=\"currentSource.base_url\" placeholder=\"https://yourcompany.atlassian.net\">
</div>

<!-- Authentication Field -->
<div class=\"form-group\">
  <label class=\"form-label\">Authentication</label>
  <select class=\"form-select\" x-model=\"currentSource.auth_id\">
    <option value=\"\">No Authentication</option>
    <template x-for=\"auth in authentications\" :key=\"auth.id\">
      <option :value=\"auth.id\" x-text=\"\`\${auth.name} (\${auth.site_domain})\`\"></option>
    </template>
  </select>
  <p class=\"form-input-hint\">Select authentication credentials for this source</p>
  <template x-if=\"authentications.length === 0\">
    <p class=\"form-input-hint text-error\">
      No authentication configured. <a href=\"/auth\">Add authentication</a> first.
    </p>
  </template>
</div>

<!-- Enabled Checkbox -->
<div class=\"form-group\">
  <label class=\"form-checkbox\">
    <input type=\"checkbox\" x-model=\"currentSource.enabled\">
    <i class=\"form-icon\"></i> Enabled
  </label>
</div>

<!-- Crawl Configuration -->
<div class=\"form-group\">
  <label class=\"form-label\">Crawl Configuration</label>
  
  <label class=\"form-label\" style=\"font-size: 0.875rem; margin-top: 0.5rem;\">Max Depth</label>
  <input class=\"form-input\" type=\"number\" x-model.number=\"currentSource.crawl_config.max_depth\" min=\"1\" max=\"10\">
  
  <label class=\"form-label\" style=\"font-size: 0.875rem; margin-top: 0.5rem;\">Concurrency</label>
  <input class=\"form-input\" type=\"number\" x-model.number=\"currentSource.crawl_config.concurrency\" min=\"1\" max=\"10\">
  
  <label class=\"form-label\" style=\"font-size: 0.875rem; margin-top: 0.5rem;\">Detail Level</label>
  <select class=\"form-select\" x-model=\"currentSource.crawl_config.detail_level\">
    <option value=\"minimal\">Minimal</option>
    <option value=\"basic\">Basic</option>
    <option value=\"full\">Full</option>
  </select>
  
  <label class=\"form-checkbox\" style=\"margin-top: 0.5rem;\">
    <input type=\"checkbox\" x-model=\"currentSource.crawl_config.follow_links\">
    <i class=\"form-icon\"></i> Follow Links
  </label>
</div>
\`\`\`

**8. Update Modal Buttons (lines 202-205):**
- Already covered in modal footer above
- Change \`<button class=\"button success\">\` to \`<button class=\"btn btn-success\">\`
- Change \`<button class=\"button\">\` to \`<button class=\"btn\">\`

**9. Preserve Alpine.js Components:**
- Keep \`x-data=\"appStatus\"\` (line 26) and \`x-data=\"sourceManagement\"\` (line 48) unchanged
- Keep all Alpine directives and component logic in \`common.js\`
- Ensure modal visibility works with \`:class=\"{'active': showCreateModal || showEditModal}\"\`

### pages\\jobs.html(MODIFY)

References: 

- docs\\style_5.html

**PHASE 3: Page Migration - Convert Metro UI components to Spectre in jobs page with detailed pagination implementation**

**Execute Thirteenth - After sources.html is updated**

**1. Update Container and Layout:**
- Change \`<div class=\"container-fluid mt-4\">\` to \`<main class=\"page-container\">\`
- Replace Metro UI grid with simple div structure
- Wrap heading in \`<div class=\"page-title\">\`

**2. Update Panel Components:**
- Change all \`<div class=\"panel\">\` to \`<div class=\"card\">\`
- Change all \`<div class=\"panel-header\">\` to \`<div class=\"card-header\">\`
- Change all \`<div class=\"panel-content\">\` to \`<div class=\"card-body\">\`
- Apply navbar pattern in card-headers for statistics panel and job tables

**3. Update Button Classes Throughout:**
- Change \`<button class=\"button small outline\">\` to \`<button class=\"btn btn-sm\">\`
- Change \`<button class=\"button primary small outline\">\` to \`<button class=\"btn btn-sm btn-primary\">\`
- Change \`<button class=\"button success small outline\">\` to \`<button class=\"btn btn-sm btn-success\">\`
- Change \`<button class=\"button danger small outline\">\` to \`<button class=\"btn btn-sm btn-error\">\`
- Update all button classes in both HTML and JavaScript-generated content

**4. Update Badge Classes:**
- Change \`<span class=\"badge success\">\` to \`<span class=\"label label-success\">\`
- Change \`<span class=\"badge warning\">\` to \`<span class=\"label label-warning\">\`
- Change \`<span class=\"badge danger\">\` to \`<span class=\"label label-error\">\`
- Change \`<span class=\"badge info\">\` to \`<span class=\"label label-primary\">\`
- Update badge generation in JavaScript functions (search for 'badge' in script section)

**5. Update Tables:**
- Keep \`<table class=\"table striped border\">\` structure
- Verify Spectre table compatibility

**6. Convert Modal Dialog (Create Job Modal):**
- Remove \`data-role=\"dialog\"\` attribute
- Implement Spectre modal structure (same as sources.html modal)
- Use \`.modal\`, \`.modal-overlay\`, \`.modal-container\`, \`.modal-header\`, \`.modal-body\`, \`.modal-footer\`
- Add close button in modal-header
- Add overlay click handler: \`@click=\"closeModal()\"\`
- Add ESC key handler in JavaScript

**7. Update Form Controls in Modal:**
- Replace \`data-role=\"select\"\` with \`<select class=\"form-select\">\`
- Replace \`data-role=\"input\"\` with \`<input class=\"form-input\">\`
- Update checkbox structure to Spectre pattern:

\`\`\`html
<label class=\"form-checkbox\">
  <input type=\"checkbox\" x-model=\"refreshExisting\">
  <i class=\"form-icon\"></i> Refresh existing documents
</label>
\`\`\`

**8. Update Pagination - DETAILED IMPLEMENTATION:**

**Remove Metro UI pagination and implement custom pagination:**

**HTML Structure:**
\`\`\`html
<div class=\"pagination-container\">
  <ul class=\"pagination-list\">
    <li class=\"pagination-item\">
      <button class=\"btn btn-sm\" onclick=\"goToPage(currentPage - 1)\" id=\"prev-page\">
        <i class=\"fas fa-chevron-left\"></i>
      </button>
    </li>
    <li class=\"pagination-item\" id=\"pagination-numbers\">
      <!-- Page numbers inserted here by JavaScript -->
    </li>
    <li class=\"pagination-item\">
      <button class=\"btn btn-sm\" onclick=\"goToPage(currentPage + 1)\" id=\"next-page\">
        <i class=\"fas fa-chevron-right\"></i>
      </button>
    </li>
  </ul>
</div>
\`\`\`

**JavaScript Pagination Logic:**
\`\`\`javascript
// Add to page state
let currentPage = 1;
let totalPages = 1;
const pageSize = 100;

// Calculate visible page numbers (show 7 pages max)
function getVisiblePages() {
  const pages = [];
  const maxVisible = 7;
  let start = Math.max(1, currentPage - Math.floor(maxVisible / 2));
  let end = Math.min(totalPages, start + maxVisible - 1);
  
  // Adjust start if we're near the end
  if (end - start < maxVisible - 1) {
    start = Math.max(1, end - maxVisible + 1);
  }
  
  for (let i = start; i <= end; i++) {
    pages.push(i);
  }
  
  return pages;
}

// Render pagination controls
function renderPagination(totalPages) {
  const paginationNumbers = document.getElementById('pagination-numbers');
  const prevBtn = document.getElementById('prev-page');
  const nextBtn = document.getElementById('next-page');
  
  if (!paginationNumbers || !prevBtn || !nextBtn) return;
  
  // Update prev/next button states
  prevBtn.disabled = currentPage === 1;
  nextBtn.disabled = currentPage === totalPages;
  
  // Generate page number buttons
  const visiblePages = getVisiblePages();
  let html = '';
  for (const page of visiblePages) {
    const activeClass = page === currentPage ? 'active' : '';
    html += \`
      <button class=\"btn btn-sm \${activeClass}\" onclick=\"goToPage(\${page})\">
        \${page}
      </button>
    \`;
  }
  
  paginationNumbers.innerHTML = html;
}

// Go to specific page
function goToPage(page) {
  if (page < 1 || page > totalPages || page === currentPage) return;
  currentPage = page;
  renderJobs(); // Re-render with new page
}

// Update pagination display in renderJobs function
function renderJobs() {
  // ... existing rendering logic ...
  
  // Calculate pagination
  totalPages = Math.max(1, Math.ceil(filteredJobs.length / pageSize));
  if (currentPage > totalPages) currentPage = totalPages;
  
  // Render pagination
  renderPagination(totalPages);
  
  // ... rest of rendering logic ...
}
\`\`\`

**9. Update JavaScript-Generated HTML:**
- Search for all string templates that generate HTML with Metro UI classes
- Update badge classes in job status rendering:
  - \`badge success\` → \`label label-success\`
  - \`badge warning\` → \`label label-warning\`
  - \`badge danger\` → \`label label-error\`
- Update button classes in action buttons:
  - \`button small outline\` → \`btn btn-sm\`
  - \`button danger small outline\` → \`btn btn-sm btn-error\`

**10. Update Filter Controls:**
- Replace Metro UI select dropdowns:
  - Remove \`data-role=\"select\"\`
  - Add \`class=\"form-select\"\`
- Update filter section styling with Spectre form classes

**11. Preserve JavaScript Logic:**
- Keep all fetch API calls unchanged
- Keep all state management and data processing logic
- Keep all event handlers and function logic
- Only update class names in DOM manipulation

### pages\\documents.html(MODIFY)

References: 

- docs\\style_5.html

**PHASE 3: Page Migration - Convert Metro UI components to Spectre in documents page with detailed pagination implementation**

**Execute Fourteenth - After jobs.html is updated**

**1. Update Container and Layout (lines 13-145):**
- Change \`<div class=\"container-fluid mt-4\">\` to \`<main class=\"page-container\">\`
- Replace Metro UI grid with simple div structure
- Wrap heading in \`<div class=\"page-title\">\` (lines 16-21)

**2. Update Panel Components (lines 26-56, 62-119, 125-136):**
- Change all \`<div class=\"panel\">\` to \`<div class=\"card\">\`
- Change all \`<div class=\"panel-header\">\` to \`<div class=\"card-header\">\`
- Change all \`<div class=\"panel-content\">\` to \`<div class=\"card-body\">\`
- Use navbar pattern in card-headers for aligned titles and buttons

**3. Update Button Classes:**
- Line 30: Change \`<button class=\"button small outline\">\` to \`<button class=\"btn btn-sm\">\`
- Lines 332-337 (JavaScript): Update button classes in table row generation:
  - \`button small outline\` → \`btn btn-sm\`
  - \`button danger small outline\` → \`btn btn-sm btn-error\`

**4. Update Badge Classes:**
- Line 129: Change \`<span class=\"badge info\">\` to \`<span class=\"label label-primary\">\`
- Lines 318-320 (JavaScript): Update badge generation for vectorized status:
  - \`<span class=\"badge success\">\` → \`<span class=\"label label-success\">\`
  - \`<span class=\"badge danger\">\` → \`<span class=\"label label-error\">\`

**5. Update Form Controls (lines 68-85):**
- Replace \`data-role=\"input\"\` with standard \`<input class=\"form-input\">\`
- Replace \`data-role=\"select\"\` with standard \`<select class=\"form-select\">\`
- Keep onchange handlers unchanged

**6. Update Tables (lines 89-107):**
- Keep \`<table class=\"table striped border\">\` structure
- Verify Spectre table compatibility

**7. Update Pagination (lines 110-116) - DETAILED IMPLEMENTATION:**

**Remove Metro UI pagination and implement custom pagination:**

**HTML Structure:**
\`\`\`html
<div class=\"pagination-container\">
  <ul class=\"pagination-list\">
    <li class=\"pagination-item\">
      <button class=\"btn btn-sm\" onclick=\"goToPage(currentPage - 1)\" id=\"prev-page\">
        <i class=\"fas fa-chevron-left\"></i>
      </button>
    </li>
    <li class=\"pagination-item\" id=\"pagination-numbers\">
      <!-- Page numbers inserted here by JavaScript -->
    </li>
    <li class=\"pagination-item\">
      <button class=\"btn btn-sm\" onclick=\"goToPage(currentPage + 1)\" id=\"next-page\">
        <i class=\"fas fa-chevron-right\"></i>
      </button>
    </li>
  </ul>
</div>
\`\`\`

**JavaScript Pagination Logic (update lines 294-348):**

\`\`\`javascript
// Update renderDocuments function to include pagination rendering
function renderDocuments() {
  const tbody = document.getElementById('documents-table-body');
  const countElement = document.getElementById('document-count');
  const filteredCount = filteredDocuments.length;
  const totalPages = Math.max(1, Math.ceil(filteredCount / pageSize));
  
  // Ensure currentPage is within valid range
  if (currentPage > totalPages) currentPage = totalPages;
  if (currentPage < 1) currentPage = 1;
  
  // Calculate pagination
  const startIndex = (currentPage - 1) * pageSize;
  const endIndex = Math.min(startIndex + pageSize, filteredCount);
  const pageDocuments = filteredDocuments.slice(startIndex, endIndex);
  
  // Update count display
  countElement.textContent = \`\${totalDocuments}\`;
  
  // Render pagination
  renderPagination(totalPages);
  
  // ... rest of table rendering logic ...
}

// New pagination rendering function
function renderPagination(totalPages) {
  const paginationNumbers = document.getElementById('pagination-numbers');
  const prevBtn = document.getElementById('prev-page');
  const nextBtn = document.getElementById('next-page');
  
  if (!paginationNumbers || !prevBtn || !nextBtn) return;
  
  // Update prev/next button states
  prevBtn.disabled = currentPage === 1;
  nextBtn.disabled = currentPage === totalPages;
  
  // Calculate visible page numbers (show 7 pages max)
  const maxVisible = 7;
  let start = Math.max(1, currentPage - Math.floor(maxVisible / 2));
  let end = Math.min(totalPages, start + maxVisible - 1);
  
  if (end - start < maxVisible - 1) {
    start = Math.max(1, end - maxVisible + 1);
  }
  
  // Generate page number buttons
  let html = '';
  for (let i = start; i <= end; i++) {
    const activeClass = i === currentPage ? 'active' : '';
    html += \`
      <button class=\"btn btn-sm \${activeClass}\" onclick=\"goToPage(\${i})\">
        \${i}
      </button>
    \`;
  }
  
  paginationNumbers.innerHTML = html;
}

// Update goToPage function
function goToPage(page) {
  const totalPages = Math.max(1, Math.ceil(filteredDocuments.length / pageSize));
  if (page < 1 || page > totalPages) return;
  currentPage = page;
  renderDocuments();
}

// Remove old onPaginationPageClick function (lines 345-348)
// Replace with goToPage function above
\`\`\`

**8. Update Code Display (line 133):**
- Keep \`<pre>\` and \`<code>\` structure for JSON display
- Ensure Highlight.js integration remains functional

**9. Update JavaScript-Generated HTML (lines 315-341):**
- Update table row generation with new class names
- Update badge classes for vectorized status
- Update button classes for action buttons
- Keep onclick handlers and event logic unchanged

**10. Update Statistics Display (lines 36-53):**
- Replace Metro UI flex utilities with custom classes or inline styles
- Update text utility classes (\`text-small\`, \`text-muted\`, \`text-bold\`, \`h3\`)

**11. Preserve JavaScript Logic:**
- Keep all fetch API calls unchanged (lines 161-238, 378-473)
- Keep all state management and filtering logic (lines 152-268)
- Keep all event handlers unchanged (lines 476-495)

### pages\\chat.html(MODIFY)

References: 

- docs\\style_5.html

**PHASE 3: Page Migration - Convert Metro UI components to Spectre in chat page**

**Execute Fifteenth - After documents.html is updated**

**1. Update Container and Layout (lines 13-82):**
- Change \`<div class=\"container-fluid mt-4\">\` to \`<main class=\"page-container\">\`
- Replace Metro UI grid with simple div structure
- Wrap heading in \`<div class=\"page-title\">\` (lines 16-21)

**2. Update Panel Component (lines 25-72):**
- Change \`<div class=\"panel\">\` to \`<div class=\"card\">\`
- Change \`<div class=\"panel-header\">\` to \`<div class=\"card-header\">\`
- Change \`<div class=\"panel-content\">\` to \`<div class=\"card-body\">\`

**3. Update Chat Container (line 31):**
- Keep custom terminal-style background (\`background: #1a1a1a\`)
- Apply \`.terminal\` class from \`quaero.css\` for consistency
- Change to: \`<div class=\"terminal\" style=\"height: 60vh; margin-bottom: 1rem;\">\`

**4. Update Form Controls:**
- Line 37: Replace \`data-role=\"textarea\"\` with standard \`<textarea class=\"form-input\" style=\"min-height: 5rem; resize: vertical;\">\`
- Line 44: Update checkbox structure to Spectre pattern:

\`\`\`html
<label class=\"form-checkbox\">
  <input type=\"checkbox\" id=\"rag-enabled\" checked>
  <i class=\"form-icon\"></i> Enable RAG (Document Retrieval) - Requires embedding support
</label>
\`\`\`

**5. Update Button Classes (lines 50-57):**
- Change \`<button class=\"button primary small outline\">\` to \`<button class=\"btn btn-sm btn-primary\">\`
- Change \`<button class=\"button danger small outline\">\` to \`<button class=\"btn btn-sm btn-error\">\`
- Keep Alpine.js id attributes and event handlers
- Remove \`<span class=\"icon\">\` wrappers, use icons directly

**6. Update Status Display (lines 62-69):**
- Replace Metro UI text utilities:
  - \`text-muted\` → custom class or inline style
  - \`text-small\` → custom class or inline style
- Keep status div structure for JavaScript updates

**7. Update JavaScript Message Styling (lines 100-157):**
- Keep custom message styling for chat bubbles
- Verify color classes work with new CSS
- Update any Metro UI color references in inline styles

**8. Update Technical Metadata Styling (lines 122-157):**
- Keep custom metadata display styling
- Update Metro UI color classes:
  - \`bg-dark\` → use inline style or custom class
- Verify color classes and borders work with new CSS

**9. Update Live Status Display (lines 369-412):**
- Update color utility classes in JavaScript-generated HTML:
  - Change \`fg-green\` to \`text-success\` or inline style \`color: var(--color-success)\`
  - Change \`fg-red\` to \`text-error\` or inline style \`color: var(--color-danger)\`
  - Change \`fg-orange\` to \`text-warning\` or inline style \`color: var(--color-warning)\`
- Update icon classes if needed
- Keep status logic and WebSocket integration unchanged

**10. Preserve JavaScript Logic:**
- Keep all chat functionality unchanged (lines 88-437)
- Keep WebSocket integration and health checks
- Keep message sending and display logic
- Keep conversation history management
- Only update class names in DOM manipulation

### pages\\settings.html(MODIFY)

References: 

- docs\\style_5.html

**PHASE 3: Page Migration - Convert Metro UI components to Spectre in settings page**

**Execute Sixteenth - After chat.html is updated**

**1. Update Container and Layout (lines 13-91):**
- Change \`<div class=\"container-fluid mt-4\">\` to \`<main class=\"page-container\">\`
- Replace Metro UI grid with simple div structure
- Wrap heading in \`<div class=\"page-title\">\` (lines 16-21)

**2. Update Panel Components (lines 26-55, 60-82):**
- Change all \`<div class=\"panel\">\` to \`<div class=\"card\">\`
- Change all \`<div class=\"panel-header\">\` to \`<div class=\"card-header\">\`
- Change all \`<div class=\"panel-content\">\` to \`<div class=\"card-body\">\`
- Use navbar pattern in card-header for configuration table header

**3. Update Button Classes:**
- Line 30: Change \`<button class=\"button small outline\">\` to \`<button class=\"btn btn-sm\">\`
- Line 74: Change \`<button class=\"button danger small outline\">\` to \`<button class=\"btn btn-sm btn-error\">\`
- Remove \`<span class=\"icon\">\` wrappers, use icons directly

**4. Update Table (lines 36-52):**
- Keep \`<table class=\"table striped border\">\` structure
- Verify Spectre table compatibility
- Update any Metro UI specific table utilities

**5. Update Danger Zone Styling (lines 58-82):**
- Keep danger zone concept with red header
- Update header color class:
  - Change \`<h6 class=\"m-0 fg-red\">\` to \`<h6 class=\"text-error\">\` or \`<h6 style=\"color: var(--color-danger);\">\`
- Replace Metro UI flex utilities:
  - \`d-flex flex-align-center flex-justify-between flex-gap-3\` → use custom classes or inline styles
- Update text styling:
  - \`text-bold\` → \`<p style=\"font-weight: 600;\">\`
- Keep warning text and button structure

**6. Update JavaScript Table Generation (lines 136-158):**
- Keep table row generation logic
- Update any Metro UI class references in generated HTML
- Verify cell styling classes work with Spectre
- Keep section and value cell structure

**7. Preserve JavaScript Logic:**
- Keep \`loadConfig()\` function unchanged (lines 99-159)
- Keep \`confirmRemoveEmbeddings()\` function unchanged (lines 162-187)
- Keep all fetch API calls and error handling
- Only update class names in DOM manipulation

### docs\\MIGRATION_TESTING.md(NEW)

References: 

- cmd\\quaero-test-runner\\main.go

**PHASE 4: Testing & Validation - Create comprehensive testing checklist**

**Execute Last - After all pages are migrated**

**Create new file documenting testing strategy:**

# Spectre CSS Migration Testing Checklist

## Pre-Migration
- [ ] Create Git checkpoint: \`git checkout -b refactor-spectre-css\`
- [ ] Commit current state: \`git commit -m \"Checkpoint before Spectre CSS migration\"\`
- [ ] Document current Metro UI version: v5
- [ ] Run baseline UI tests: \`cd cmd/quaero-test-runner && go run .\`
- [ ] Take screenshots of all pages for visual comparison

## Interactive Components Testing

### Toast Notifications
- [ ] Toast appears on success actions (e.g., save source, delete job)
- [ ] Toast appears on error actions (e.g., failed API call)
- [ ] Toast appears on warning actions
- [ ] Toast appears on info actions
- [ ] Toast auto-dismisses after 3000ms
- [ ] Multiple toasts stack correctly (max 5)
- [ ] Toast slide-in animation works
- [ ] Toast slide-out animation works
- [ ] Toast icons display correctly for each type
- [ ] Toast container doesn't interfere with page layout

### Pagination
- [ ] Pagination displays correct page numbers
- [ ] Pagination navigates to next page
- [ ] Pagination navigates to previous page
- [ ] Pagination navigates to specific page number
- [ ] Pagination disables prev button on first page
- [ ] Pagination disables next button on last page
- [ ] Pagination updates when filters change
- [ ] Pagination shows correct active page
- [ ] Pagination handles edge cases (1 page, 0 items)
- [ ] Pagination visible page range adjusts correctly (7 pages max)

### Modal Dialogs
- [ ] Modal opens when clicking \"Add Source\" button
- [ ] Modal opens when clicking \"Edit\" button
- [ ] Modal opens when clicking \"Create Job\" button
- [ ] Modal closes when clicking overlay
- [ ] Modal closes when clicking close button
- [ ] Modal closes when pressing ESC key
- [ ] Modal prevents body scroll when open
- [ ] Modal form fields are accessible
- [ ] Modal form validation works
- [ ] Modal save/submit actions work correctly

### Form Controls
- [ ] Text inputs accept input correctly
- [ ] Select dropdowns open and select options
- [ ] Textareas accept multi-line input
- [ ] Checkboxes toggle correctly
- [ ] Number inputs accept numeric values
- [ ] URL inputs validate URLs
- [ ] Form labels are associated with inputs
- [ ] Form hints display correctly
- [ ] Form error messages display correctly
- [ ] Alpine.js x-model bindings work with new inputs

## Page-Specific Testing

### Index Page (Home)
- [ ] Page loads without errors
- [ ] Page title displays correctly
- [ ] Quick Actions buttons work
- [ ] Service Logs display correctly
- [ ] Service Logs auto-scroll works
- [ ] Service Logs color coding works (INFO, WARN, ERROR)
- [ ] WebSocket connection status updates

### Authentication Page
- [ ] Page loads without errors
- [ ] Instructions card displays correctly
- [ ] Authentication table displays correctly
- [ ] Refresh button works
- [ ] Delete button shows confirmation
- [ ] Delete action works correctly
- [ ] Empty state displays when no auth
- [ ] Date formatting works correctly

### Sources Page
- [ ] Page loads without errors
- [ ] Application Status displays correctly
- [ ] Sources table displays correctly
- [ ] Add Source button opens modal
- [ ] Edit button opens modal with data
- [ ] Delete button shows confirmation
- [ ] Modal form saves correctly (create)
- [ ] Modal form saves correctly (edit)
- [ ] Badge colors display correctly (enabled/disabled)
- [ ] Authentication dropdown populates

### Jobs Page
- [ ] Page loads without errors
- [ ] Job Statistics display correctly
- [ ] Default Jobs table displays correctly
- [ ] Crawler Jobs table displays correctly
- [ ] Pagination works correctly
- [ ] Filters work correctly
- [ ] Create Job modal opens
- [ ] Job actions work (rerun, cancel, delete)
- [ ] Job detail displays correctly
- [ ] Auto-refresh works for running jobs
- [ ] Badge colors display correctly (status)

### Documents Page
- [ ] Page loads without errors
- [ ] Document Statistics display correctly
- [ ] Documents table displays correctly
- [ ] Pagination works correctly
- [ ] Search filter works
- [ ] Source filter works
- [ ] Vectorized filter works
- [ ] Document detail displays correctly
- [ ] Reprocess button works
- [ ] Clear embedding button works
- [ ] Syntax highlighting works for JSON

### Chat Page
- [ ] Page loads without errors
- [ ] Chat container displays correctly
- [ ] Message input accepts text
- [ ] Send button sends message
- [ ] Clear button clears chat
- [ ] RAG checkbox toggles
- [ ] Messages display correctly (user/assistant)
- [ ] Technical metadata displays
- [ ] Live status updates
- [ ] Thinking animation displays
- [ ] Error messages display correctly

### Config Page
- [ ] Page loads without errors
- [ ] Service Status displays correctly
- [ ] Configuration details display correctly
- [ ] JSON formatting works
- [ ] Service Logs display correctly

### Settings Page
- [ ] Page loads without errors
- [ ] Configuration table displays correctly
- [ ] Refresh button works
- [ ] Danger Zone displays correctly
- [ ] Clear All button shows confirmation
- [ ] Clear All action works correctly

## Cross-Browser Testing
- [ ] Chrome: All features work
- [ ] Firefox: All features work
- [ ] Edge: All features work
- [ ] Safari: All features work (if available)

## Responsive Testing
- [ ] Desktop (1920x1080): Layout correct
- [ ] Laptop (1366x768): Layout correct
- [ ] Tablet (768x1024): Layout correct
- [ ] Mobile (375x667): Layout correct

## WebSocket Integration
- [ ] Navbar status updates on connection change
- [ ] Service logs receive real-time updates
- [ ] App status updates in real-time
- [ ] Connection loss handled gracefully
- [ ] Reconnection works correctly

## Alpine.js Integration
- [ ] serviceLogs component works
- [ ] appStatus component works
- [ ] sourceManagement component works
- [ ] authPage component works
- [ ] configPage component works
- [ ] All x-data bindings work
- [ ] All x-model bindings work
- [ ] All @click handlers work

## Visual Consistency
- [ ] Colors match brand guidelines
- [ ] Typography is consistent
- [ ] Spacing is consistent
- [ ] Buttons have consistent styling
- [ ] Cards have consistent styling
- [ ] Tables have consistent styling
- [ ] Forms have consistent styling
- [ ] GitHub-like aesthetic maintained

## Performance
- [ ] Page load times acceptable
- [ ] No console errors
- [ ] No console warnings
- [ ] CSS file size reasonable
- [ ] No layout shifts on load
- [ ] Animations smooth (60fps)

## Automated Testing
- [ ] Run UI test suite: \`cd cmd/quaero-test-runner && go run .\`
- [ ] All existing tests pass
- [ ] No new test failures introduced
- [ ] Test coverage maintained

## Rollback Plan
- [ ] If critical issues found: \`git checkout main\`
- [ ] If minor issues found: Document and fix incrementally
- [ ] Keep Metro UI branch available for comparison

## Post-Migration
- [ ] Update documentation with new CSS framework
- [ ] Remove Metro UI references from README
- [ ] Update style guide if exists
- [ ] Merge to main: \`git checkout main && git merge refactor-spectre-css\`
- [ ] Tag release: \`git tag -a v1.0.0-spectre -m \"Migrated to Spectre CSS\"\`

## Known Issues / Notes
- Document any known issues or workarounds here
- Note any browser-specific quirks
- Note any features that behave differently but acceptably