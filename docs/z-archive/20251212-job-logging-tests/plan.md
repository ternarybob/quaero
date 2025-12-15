# Plan: Job Logging UI Tests
Type: feature | Workdir: ./docs/feature/20251212-job-logging-tests/ | Date: 2025-12-12

## Context
Project: Quaero
Related files:
- `test/ui/job_framework_test.go` - UITestContext and test framework
- `test/ui/logs_test.go` - Existing log tests
- `pages/queue.html` - UI implementation with new logging features

## User Intent (from manifest)
Create UI tests for the three job logging improvements:
1. "Show earlier logs" button - loads 200 logs
2. Log level filter dropdown with All/Warn+/Error options and "Filter logs..." placeholder
3. Colored log level badges [INF]/[DBG]/[WRN]/[ERR] with terminal-* CSS classes

Test should use UITestContext framework, create a simple job that generates logs, and test UI features.

## Success Criteria (from manifest)
- [ ] Test file created in test/ui/ using UITestContext framework
- [ ] Test creates a job that generates logs at multiple levels
- [ ] Test verifies "Filter logs..." placeholder text
- [ ] Test verifies log level filter dropdown exists with All/Warn+/Error options
- [ ] Test verifies log level badges [INF]/[DBG]/[WRN]/[ERR] appear in tree log lines
- [ ] Test verifies terminal-* CSS classes are applied for colored log levels
- [ ] Test passes when executed

## Active Skills
| Skill | Key Patterns to Apply |
|-------|----------------------|
| go | Error handling with context, table-driven tests, test infrastructure in test/ui/ |

## Technical Approach
Create a single test file `test/ui/job_logging_improvements_test.go` that:
1. Uses UITestContext from job_framework_test.go
2. Creates a job definition via API with a local_dir step that generates multiple log entries
3. Triggers the job and monitors until it generates logs
4. Navigates to Queue page and verifies:
   - Filter input has "Filter logs..." placeholder
   - Log level filter dropdown exists with All/Warn+/Error options
   - Tree log lines contain [INF]/[DBG]/[WRN]/[ERR] badges
   - Level badges have terminal-* CSS classes for colors

## Files to Change
| File | Action | Purpose |
|------|--------|---------|
| test/ui/job_logging_improvements_test.go | create | New UI test file for logging improvements |

## Tasks
| # | Desc | Depends | Critical | Model | Skill | Est. Files |
|---|------|---------|----------|-------|-------|------------|
| 1 | Create job_logging_improvements_test.go with UI tests | - | no | opus | go | 1 |

## Execution Order
[1]

## Risks/Decisions
- Job needs to generate logs quickly for test to verify UI features
- Using local_dir job type since it generates logs and doesn't require external dependencies
- Test may need to wait for job to generate enough logs before verification
