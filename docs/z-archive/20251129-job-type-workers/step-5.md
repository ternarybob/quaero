# Step 5: Update Test Job Definitions

- Task: task-5.md | Group: 5 | Model: sonnet

## Actions
1. Updated all 8 test job definitions to new format
2. Changed `action` to `type` using correct StepType values
3. Added `description` field to all steps
4. Verified no redundant `name` fields in step configs

## Files
- `test/config/job-definitions/github-actions-collector.toml` - type="github_actions"
- `test/config/job-definitions/github-repo-collector.toml` - type="github_repo"
- `test/config/job-definitions/github-repo-collector-batch.toml` - type="github_repo"
- `test/config/job-definitions/github-repo-collector-by-name.toml` - type="github_repo"
- `test/config/job-definitions/keyword-extractor-agent.toml` - type="agent"
- `test/config/job-definitions/nearby-restaurants-places.toml` - type="places_search"
- `test/config/job-definitions/test-agent-job.toml` - type="agent"
- `test/config/job-definitions/web-search-asx.toml` - type="web_search"

## Mapping Applied
- github_actions_fetch → github_actions
- github_repo_fetch → github_repo
- agent → agent
- places_search → places_search
- web_search → web_search

## Verify
Compile: ✅ | Config Parse: ✅

## Status: ✅ COMPLETE
