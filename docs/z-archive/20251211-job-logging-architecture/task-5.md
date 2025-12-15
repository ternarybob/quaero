# Task 5: Verify Build and Test
Depends: 4 | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent
Ensure the refactored architecture works correctly end-to-end

## Skill Patterns to Apply
From go/SKILL.md:
- Always use build scripts (scripts/build.ps1)
- Tests in test/api and test/ui directories

## Do
- Run build script to verify compilation
- Run existing UI tests for queue functionality
- Manual verification that tree expansion works as expected
- Document any issues found

## Accept
- [ ] Build completes without errors
- [ ] Existing tests pass
- [ ] Tree expansion works with backend-driven state
- [ ] No regressions in job monitoring functionality
