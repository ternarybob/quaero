# Metro UI v5 → Spectre CSS Migration Testing Guide

## Migration Overview

**Branch:** `refactor-spectre-css`
**Date:** 2025-10-17
**Scope:** Complete UI framework migration from Metro UI v5 to Spectre CSS

## Phase 1: Foundation Changes

### ✅ Files Modified
- `pages/partials/head.html` - CDN links updated
- `pages/static/quaero.css` - Complete rewrite (520 lines)

### Testing Checklist

#### Visual Verification
- [ ] CSS loads without 404 errors (check browser console)
- [ ] GitHub-like theme colors applied correctly
- [ ] Custom CSS variables working (--color-primary, --color-success, etc.)
- [ ] Terminal styling renders correctly (black background, monospace font)
- [ ] Code blocks use `code-block` class styling
- [ ] Toast notifications styled correctly

## Phase 2: Shared Components

### ✅ Files Modified
- `pages/partials/snackbar.html` - Custom toast system
- `pages/static/common.js` - Alpine.js components updated
- `pages/partials/navbar.html` - Custom header
- `pages/partials/footer.html` - Inline styles
- `pages/partials/service-status.html` - Card layout
- `pages/partials/service-logs.html` - Card with terminal

### Testing Checklist

#### Toast Notifications
- [ ] Success notifications appear (green icon)
- [ ] Error notifications appear (red icon)
- [ ] Warning notifications appear (orange icon)
- [ ] Info notifications appear (blue icon)
- [ ] Toasts auto-dismiss after 3 seconds
- [ ] Slide-in animation works
- [ ] Multiple toasts stack correctly

#### Navbar
- [ ] Fixed header at top (64px height)
- [ ] Logo/brand text visible
- [ ] Navigation links visible and clickable
- [ ] Active page highlighted correctly
- [ ] Online/Offline status badge updates
- [ ] Navbar responsive on mobile (menu collapses)

#### Footer
- [ ] Version info loads and displays
- [ ] Centered alignment
- [ ] Separator border visible

#### Service Status Card
- [ ] Quick action buttons render
- [ ] Icons visible in buttons
- [ ] Links navigate correctly
- [ ] Getting Started text readable

#### Service Logs Card
- [ ] Terminal styling (dark background, monospace font)
- [ ] Log lines render with timestamps
- [ ] Color coding by log level (ERROR=red, WARN=orange, INFO=white)
- [ ] Auto-scroll toggle works
- [ ] Refresh button works
- [ ] Clear logs button works

## Phase 3: Page Migrations

### ✅ index.html
#### Checklist
- [ ] Page title renders
- [ ] Service status card displays
- [ ] Service logs card displays
- [ ] Sections have proper spacing (1.5rem margin-top)

### ✅ auth.html
#### Checklist
- [ ] Instructions card displays
- [ ] Table renders with striped rows
- [ ] Badge classes (label label-primary, label label-success)
- [ ] Delete button (btn btn-sm btn-error) works
- [ ] Refresh button works
- [ ] Alpine.js `authPage()` component functional
- [ ] Empty state message shows when no auth

### ✅ config.html
#### Checklist
- [ ] Service Status card shows online/offline badge
- [ ] Badge colors (label label-success, label-error)
- [ ] Configuration Details card shows TOML config
- [ ] Code block styling (code-block class)
- [ ] Service logs display
- [ ] Refresh button works

### ✅ sources.html
#### Checklist
- [ ] Application Status card displays
- [ ] Sources list table renders
- [ ] Add Source button (btn btn-sm btn-primary)
- [ ] Refresh button works
- [ ] Badge classes for source types (label label-primary)
- [ ] Edit/Delete buttons work (btn btn-sm, btn btn-sm btn-error)

#### Modal Testing
- [ ] Modal opens when "Add Source" clicked
- [ ] Modal overlay darkens background
- [ ] Modal close button (X) works
- [ ] ESC key closes modal
- [ ] Click outside modal closes it
- [ ] Form inputs styled correctly (form-group, form-label, form-input)
- [ ] Checkboxes styled correctly (form-checkbox with form-icon)
- [ ] Save button works (btn btn-primary)
- [ ] Cancel button works (btn)

### ✅ jobs.html
#### Checklist
- [ ] Job Statistics card displays
- [ ] Statistics use CSS variables for colors
- [ ] Default Jobs table renders
- [ ] Badge classes (label label-success, label-warning)
- [ ] Enable/Disable buttons work (btn btn-sm btn-success/btn-warning)
- [ ] Running indicator (spinner with primary color)
- [ ] Crawler Jobs table renders
- [ ] Filter dropdowns work (form-select)
- [ ] Action buttons work (btn btn-sm, btn btn-sm btn-error)

#### Modal Testing
- [ ] Create Job modal opens
- [ ] Form inputs styled correctly
- [ ] Checkbox styled correctly (form-checkbox)
- [ ] Save/Cancel buttons work

#### Pagination Testing
- [ ] Custom pagination renders
- [ ] Previous/Next buttons work
- [ ] Page numbers clickable
- [ ] Current page highlighted (active class)
- [ ] Ellipsis (...) shown for large page counts
- [ ] Pagination updates when filters change

### ✅ documents.html
#### Checklist
- [ ] Document Statistics card displays
- [ ] Filter controls render (search box, dropdowns)
- [ ] Search input styled (form-input)
- [ ] Dropdowns styled (form-select)
- [ ] Table renders with documents
- [ ] Vectorized badges (label label-success ✓, label label-error ✗)
- [ ] Reprocess button works (btn btn-sm)
- [ ] Clear Embedding button works (btn btn-sm btn-error)
- [ ] Document Detail card shows JSON
- [ ] Code block styling correct

#### Pagination Testing
- [ ] Same checks as jobs.html pagination

### ✅ chat.html
#### Checklist
- [ ] Chat container renders (dark background)
- [ ] Message input styled (form-input)
- [ ] RAG checkbox styled correctly (form-checkbox)
- [ ] Send button works (btn btn-sm btn-primary)
- [ ] Clear button works (btn btn-sm btn-error)
- [ ] Messages render correctly
- [ ] Technical metadata displays with green border
- [ ] Thinking animation shows during request
- [ ] Live status indicators use CSS variables
- [ ] Health check icons colored correctly

### ✅ settings.html
#### Checklist
- [ ] Configuration table renders
- [ ] Table striped rows visible
- [ ] Danger Zone heading red (color: var(--color-danger))
- [ ] Clear All button works (btn btn-sm btn-error)
- [ ] Confirmation dialog shows before deletion

## Phase 4: Cross-Component Testing

### Interactive Features
- [ ] All buttons have hover states
- [ ] All links have hover states
- [ ] Focus states visible for keyboard navigation
- [ ] Form inputs have focus styling
- [ ] Checkboxes toggle correctly
- [ ] Dropdowns open and select values
- [ ] Modals animate smoothly
- [ ] Toasts slide in from right

### Alpine.js Components
- [ ] `appStatus` component updates status
- [ ] `serviceLogs` component auto-scrolls
- [ ] `authPage()` component loads data
- [ ] `configPage()` component loads config
- [ ] `sourceManagement` component CRUD operations
- [ ] All Alpine.js `x-data`, `x-show`, `x-if`, `x-for` directives work

### WebSocket Integration
- [ ] Service logs receive real-time updates
- [ ] Status badges update in real-time
- [ ] No console errors for WebSocket

### Responsive Design
- [ ] Desktop (1920x1080): All layouts correct
- [ ] Laptop (1366x768): No horizontal scroll
- [ ] Tablet (768x1024): Cards stack correctly
- [ ] Mobile (375x667): Navigation collapses, cards full-width

### Browser Compatibility
- [ ] Chrome (latest): All features work
- [ ] Firefox (latest): All features work
- [ ] Edge (latest): All features work
- [ ] Safari (latest): All features work

## Phase 5: Performance Testing

### Load Times
- [ ] CSS loads in < 100ms
- [ ] JavaScript loads in < 200ms
- [ ] First Contentful Paint < 1s
- [ ] Time to Interactive < 2s

### Runtime Performance
- [ ] No console errors
- [ ] No console warnings
- [ ] Smooth scrolling in terminal/logs
- [ ] Smooth animations (toasts, modals)
- [ ] No memory leaks (check DevTools Memory)

## Rollback Procedure

If critical issues found:

1. **Switch to main branch:**
   ```bash
   git checkout main
   ```

2. **Restart service:**
   ```bash
   .\scripts\build.ps1 -Run
   ```

3. **Document issues:**
   - Create GitHub issue with:
     - Screenshot of problem
     - Browser console errors
     - Steps to reproduce
     - Expected vs actual behavior

4. **Return to branch for fixes:**
   ```bash
   git checkout refactor-spectre-css
   ```

## Automated Testing

Run the UI test suite:

```bash
# Recommended: Use test runner (handles everything automatically)
.\scripts\build.ps1
cd bin
.\quaero-test-runner.exe

# Alternative: Manual testing (requires service already running)
.\scripts\build.ps1 -Run  # Start service first
cd test
go test -v ./api         # API tests
go test -v ./ui          # UI tests
```

### Expected Test Results
- [ ] All API tests pass
- [ ] All UI tests pass
- [ ] Screenshots saved to `test/results/`
- [ ] No test failures
- [ ] No test panics

## Migration Completion Checklist

### Code Review
- [ ] All Metro UI CDN links removed
- [ ] All `data-role` attributes removed
- [ ] All Metro UI class names replaced
- [ ] All `badge` classes → `label` classes
- [ ] All `button` classes → `btn` classes
- [ ] All `panel` → `card` conversions complete
- [ ] All `fg-*` color classes → CSS variables
- [ ] All `d-flex`, `flex-*` → inline styles or Spectre classes

### Documentation
- [ ] MIGRATION_TESTING.md complete
- [ ] README.md updated (if needed)
- [ ] CHANGELOG.md entry added
- [ ] Code comments updated

### Git
- [ ] All changes committed
- [ ] Commit messages descriptive
- [ ] Branch up to date with main
- [ ] Ready for pull request

## Sign-Off

**Tester:** _________________________
**Date:** _________________________
**Status:** [ ] PASS [ ] FAIL
**Notes:**

---

## Appendix: Key Class Mappings

### Containers
- `container-fluid mt-4` → `page-container`
- `row` → `section`
- `cell-*` → removed (use native HTML)

### Components
- `panel` → `card`
- `panel-header` → `card-header`
- `panel-content` → `card-body`
- `badge` → `label`
- `badge success` → `label label-success`
- `badge danger` → `label label-error`
- `badge info` → `label label-primary`
- `badge warning` → `label label-warning`

### Buttons
- `button primary small outline` → `btn btn-sm btn-primary`
- `button danger small outline` → `btn btn-sm btn-error`
- `button success small outline` → `btn btn-sm btn-success`
- `button warning small outline` → `btn btn-sm btn-warning`
- `button small outline` → `btn btn-sm`

### Forms
- `data-role="input"` → `class="form-input"`
- `data-role="select"` → `class="form-select"`
- `data-role="textarea"` → `class="form-input"`
- Metro checkbox → `form-checkbox` with `form-icon`

### Colors
- `fg-red` → `style="color: var(--color-danger);"`
- `fg-green` → `style="color: var(--color-success);"`
- `fg-orange` → `style="color: var(--color-warning);"`
- `fg-cyan` → `style="color: var(--color-primary);"`
- `bg-light` → `bg-gray`
- `bg-dark` → `style="background: var(--code-bg);"`

### Layouts
- `d-flex` → `style="display: flex;"`
- `flex-align-center` → `style="align-items: center;"`
- `flex-justify-between` → `style="justify-content: space-between;"`
- `flex-gap-2` → `style="gap: 0.5rem;"`
- `mt-4` → `style="margin-top: 1.5rem;"`

---

**End of Migration Testing Guide**
