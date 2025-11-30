# Task 2: Remove deprecated fields from test/config jobs
- Group: 2 | Mode: concurrent | Model: sonnet
- Skill: @config | Critical: no | Depends: 1
- Sandbox: /tmp/3agents/task-2/ | Source: ./ | Output: ./docs/feature/20251130-job-description-optim/

## Files
- `test/config/job-definitions/github-actions-collector.toml` - Remove type, job_type, source_type
- `test/config/job-definitions/github-repo-collector.toml` - Remove type, job_type, source_type
- `test/config/job-definitions/github-repo-collector-batch.toml` - Remove type, job_type, source_type
- `test/config/job-definitions/github-repo-collector-by-name.toml` - Remove type, job_type, source_type
- `test/config/job-definitions/keyword-extractor-agent.toml` - Remove type, job_type
- `test/config/job-definitions/nearby-restaurants-places.toml` - Remove type, job_type
- `test/config/job-definitions/test-agent-job.toml` - Remove type, job_type
- `test/config/job-definitions/web-search-asx.toml` - Remove type, job_type

## Requirements
Remove these lines from root level of each TOML file (where present):
- `type = "..."`
- `job_type = "..."`
- `source_type = "..."`

Keep all other fields intact (id, name, description, tags, schedule, timeout, enabled, auto_start, step sections, error_tolerance).

## Acceptance
- [ ] No root-level `type` field in any file
- [ ] No root-level `job_type` field in any file
- [ ] No root-level `source_type` field in any file
- [ ] All files are valid TOML
- [ ] Step sections and other config preserved
