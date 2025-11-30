# Plan: Document Markdown View

## Classification
- Type: feature
- Workdir: ./docs/feature/document-markdown-view/

## Analysis

### Dependencies
- Spectre CSS tabs component (`tab`, `tab-item`, `active` classes)
- Highlight.js already included in head.html for code syntax highlighting
- Existing `documents.html` page with accordion-style document expansion

### Approach
1. Modify the document detail row to include Spectre tabs with two views:
   - "JSON" tab (existing functionality - raw JSON display)
   - "Markdown" tab (new - render content_markdown as HTML)
2. Use a lightweight markdown-to-HTML library (marked.js) loaded from CDN
3. Apply highlight.js to code blocks within rendered markdown
4. Add minimal CSS for markdown content styling in quaero.css

### Risks
- Breaking change: Users who rely on the current JSON-only view will see different UI
- Markdown parsing: Need to handle edge cases like empty content_markdown
- XSS: Must sanitize rendered HTML (use marked.js with sanitization)

## Groups

| Task | Desc | Depends | Critical | Complexity | Model |
|------|------|---------|----------|------------|-------|
| 1 | Add marked.js CDN to head.html for markdown parsing | none | no | low | sonnet |
| 2 | Add markdown content styles to quaero.css | none | no | low | sonnet |
| 3 | Update documents.html detail row with tabs UI | 1,2 | no | medium | sonnet |

## Order
Concurrent: [1, 2] → Sequential: [3] → Validate
