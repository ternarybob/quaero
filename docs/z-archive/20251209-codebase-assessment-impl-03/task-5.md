# Task 5: Update UAT job definition
Depends: - | Critical: no | Model: sonnet

## Addresses User Intent
Fixes the UAT job definition so it doesn't fail with "no documents found matching tags: [codebase {project_name}]" - either by using concrete values or documenting the KV setup requirement.

## Do
- Review `bin/job-definitions/codebase_assess.toml`
- Option A: Update to use concrete default project name (simpler)
- Option B: Keep placeholders but ensure test sets KV values before running
- Ensure test config `test/config/job-definitions/codebase_assess.toml` is aligned

## Accept
- [ ] Job definition doesn't fail due to unresolved placeholders
- [ ] Test and UAT configs are consistent
- [ ] Documentation updated if KV values need to be set
