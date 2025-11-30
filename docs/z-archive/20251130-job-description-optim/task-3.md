# Task 3: Remove deprecated fields from deployments/local jobs
- Group: 3 | Mode: concurrent | Model: sonnet
- Skill: @config | Critical: no | Depends: none
- Sandbox: /tmp/3agents/task-3/ | Source: ./ | Output: ./docs/feature/20251130-job-description-optim/

## Files
- `deployments/local/job-definitions/agent-document-generator.toml` - Remove type, job_type
- `deployments/local/job-definitions/agent-web-enricher.toml` - Remove type, job_type
- `deployments/local/job-definitions/github-actions-collector.toml` - Remove type, job_type, source_type
- `deployments/local/job-definitions/github-repo-collector.toml` - Remove type, job_type, source_type
- `deployments/local/job-definitions/keyword-extractor-agent.toml` - Remove type, job_type
- `deployments/local/job-definitions/nearby-restaurants-places.toml` - Remove type, job_type
- `deployments/local/job-definitions/news-crawler.toml` - Remove type, job_type, source_type

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
