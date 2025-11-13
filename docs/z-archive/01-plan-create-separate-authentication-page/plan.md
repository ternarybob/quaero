# Plan: Create Separate Authentication Page (auth.html)

## Steps
1. **Create pages/auth.html by extracting authentication section**
   - Skill: @go-coder
   - Files: pages/auth.html (NEW), pages/jobs.html (read reference), pages/settings.html (read reference)
   - User decision: no

2. **Validate authentication page implementation**
   - Skill: @test-writer
   - Files: pages/auth.html
   - User decision: no

3. **Modify routes.go to serve new auth.html template**
   - Skill: @go-coder
   - Files: internal/server/routes.go
   - User decision: no

4. **Validate routes.go changes**
   - Skill: @test-writer
   - Files: internal/server/routes.go
   - User decision: no

5. **Run final compilation and testing**
   - Skill: @test-writer
   - Files: All modified files
   - User decision: no

6. **Final validation and summary**
   - Skill: @test-writer
   - Files: All files
   - User decision: no

## Success Criteria
- New auth.html page serves authentication management functionality independently
- Routes.go correctly routes /auth to serve auth.html instead of jobs.html
- All existing functionality preserved and working
- Code compiles without errors
- Authentication management page displays correctly

## Context
The current /auth route serves jobs.html with embedded authentication UI. This needs to be separated into a dedicated auth.html page for better code organization and user experience. The plan extracts the authentication section (lines 22-91) and Alpine.js component (lines 239-309) from jobs.html into a new standalone page.