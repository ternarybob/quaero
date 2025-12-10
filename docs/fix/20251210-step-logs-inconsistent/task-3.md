# Task 3: Build and test fix manually
Depends: 2 | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent
Verify the fix works by building and running a test job.

## Skill Patterns to Apply
- Use build scripts (build.ps1)

## Do
1. Run `.\scripts\build.ps1` to compile
2. Verify no build errors
3. (Manual test by user) Run a job and verify step events appear in UI

## Accept
- [ ] Build succeeds without errors
- [ ] Application starts correctly
