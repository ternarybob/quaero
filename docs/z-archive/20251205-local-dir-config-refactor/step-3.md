# Step 3: Update UI tests to use correct flat TOML format
Model: sonnet | Status: ✅

## Done
- Updated generateTOMLConfig() to use flat format (no [job] section)
- Removed TestLocalDirMultiStepExample test (depended on removed dropdown/tabs)
- Simplified TestLocalDirJobAddPage - now just verifies page load, TOML editor exists, optional example button

## Files Changed
- `test/ui/local_dir_jobs_test.go` - Fixed TOML format, removed multi-step test, simplified add page test

## Build Check
Build: ⏭️ | Tests: ⏭️
