# Plan: Add 1rem right/left margins to container elements

## Problem Analysis

The user wants to add 1rem left and right margins to all `<main class="container">` elements across all pages. This is a CSS styling change that will add horizontal spacing to the main content area.

## Steps

1. **Add CSS rule for container margins**
   - Skill: @none
   - Files: `pages/static/quaero.css`
   - User decision: no
   - Add a CSS rule that applies 1rem left and right margins to the `.container` class

2. **Verify changes across all pages**
   - Skill: @none
   - Files: All 9 HTML template files (settings.html, chat.html, documents.html, job_add.html, queue.html, job.html, search.html, index.html, config.html)
   - User decision: no
   - Verify that the CSS changes apply correctly to all pages using the `.container` class

## Success Criteria

- `.container` class has 1rem left and right margins applied
- All 9 HTML pages that use `<main class="container">` display with the new margins
- CSS syntax is valid
- No visual regressions or layout breaks
- Margins are consistent across all pages
