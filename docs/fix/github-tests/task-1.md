# Task 1: Configure github-actions-collector.toml

- Group: 1 | Mode: concurrent | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: none
- Sandbox: /tmp/3agents/task-1/ | Source: ./ | Output: docs/fixes/github-tests/

## Files
- `test/config/job-definitions/github-actions-collector.toml` - edit owner/repo values

## Requirements
1. Update `owner` field from empty string to `ternarybob`
2. Update `repo` field from empty string to `quaero`
3. Ensure `type = "fetch"` is correct (already set)
4. Reduce `limit` to 5 for faster tests

## Acceptance
- [x] owner = "ternarybob"
- [x] repo = "quaero"
- [x] type = "fetch"
- [x] limit = 5
- [x] Compiles (N/A - TOML config)
- [x] Valid TOML syntax
