# Plan: Local Dir Config Refactor
Type: fix | Workdir: ./docs/fix/20251205-local-dir-config-refactor/

## User Intent (from manifest)
1. Remove the redundant `[job]` section from local_dir TOML config - job definitions should use flat top-level fields like existing jobs
2. Remove the over-engineered UI additions (dropdown with 3 examples, tab navigation in help section)
3. Update API and UI tests to use the correct TOML format
4. Run tests and fix any failures

## Tasks
| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Revert UI changes in job_add.html - remove dropdown, tabs, extra examples | - | no | sonnet |
| 2 | Update API tests to use correct flat TOML format | 1 | no | sonnet |
| 3 | Update UI tests to use correct flat TOML format | 1 | no | sonnet |
| 4 | Run API tests and fix failures | 2 | no | sonnet |
| 5 | Run UI tests and fix failures | 3 | no | sonnet |

## Order
[1] → [2, 3] → [4, 5]
