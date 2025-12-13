# Step 1: Fix saveJobToml to not output redundant [step] line

Model: sonnet | Status: ✅

## Done

- Modified `saveJobToml` function to remove redundant `[step]` line after TOML marshalling
- Added `strings.Replace(tomlStr, "[step]\n", "", 1)` to strip the unwanted header
- The go-toml library generates `[step]` for nested map structures, but we only want `[step.{name}]` sections
- **Updated:** Convert `depends` string to array format for TOML output (e.g., `depends = ['index-files']`)

## Files Changed

- `test/ui/local_dir_jobs_test.go` - Added post-processing to remove `[step]` line and convert `depends` to array

## Build Check

Build: ✅ | Tests: ⏭️ (not run yet)
