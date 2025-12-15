# Feature: HTML/CSS Cleanup and Standardization

Date: 2025-12-15
Request: "Comprehensively review the html pages. Focus on CSS and styling. Prefer Spectre CSS default styles, remove redundant CSS/HTML, extract shared JavaScript to libs."

## User Intent

Consolidate and simplify the frontend codebase by:
1. Replacing custom CSS with Spectre CSS default classes where possible
2. Removing redundant/duplicated CSS and HTML
3. Extracting shared JavaScript into reusable modules

## Success Criteria

- [ ] Inline styles replaced with Spectre CSS utility classes where applicable
- [ ] Element-specific CSS in quaero.css reduced (prefer default Spectre styles)
- [ ] Redundant CSS rules removed from quaero.css
- [ ] Shared JavaScript utilities extracted to pages/static/ libs
- [ ] HTML pages use consistent class patterns
- [ ] Build/tests pass

## Applicable Architecture Requirements

| Doc | Section | Requirement |
|-----|---------|-------------|
| QUEUE_UI.md | Icon Standards | Status icons MUST use fa-clock, fa-spinner fa-spin, fa-check-circle, fa-times-circle, fa-ban |
| QUEUE_UI.md | State Management | Alpine.js x-data="jobList" with jobTreeData, jobTreeExpandedSteps, jobLogs |
| QUEUE_UI.md | WebSocket Events | Must subscribe to job_update, job_status_change, refresh_logs, queue_stats |
| QUEUE_UI.md | Log Line Numbering | Log lines MUST start at 1 and increment sequentially |
| QUEUE_UI.md | Auto-Expand Behavior | ALL steps should auto-expand when they start running |

## Current State Analysis

### CSS Files
- `pages/static/quaero.css` - 2246 lines, extensive custom styling
- Spectre CSS loaded from CDN (spectre.min.css, spectre-exp.min.css, spectre-icons.min.css)

### JavaScript Files
- `pages/static/websocket-manager.js` - WebSocket singleton (good pattern, already shared)
- `pages/static/common.js` - Alpine.js components and utilities (good pattern)
- `pages/static/partial-loader.js` - Partial loading
- `pages/static/settings-components.js` - Settings-specific components
- `pages/queue.html` - Large inline script (~5000+ lines)

### HTML Pages (10 total)
- chat.html, search.html, index.html, config.html, settings.html
- job_add.html, documents.html, jobs.html, job.html, queue.html

### Key Observations

1. **quaero.css has significant redundancy with Spectre CSS:**
   - Custom `.btn` styling duplicates Spectre's btn classes
   - Custom `.label` styling duplicates Spectre's label/badge classes
   - Custom `.card` styling partially duplicates Spectre
   - Custom `.table` styling partially duplicates Spectre

2. **Inline styles in HTML pages:**
   - documents.html has inline `<style>` block (~40 lines)
   - Many elements use style="" attributes instead of classes

3. **Large inline scripts:**
   - queue.html has massive inline JavaScript that could be extracted
   - documents.html has ~700 lines of inline JavaScript

4. **Spectre CSS classes not being used:**
   - .text-gray, .text-small already exist in Spectre
   - .empty (empty states) exists in Spectre
   - .chip exists in Spectre (used inconsistently)

## Cleanup Priorities

### High Priority (Reduce CSS Size)
1. Remove duplicate button styles (use Spectre .btn-primary, .btn-success, etc.)
2. Remove duplicate label/badge styles (use Spectre .label)
3. Remove duplicate table styles (use Spectre .table)

### Medium Priority (Improve Maintainability)
1. Extract queue.html inline JavaScript to `pages/static/queue-components.js`
2. Replace inline styles with CSS classes
3. Consolidate terminal/log styling

### Low Priority (Polish)
1. Remove commented CSS
2. Consistent use of CSS custom properties
3. Mobile responsive consistency
