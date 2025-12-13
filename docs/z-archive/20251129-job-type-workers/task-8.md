# Task 8: Update Architecture Documentation

- Group: 8 | Mode: concurrent | Model: sonnet
- Skill: @docs-architect | Critical: no | Depends: 6
- Sandbox: /tmp/3agents/task-8/ | Source: ./ | Output: ./docs/feature/20251129-job-type-workers/

## Files
- `docs/architecture/manager_worker_architecture.md` - Rename from MANAGER_WORKER_ARCHITECTURE.md and update

## Requirements

1. Rename file to lowercase: `manager_worker_architecture.md`

2. Update documentation to reflect new architecture:
   - Generic StepManager concept
   - Type-defined workers
   - New TOML schema with `type` field
   - Worker interface definition
   - Execution flow diagram

3. Document new step types:
   - `agent` - AI agent document processing
   - `crawler` - Web crawling
   - `places_search` - Google Places API
   - `web_search` - Gemini web search
   - `github_repo` - GitHub repository fetching
   - `github_actions` - GitHub Actions logs
   - `transform` - Data transformation
   - `reindex` - Database reindexing

4. Include migration guide from old format to new format

## Acceptance
- [ ] File renamed to lowercase
- [ ] Architecture reflects new design
- [ ] New TOML schema documented
- [ ] Worker interface documented
- [ ] Migration guide included
