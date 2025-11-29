# Task 1: Add marked.js CDN to head.html
- Group: 1 | Mode: concurrent | Model: sonnet
- Skill: @frontend-developer | Critical: no | Depends: none
- Sandbox: /tmp/3agents/task-1/ | Source: pages/partials/ | Output: ./docs/feature/document-markdown-view/

## Files
- `pages/partials/head.html` - Add marked.js script tag

## Requirements
Add marked.js library from CDN for markdown-to-HTML parsing:
- Use cdnjs CDN for consistency with other libraries
- Add after highlight.js scripts
- Enable sanitization by default for XSS prevention

## Acceptance
- [ ] marked.js loaded from CDN
- [ ] Compiles (N/A - HTML only)
- [ ] Page loads without errors
