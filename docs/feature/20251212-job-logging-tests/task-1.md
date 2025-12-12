# Task 1: Create job_logging_improvements_test.go with UI tests
Workdir: ./docs/feature/20251212-job-logging-tests/ | Depends: none | Critical: no
Model: opus | Skill: go

## Context
This task is part of: Creating UI tests for job logging improvements
Prior tasks completed: none - this is first

## User Intent Addressed
Create UI tests that verify:
1. "Filter logs..." placeholder text
2. Log level filter dropdown with All/Warn+/Error options
3. Log level badges [INF]/[DBG]/[WRN]/[ERR] in tree log lines
4. terminal-* CSS classes for colored log levels

## Input State
Files that exist before this task:
- `test/ui/job_framework_test.go` - UITestContext and helper methods
- `test/ui/logs_test.go` - Example log test patterns
- `pages/queue.html` - UI with logging features (already implemented)

## Output State
Files after this task completes:
- `test/ui/job_logging_improvements_test.go` - New test file with logging improvement tests

## Skill Patterns to Apply
### From go/SKILL.md:
- **DO:** Use error wrapping with context (%w)
- **DO:** Use structured logging via arbor
- **DO:** Follow existing test patterns in test/ui/
- **DON'T:** Panic on errors - return (result, error)
- **DON'T:** Use global state

## Implementation Steps
1. Create test/ui/job_logging_improvements_test.go
2. Import required packages (testing, time, chromedp, test/common)
3. Create TestJobLoggingImprovements function using UITestContext
4. Create helper function to create a local_dir job definition via API
5. Trigger job and wait for logs to appear
6. Add subtests:
   - VerifyFilterLogsPlaceholder - check placeholder text
   - VerifyLogLevelDropdown - check dropdown options
   - VerifyLogLevelBadges - check [INF]/[DBG]/etc. display
   - VerifyLogLevelColors - check terminal-* CSS classes

## Code Specifications
Function signature:
```go
func TestJobLoggingImprovements(t *testing.T)
```

Key JavaScript evaluations for verification:
- Placeholder: `document.querySelector('input[placeholder="Filter logs..."]')`
- Dropdown: Check for `.dropdown .menu-item` with All/Warn+/Error text
- Badges: Check for elements with `.terminal-info`, `.terminal-warning`, etc.
- Level text: Check for `[INF]`, `[DBG]`, `[WRN]`, `[ERR]` in log lines

## Accept Criteria
- [ ] Test file compiles without errors
- [ ] Test uses UITestContext from job_framework_test.go
- [ ] Test creates a job that generates logs
- [ ] Test verifies "Filter logs..." placeholder
- [ ] Test verifies log level dropdown exists
- [ ] Test verifies log level badges appear
- [ ] Test verifies terminal-* CSS classes
- [ ] Build passes

## Handoff
After completion, next task(s): Execute test and iterate
