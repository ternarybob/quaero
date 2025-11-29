# Complete: Document Markdown View

## Classification
- Type: feature
- Location: ./docs/feature/document-markdown-view/

Added a new Markdown tab to the Documents page that renders the `content_markdown` field as formatted HTML. Users can now switch between JSON (raw data) and Markdown (formatted) views when expanding a document row. The implementation uses marked.js for markdown parsing with GitHub Flavored Markdown support, and applies highlight.js syntax highlighting to code blocks within the rendered markdown.

## Stats
Tasks: 3 | Files: 3 | Duration: ~10 minutes
Models: Planning=opus, Workers=3×sonnet, Review=skipped (no critical triggers)

## Tasks
- Task 1: Added marked.js v12.0.1 CDN to head.html for markdown parsing
- Task 2: Added comprehensive markdown content styles to quaero.css (.markdown-content class)
- Task 3: Updated documents.html with Spectre CSS tabs (JSON/Markdown) and switchTab() function

## Files Modified
1. `pages/partials/head.html` - Added marked.js script
2. `pages/static/quaero.css` - Added section 16 with markdown styles
3. `pages/documents.html` - Added tabs UI and markdown rendering logic

## Review: SKIPPED
No critical triggers (security, auth, crypto, etc.) detected.

## Verify
- go build: PASS
- JavaScript syntax: PASS
- Manual testing: Required (start service and test Documents page)

## Usage
1. Navigate to Documents page
2. Click on a document row to expand
3. Two tabs appear: "JSON" (default) and "Markdown"
4. Click "Markdown" tab to see rendered content_markdown
5. Code blocks in markdown get syntax highlighting
