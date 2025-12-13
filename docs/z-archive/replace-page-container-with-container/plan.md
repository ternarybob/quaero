# Plan: Replace page-container with standard container

## Problem Analysis
The codebase currently uses a custom `.page-container` class with custom CSS styling. The user wants to replace this with the standard, non-customized Spectre CSS `.container` class. This will simplify the codebase and rely on Spectre's built-in grid system.

## Current State
- `.page-container` is used in 9 HTML files: settings.html, queue.html, chat.html, documents.html, job_add.html, job.html, search.html, index.html, config.html
- `.page-container` has custom CSS starting at line 1702 in quaero.css
- The CSS includes nested rules for `.columns`, `.nav`, and `.nav-item`

## Desired State
- All HTML files use standard `.container` class instead of `.page-container`
- Remove all custom `.page-container` CSS rules
- Rely on Spectre CSS's built-in `.container` behavior

## Steps

1. **Replace page-container with container in all HTML files**
   - Skill: @none
   - Files: `pages/*.html` (9 files: settings.html, queue.html, chat.html, documents.html, job_add.html, job.html, search.html, index.html, config.html)
   - User decision: no
   - Find and replace all instances of `class="page-container"` with `class="container"`

2. **Remove page-container CSS rules**
   - Skill: @none
   - Files: `pages/static/quaero.css`
   - User decision: no
   - Remove the `.page-container` CSS block (approximately lines 1702-1745)
   - Remove or comment out "Page Container" comment if no longer needed

3. **Update page-title CSS to not depend on page-container**
   - Skill: @none
   - Files: `pages/static/quaero.css`
   - User decision: no
   - Ensure `.page-title` CSS (lines 323-336) doesn't rely on `.page-container` context
   - Add container padding if needed for proper spacing

## Success Criteria
- All HTML files use `.container` instead of `.page-container`
- All custom `.page-container` CSS removed from quaero.css
- `.page-title` and other elements maintain proper spacing
- All pages render correctly with standard Spectre container
- No console errors
- Responsive behavior preserved
