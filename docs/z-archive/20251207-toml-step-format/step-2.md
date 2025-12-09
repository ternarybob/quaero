# Step 2: Add test for step dependency ordering validation

Model: sonnet | Status: ✅

## Done

- Added `verifyTomlStepFormat` function that validates:
  1. No standalone `[step]` line exists
  2. `[step.generate-summary]` section exists
  3. `[step.index-files]` section exists
  4. `depends = 'index-files'` is present in the generate-summary step
- Added verification call in `TestSummaryAgentWithDependency` after creating job definition

## Files Changed

- `test/ui/local_dir_jobs_test.go` - Added `verifyTomlStepFormat` function and verification step in test

## Build Check

Build: ✅ | Tests: ⏭️ (not run yet)
