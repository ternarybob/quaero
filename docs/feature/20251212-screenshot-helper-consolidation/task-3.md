# Task 3: Verify build and run tests
Workdir: ./docs/feature/20251212-screenshot-helper-consolidation/ | Depends: 2 | Critical: no
Model: opus | Skill: go

## Context
This task is part of: Screenshot helper consolidation - verification step
Prior tasks completed: Task 1 - Enhanced screenshot_helper.go, Task 2 - Updated setup.go

## User Intent Addressed
Verify all existing tests continue to work after consolidation.

## Input State
Files that exist before this task:
- `test/ui/screenshot_helper.go` - Enhanced with full page capture
- `test/common/setup.go` - Delegates to screenshot_helper

## Output State
Files after this task completes:
- No file changes, verification only

## Skill Patterns to Apply
### From go/SKILL.md:
- **DO:** Run build to verify compilation
- **DO:** Run tests to verify functionality

## Implementation Steps
1. Run `powershell -ExecutionPolicy Bypass -File scripts/build.ps1` to verify build
2. Run `go test -v ./test/ui -run TestJobLoggingImprovements` to verify a screenshot-heavy test
3. Check screenshots are being captured correctly

## Accept Criteria
- [ ] Build passes
- [ ] Test passes
- [ ] Screenshots are captured (verified by test output)

## Handoff
After completion, next task(s): Validation phase
