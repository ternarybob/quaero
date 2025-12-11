# Task 7: Build and verify all changes compile
Depends: 1-6 | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent
All optimizations - ensure complete implementation compiles and is ready for testing

## Skill Patterns to Apply
- Always use build scripts (scripts/build.ps1)
- No binaries in repo root

## Do
1. Run go build ./... to verify compilation
2. Fix any compilation errors
3. Run scripts/build.ps1 to do full build with versioning

## Accept
- [ ] go build ./... succeeds
- [ ] scripts/build.ps1 succeeds
- [ ] No compilation errors
