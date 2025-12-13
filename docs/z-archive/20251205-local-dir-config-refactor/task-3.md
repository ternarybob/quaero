# Task 3: Update UI tests to use correct flat TOML format
Depends: 1 | Critical: no | Model: sonnet

## Addresses User Intent
Update UI tests to use the correct TOML format matching existing job definitions

## Do
- Update test/ui/local_dir_jobs_test.go
- Change generateTOMLConfig to use flat format
- Remove tests that depend on dropdown/tabs (TestLocalDirMultiStepExample)
- Update TestLocalDirJobAddPage to work with simple UI
- Ensure saveTOMLConfig saves correct format

## Accept
- [ ] TOML in tests uses flat format
- [ ] Tests don't depend on removed UI elements
- [ ] Tests compile without errors
