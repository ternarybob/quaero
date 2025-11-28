# Task 2: Configure github-repo-collector.toml

- Group: 1 | Mode: concurrent | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: none
- Sandbox: /tmp/3agents/task-2/ | Source: ./ | Output: docs/fixes/github-tests/

## Files
- `test/config/job-definitions/github-repo-collector.toml` - edit type, owner/repo values

## Requirements
1. Change `type` from `custom` to `fetch` (semantic correctness)
2. Update `owner` field from empty string to `ternarybob`
3. Update `repo` field from empty string to `quaero`
4. Reduce `max_files` to 10 for faster tests

## Acceptance
- [x] type = "fetch"
- [x] owner = "ternarybob"
- [x] repo = "quaero"
- [x] max_files = 10
- [x] Compiles (N/A - TOML config)
- [x] Valid TOML syntax
