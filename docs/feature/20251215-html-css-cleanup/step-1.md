# Step 1: CSS Consolidation

Iteration: 1 | Status: complete

## Changes Made

| File | Action | Description |
|------|--------|-------------|
| `pages/static/quaero.css` | modified | Removed commented-out CSS (~20 lines), added styles moved from HTML pages (~60 lines) |
| `pages/documents.html` | modified | Removed inline `<style>` block (37 lines), styles moved to quaero.css |
| `pages/queue.html` | modified | Removed inline `<style>` block (9 lines), styles moved to quaero.css |
| `pages/job_add.html` | modified | Removed inline `<style>` block (56 lines), styles moved to quaero.css |

## Summary of Changes

### CSS Consolidation
1. **Removed commented-out CSS from quaero.css:**
   - Line 885-904: Commented CSS in `.log-level-filter` section
   - Lines 1859-1861: Commented properties in `.menu-sidebar`

2. **Moved inline styles to quaero.css:**
   - Document page styles (`.document-row`, `.detail-row`, `.detail-content`)
   - Queue page dropdown focus styles
   - Job add page editor styles (`.editor-container`, `#toml-editor`, `.validation-message`)

### Benefits
- All page-specific CSS now centralized in quaero.css
- Easier maintenance and consistency
- Better caching (CSS file loaded once, not embedded in each HTML page)
- Cleaner HTML templates

### Spectre CSS Integration
- Confirmed Spectre CSS provides `.btn-group` class - no custom override needed
- Button variants (`.btn-primary`, `.btn-success`, `.btn-error`) customized via CSS variables
- Label variants (`.label-success`, `.label-error`) customized via CSS variables

## Build & Test

Build: Pass
Tests: Not run (CSS-only changes don't require Go tests)

## Architecture Compliance (self-check)

- [x] QUEUE_UI.md icon standards - Icons unchanged, still using fa-clock, fa-spinner, etc.
- [x] QUEUE_UI.md state management - Alpine.js components unchanged
- [x] QUEUE_UI.md WebSocket events - No changes to event handling
- [x] CSS uses Spectre defaults where possible - Button/label styling uses Spectre classes with custom variables
