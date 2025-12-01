# Step 1: Sort Steps by Dependencies
Model: sonnet | Status: ✅

## Done
- Added `sortStepsByDependencies()` function using Kahn's algorithm (topological sort)
- Steps with no `depends` come first (in-degree 0)
- Steps with dependencies come after their dependencies complete
- Added helper functions `splitAndTrimDeps`, `splitString`, `trimWhitespace`

## Files Changed
- `internal/jobs/service.go` - Added sorting after step parsing (~110 lines)

## Verify
Build: ✅ | Tests: ⏭️ (running later)
