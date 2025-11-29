# Task 3: Update documents.html with tabs UI
- Group: 2 | Mode: sequential | Model: sonnet
- Skill: @frontend-developer | Critical: no | Depends: 1, 2
- Sandbox: /tmp/3agents/task-3/ | Source: pages/ | Output: ./docs/feature/document-markdown-view/

## Files
- `pages/documents.html` - Update detail row with Spectre tabs

## Requirements
Modify the document detail expansion to include tabs:
1. Add Spectre CSS tabs with "JSON" and "Markdown" tab items
2. JSON tab shows existing JSON code block functionality
3. Markdown tab renders content_markdown using marked.js
4. Apply highlight.js to code blocks in rendered markdown
5. Handle case where content_markdown is empty/null
6. Tab state managed per-document (not global)

## Acceptance
- [ ] Two tabs visible when document expanded: "JSON" and "Markdown"
- [ ] JSON tab shows raw JSON (existing behavior)
- [ ] Markdown tab renders content_markdown as HTML
- [ ] Code blocks in markdown are syntax highlighted
- [ ] Empty content_markdown shows appropriate message
- [ ] Tab switching works correctly per document
- [ ] Compiles (N/A - HTML/JS only)
- [ ] Tests pass (manual verification)
