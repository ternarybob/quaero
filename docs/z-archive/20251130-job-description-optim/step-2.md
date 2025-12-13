# Step 2: Remove deprecated fields from test/config jobs
- Task: task-2.md | Group: 2 | Model: opus

## Actions
1. Removed `type`, `job_type`, `source_type` from github-actions-collector.toml
2. Removed `type`, `job_type`, `source_type` from github-repo-collector.toml
3. Removed `type`, `job_type`, `source_type` from github-repo-collector-batch.toml
4. Removed `type`, `job_type`, `source_type` from github-repo-collector-by-name.toml
5. Removed `type`, `job_type` from keyword-extractor-agent.toml
6. Removed `type`, `job_type` from nearby-restaurants-places.toml
7. Removed `type`, `job_type` from test-agent-job.toml
8. Removed `type`, `job_type` from web-search-asx.toml

## Files
- `test/config/job-definitions/github-actions-collector.toml` - Removed 3 deprecated fields
- `test/config/job-definitions/github-repo-collector.toml` - Removed 3 deprecated fields
- `test/config/job-definitions/github-repo-collector-batch.toml` - Removed 3 deprecated fields
- `test/config/job-definitions/github-repo-collector-by-name.toml` - Removed 3 deprecated fields
- `test/config/job-definitions/keyword-extractor-agent.toml` - Removed 2 deprecated fields
- `test/config/job-definitions/nearby-restaurants-places.toml` - Removed 2 deprecated fields
- `test/config/job-definitions/test-agent-job.toml` - Removed 2 deprecated fields
- `test/config/job-definitions/web-search-asx.toml` - Removed 2 deprecated fields

## Decisions
- Keep all other fields (id, name, description, tags, schedule, timeout, enabled, auto_start, step sections, error_tolerance)

## Verify
Compile: N/A (TOML config) | Tests: ✅

## Status: ✅ COMPLETE
