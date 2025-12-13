# Step 2: Update API tests to use correct flat TOML format
Model: sonnet | Status: ✅

## Done
- Updated TestLocalDirJobs_TOMLUpload TOML format
- Changed from [job] section to flat format with id, name, description, tags, enabled at top level
- Kept [step.xxx] sections as-is (correct format)

## Files Changed
- `test/api/local_dir_jobs_test.go` - Fixed TOML format in upload tests

## Build Check
Build: ⏭️ | Tests: ⏭️
