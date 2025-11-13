# Plan: Settings Page Partial File Routing Fix

## Steps
1. **Add New Route Handler for Settings Partials**
   - Skill: @go-coder
   - Files: `internal\server\routes.go`
   - User decision: no

2. **Implement ServePartial Method for PageHandler**
   - Skill: @go-coder
   - Files: `internal\handlers\page_handler.go`
   - User decision: no

## Success Criteria
- AJAX requests to `/settings/{partial}.html` correctly serve HTML fragments from `pages/partials/settings-{partial}.html`
- Proper security validation using allowlist approach
- Content-Type headers set correctly for HTML fragments
- Comprehensive error handling and logging
- All existing functionality preserved
