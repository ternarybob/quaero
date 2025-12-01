# Task 8: Create TOML Job Definitions

- Group: 8 | Mode: concurrent | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 7
- Sandbox: /tmp/3agents/task-8/ | Source: . | Output: docs/fixes/github-collector/

## Files
- `deployments/local/job-definitions/github-repo-collector.toml` - NEW
- `deployments/local/job-definitions/github-actions-collector.toml` - NEW

## Requirements

### github-repo-collector.toml:
```toml
# GitHub Repository Content Collector
# Imports repository files as documents with folder path tracking

id = "github-repo-collector"
name = "GitHub Repository Collector"
type = "custom"
job_type = "user"
source_type = "github"
description = "Fetches repository content from specified branches and imports as searchable documents"
tags = ["github", "source-code", "repository"]
schedule = ""
timeout = "30m"
enabled = true
auto_start = false

[[steps]]
name = "fetch_repo_content"
action = "github_repo_fetch"
on_error = "continue"

[steps.config]
connector_id = "{github_connector_id}"
owner = ""
repo = ""
branches = ["main"]
extensions = [".go", ".ts", ".tsx", ".js", ".jsx", ".md", ".yaml", ".yml", ".toml", ".json"]
exclude_paths = ["vendor/", "node_modules/", ".git/", "dist/", "build/"]
max_files = 1000

[error_tolerance]
max_child_failures = 50
failure_action = "continue"
```

### github-actions-collector.toml:
```toml
# GitHub Actions Log Collector
# Imports workflow run logs with metadata (time/date, workflow info)

id = "github-actions-collector"
name = "GitHub Actions Log Collector"
type = "custom"
job_type = "user"
source_type = "github"
description = "Fetches GitHub Actions workflow logs with metadata including timestamps and workflow info"
tags = ["github", "actions", "ci-cd", "logs"]
schedule = ""
timeout = "15m"
enabled = true
auto_start = false

[[steps]]
name = "fetch_action_logs"
action = "github_actions_fetch"
on_error = "continue"

[steps.config]
connector_id = "{github_connector_id}"
owner = ""
repo = ""
limit = 20
# Optional filters:
# status_filter = "completed"
# branch_filter = "main"

[error_tolerance]
max_child_failures = 10
failure_action = "continue"
```

### Notes:
1. `connector_id` uses variable replacement pattern `{github_connector_id}`
2. Users need to update `owner` and `repo` for their specific repository
3. Both jobs use "custom" type with specific step actions
4. Tags are propagated to all created documents
5. Error tolerance allows some failures without stopping entire job

## Acceptance
- [ ] github-repo-collector.toml created with correct structure
- [ ] github-actions-collector.toml created with correct structure
- [ ] Variable replacement pattern used for connector_id
- [ ] Tags defined for categorization
- [ ] Error tolerance configured
- [ ] TOML syntax valid
- [ ] Jobs load successfully at startup
