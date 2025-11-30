# Step 4: Remove deprecated fields from bin/job-definitions jobs
- Task: task-4.md | Group: 4 | Model: opus

## Actions
1. Removed `type`, `job_type` from agent-document-generator.toml
2. Removed `type`, `job_type` from agent-web-enricher.toml
3. Removed `type`, `job_type`, `source_type` from github-repo-collector.toml
4. Removed `type`, `job_type` from keyword-extractor-agent.toml
5. Removed `type`, `job_type` from nearby-restaurants-places.toml
6. Removed `type`, `job_type` from news-crawler.toml
7. Removed `type`, `job_type` from web-search-asx.toml

## Files
- `bin/job-definitions/agent-document-generator.toml` - Removed 2 deprecated fields
- `bin/job-definitions/agent-web-enricher.toml` - Removed 2 deprecated fields
- `bin/job-definitions/github-repo-collector.toml` - Removed 3 deprecated fields
- `bin/job-definitions/keyword-extractor-agent.toml` - Removed 2 deprecated fields
- `bin/job-definitions/nearby-restaurants-places.toml` - Removed 2 deprecated fields
- `bin/job-definitions/news-crawler.toml` - Removed 2 deprecated fields
- `bin/job-definitions/web-search-asx.toml` - Removed 2 deprecated fields

## Decisions
- Keep all other fields (id, name, description, tags, schedule, timeout, enabled, auto_start, step sections, error_tolerance)

## Verify
Compile: N/A (TOML config) | Tests: ✅

## Status: ✅ COMPLETE
