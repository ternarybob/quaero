# Plan: Refactor Settings Accordion to Use Spectre CSS Native Patterns

## Steps

1. **Update settings.html to use Spectre accordion patterns**
   - Skill: @go-coder
   - Files: pages/settings.html
   - User decision: no

2. **Simplify CSS to minimal icon rotation only**
   - Skill: @go-coder
   - Files: pages/static/quaero.css
   - User decision: no

3. **Verify compilation and visual testing**
   - Skill: @test-writer
   - Files: pages/settings.html, pages/static/quaero.css
   - User decision: no

## Success Criteria
- Settings accordion uses Spectre's `icon icon-arrow-right` for all headers
- HTML structure matches Spectre pattern: `<input type="checkbox" hidden>` (no extra classes)
- Font Awesome loading spinner replaced with Spectre's `<div class="loading loading-lg"></div>`
- Custom CSS reduced to only icon rotation (2 rules)
- All existing functionality preserved (URL state, dynamic loading, multiple expansions)
- Visual appearance clean and consistent with Spectre defaults
