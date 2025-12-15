# Feature: Job Logging UI Tests
- Slug: job-logging-tests | Type: feature | Date: 2025-12-12
- Request: "Add a test for all items listed in job-logging-improvements summary (./test/ui). Use test/ui/job_framework_test.go as a template. Implement a basic job configuration in the test, which simple creates logs, and tests the UI, not job function. Execute the test. Iterate to complete/pass."
- Prior: ./docs/feature/20251212-job-logging-improvements/

## User Intent
Create UI tests for the three job logging improvements implemented in the prior work:

1. **"Show earlier logs" button** - Test that clicking loads more logs (default 200)
2. **Log level filter** - Test the dropdown with All/Warn+/Error filtering and "Filter logs..." placeholder
3. **Colored log levels** - Test that [INF]/[DBG]/[WRN]/[ERR] are displayed with correct colors

The test should:
- Use the UITestContext framework from job_framework_test.go
- Create a simple job that generates logs at different levels (INF, DBG, WRN, ERR)
- Test the UI features, not job functionality
- Be located in test/ui/

## Success Criteria
- [ ] Test file created in test/ui/ using UITestContext framework
- [ ] Test creates a job that generates logs at multiple levels
- [ ] Test verifies "Filter logs..." placeholder text
- [ ] Test verifies log level filter dropdown exists with All/Warn+/Error options
- [ ] Test verifies log level badges [INF]/[DBG]/[WRN]/[ERR] appear in tree log lines
- [ ] Test verifies terminal-* CSS classes are applied for colored log levels
- [ ] Test passes when executed

## Skills Assessment
| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | yes | yes | Writing Go test file |
| frontend | .claude/skills/frontend/SKILL.md | yes | no | Not modifying frontend, only testing |

**Active Skills:** go
