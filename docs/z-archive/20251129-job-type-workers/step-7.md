# Step 7: Update Example Configurations

- Task: task-7.md | Group: 7 | Model: sonnet

## Actions
1. Updated 7 files in deployments/local/job-definitions/
2. Updated 7 files in bin/job-definitions/
3. Changed action→type in all step definitions
4. Added description fields to all steps
5. Verified build passes

## Files Updated (14 total)

### deployments/local/job-definitions/
- `agent-document-generator.toml` - type="agent"
- `agent-web-enricher.toml` - type="agent"
- `github-actions-collector.toml` - type="github_actions"
- `github-repo-collector.toml` - type="github_repo"
- `keyword-extractor-agent.toml` - type="agent"
- `nearby-restaurants-places.toml` - type="places_search"
- `news-crawler.toml` - type="crawler"

### bin/job-definitions/
- `agent-document-generator.toml` - type="agent"
- `agent-web-enricher.toml` - type="agent"
- `github-repo-collector.toml` - type="github_repo"
- `keyword-extractor-agent.toml` - type="agent"
- `nearby-restaurants-places.toml` - type="places_search"
- `news-crawler.toml` - type="crawler"
- `web-search-asx.toml` - type="web_search"

## Mapping Applied
- create_summaries, web_enrichment, scan_keywords, agent → agent
- crawl → crawler
- places_search → places_search
- github_actions_fetch → github_actions
- github_repo_fetch → github_repo
- web_search → web_search

## Verify
Compile: ✅ | Config Parse: ✅

## Status: ✅ COMPLETE
