# Plan: Remove Authentication Section from Jobs.html and Add Auth Nav Link

## Steps
1. **Remove authentication section from jobs.html**
   - Skill: @go-coder
   - Files: pages/jobs.html (MODIFY), pages/auth.html (read reference)
   - User decision: no

2. **Validate jobs.html changes**
   - Skill: @test-writer
   - Files: pages/jobs.html
   - User decision: no

3. **Add AUTH navigation link to navbar**
   - Skill: @go-coder
   - Files: pages/partials/navbar.html (MODIFY)
   - User decision: no

4. **Validate navbar changes**
   - Skill: @test-writer
   - Files: pages/partials/navbar.html
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
- Authentication section completely removed from jobs.html
- Jobs page focuses solely on job definitions management
- AUTH navigation link added with proper active state logic
- Navigation active state properly separates AUTH from JOBS
- All existing functionality preserved
- Code compiles without errors

## Context
The authentication management section (lines 22-91) and Alpine.js component (lines 239-309) need to be removed from jobs.html since authentication now has its own dedicated auth.html page. A new AUTH navigation link should be added to provide direct access to the authentication management page, and the JOBS link active state should be updated to only highlight on the /jobs page.