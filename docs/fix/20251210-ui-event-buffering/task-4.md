# Task 4: Build and verify no errors
Depends: 1,2,3 | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent
Ensure all changes compile and work together.

## Skill Patterns to Apply
- Use build scripts (not `go build` directly)

## Do
1. Run `.\scripts\build.ps1` to compile
2. Fix any compilation errors
3. Verify no runtime errors in logs

## Accept
- [ ] Build succeeds with no errors
- [ ] No new warnings introduced
