# Task 7: Update Example Configurations

- Group: 7 | Mode: concurrent | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 6
- Sandbox: /tmp/3agents/task-7/ | Source: ./ | Output: ./docs/feature/20251129-job-type-workers/

## Files
- `deployments/local/job-definitions/*.toml` - All local deployment configs
- `bin/job-definitions/*.toml` - User job definitions (if exists)

## Requirements

1. Update all deployment job definitions to new format:
   - Replace `action = "..."` with `type = "..."`
   - Add `description = "..."` field
   - Remove redundant fields

2. Files to update in deployments/local/job-definitions:
   - `agent-document-generator.toml`
   - `agent-web-enricher.toml`
   - `github-actions-collector.toml`
   - `github-repo-collector.toml`
   - `keyword-extractor-agent.toml`
   - `nearby-restaurants-places.toml`
   - `news-crawler.toml`

3. Verify all configs parse correctly:
   ```bash
   go run ./cmd/quaero validate-jobs ./deployments/local/job-definitions/
   ```

## Acceptance
- [ ] All deployment configs updated
- [ ] All user configs updated (if any)
- [ ] Configs parse without errors
- [ ] No validation warnings
