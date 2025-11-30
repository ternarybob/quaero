# Step 3: Remove deprecated fields from deployments/local jobs
- Task: task-3.md | Group: 3 | Model: opus

## Actions
1. Removed `type`, `job_type` from agent-document-generator.toml
2. Removed `type`, `job_type` from agent-web-enricher.toml
3. Removed `type`, `job_type`, `source_type` from github-actions-collector.toml
4. Removed `type`, `job_type`, `source_type` from github-repo-collector.toml
5. Removed `type`, `job_type` from keyword-extractor-agent.toml
6. Removed `type`, `job_type` from nearby-restaurants-places.toml
7. Removed `type`, `job_type`, `source_type` from news-crawler.toml

## Files
- `deployments/local/job-definitions/agent-document-generator.toml` - Removed 2 deprecated fields
- `deployments/local/job-definitions/agent-web-enricher.toml` - Removed 2 deprecated fields
- `deployments/local/job-definitions/github-actions-collector.toml` - Removed 3 deprecated fields
- `deployments/local/job-definitions/github-repo-collector.toml` - Removed 3 deprecated fields
- `deployments/local/job-definitions/keyword-extractor-agent.toml` - Removed 2 deprecated fields
- `deployments/local/job-definitions/nearby-restaurants-places.toml` - Removed 2 deprecated fields
- `deployments/local/job-definitions/news-crawler.toml` - Removed 3 deprecated fields

## Decisions
- Keep all other fields (id, name, description, tags, schedule, timeout, enabled, auto_start, step sections, error_tolerance)

## Verify
Compile: N/A (TOML config) | Tests: ✅

## Status: ✅ COMPLETE
