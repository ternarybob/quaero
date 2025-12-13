# Task 2: Update API tests to use correct flat TOML format
Depends: 1 | Critical: no | Model: sonnet

## Addresses User Intent
Update API tests to use the correct TOML format matching existing job definitions

## Do
- Update test/api/local_dir_jobs_test.go
- Change TOML format from [job]/[step.xxx] to flat format (id, name at top level, then [step.xxx])
- Update createTestLocalDirJobDefinition to use correct format
- Update TestLocalDirJobs_TOMLUpload to use correct format

## Accept
- [ ] TOML in tests uses flat format matching github-git-collector.toml
- [ ] No [job] section in any TOML
- [ ] Tests compile without errors
