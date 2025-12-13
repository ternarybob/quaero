# Plan: Refactor Settings Page - Remove Legacy Auth Route

## Steps

1. **Replace /auth route with redirect handler**
   - Skill: @go-coder
   - Files: internal/server/routes.go
   - User decision: no

2. **Remove AUTH navigation link from navbar**
   - Skill: @go-coder
   - Files: pages/partials/navbar.html
   - User decision: no

3. **Delete legacy auth.html page**
   - Skill: @none
   - Files: pages/auth.html
   - User decision: no

4. **Update UI tests to validate settings accordion**
   - Skill: @test-writer
   - Files: test/ui/auth_test.go
   - User decision: no

5. **Verify compilation and run tests**
   - Skill: @test-writer
   - Files: test/ui/auth_test.go
   - User decision: no

## Success Criteria
- `/auth` route redirects to `/settings?a=auth-apikeys,auth-cookies`
- AUTH navigation link removed from navbar
- Legacy auth.html page deleted
- UI tests pass with settings accordion validation
- All files compile cleanly
- Backward compatibility maintained for bookmarks/links
