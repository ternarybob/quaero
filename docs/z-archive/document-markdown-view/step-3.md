# Step 3: Update documents.html with tabs UI
- Task: task-3.md | Group: 2 | Model: sonnet

## Actions
1. Updated detail row HTML to include Spectre CSS tabs structure
2. Added "JSON" and "Markdown" tab items using `ul.tab > li.tab-item` pattern
3. Created separate content divs for JSON and Markdown views
4. Updated toggleDetail() function to load both JSON and markdown content
5. Added switchTab() function for tab switching per document
6. Configured marked.js with GFM and line breaks enabled
7. Applied hljs.highlightElement() to code blocks in rendered markdown
8. Added empty state handling for documents without content_markdown

## Files
- `pages/documents.html` - Updated detail row template and added JavaScript functions

## Decisions
- Used Spectre CSS native tab classes (no custom CSS needed)
- Tab state is per-document (each expanded document has independent tabs)
- JSON tab is default (active) for backward compatibility
- Markdown parsing uses marked.js with GFM mode for GitHub-style markdown
- Code blocks in markdown get syntax highlighting via highlight.js

## Verify
Compile: N/A (HTML/JS only) | Tests: Manual verification required

## Status: COMPLETE
