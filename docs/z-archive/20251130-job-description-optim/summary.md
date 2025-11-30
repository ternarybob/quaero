# Complete: Job Description Optimization

## Classification
- Type: feature
- Location: ./docs/feature/20251130-job-description-optim/

Refactored all job definition TOML files across three directories to use the standardized step-based configuration structure. Key changes:
1. Ensured all jobs have the required `type` field at root level (crawler, agent, places, fetch, web_search)
2. Converted flat crawler configs to step-based structure with `[step.name]` sections
3. Removed deprecated `job_type` field (defaults to "user")
4. Removed unnecessary `source_type` field from non-crawler jobs
5. Updated README.md documentation

## Correct Job Structure

### Required Fields
- `id` - Unique job identifier
- `name` - Human-readable name
- `type` - Job type: `crawler`, `agent`, `places`, `fetch`, `web_search`, `summarizer`, `custom`
- `[step.name]` - At least one step section with worker `type`

### Optional Fields
- `description`, `schedule`, `timeout`, `enabled`, `auto_start`, `tags`

### Deprecated Fields (safe to remove)
- `job_type` - Defaults to "user" if not specified
- `source_type` - Only needed for crawler jobs with source integration

## Stats
Tasks: 5 | Files: 24 | Duration: ~10 min

## Files Modified

### bin/job-definitions/ (8 files)
- news-crawler.toml - Added type="crawler", step-based structure
- nearby-restaurants-places.toml - Added type="places"
- keyword-extractor-agent.toml - Added type="agent"
- agent-document-generator.toml - Added type="agent"
- agent-web-enricher.toml - Added type="agent"
- github-repo-collector.toml - Added type="fetch"
- github-actions-collector.toml - Added type="fetch"
- web-search-asx.toml - Added type="web_search"
- README.md - Updated documentation

### deployments/local/job-definitions/ (7 files)
- All files updated with correct type field

### test/config/job-definitions/ (10 files)
- All files updated with correct type field and step-based structure

## Verify
go test: ✅ TestJobManagement_JobQueue passed
