# Task 3: Create github-repo-collector-by-name.toml config

- Group: 2 | Mode: sequential | Model: sonnet
- Skill: @config | Critical: no | Depends: 1
- Sandbox: /tmp/3agents/task-3/ | Source: . | Output: docs/fixes/connector-name-test/

## Files
- `test/config/job-definitions/github-repo-collector-by-name.toml` - new config using connector_name

## Requirements
1. Copy structure from existing `github-repo-collector.toml`
2. Change `connector_id` to `connector_name`
3. Use a static connector name "Test GitHub Connector" (same name test creates)
4. Keep same owner/repo settings for ternarybob/quaero
5. Give it a unique job ID: "github-repo-collector-by-name"
6. Give it a unique name: "GitHub Repository Collector (By Name)"

## Acceptance
- [ ] Valid TOML syntax
- [ ] Uses connector_name instead of connector_id
- [ ] All required fields present
