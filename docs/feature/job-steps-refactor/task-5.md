# Task 5: Update Test TOML Files

- Group: 5 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 4
- Sandbox: /tmp/3agents/task-5/ | Source: ./ | Output: ./docs/feature/job-steps-refactor/

## Files
- `test/config/job-definitions/*.toml` - All test job definition files

## Requirements

1. Convert each test TOML file from old format to new format:

   ```toml
   # OLD
   [[steps]]
   name = "step_name"
   action = "agent"
   [steps.config]
   agent_type = "keyword_extractor"
   [steps.config.document_filter]
   limit = 100

   # NEW
   [step.step_name]
   action = "agent"
   agent_type = "keyword_extractor"
   filter_limit = 100
   ```

2. Files to update:
   - keyword-extractor-agent.toml
   - test-agent-job.toml
   - web-search-asx.toml
   - github-repo-collector.toml
   - github-repo-collector-batch.toml
   - github-repo-collector-by-name.toml
   - github-actions-collector.toml
   - nearby-restaurants-places.toml
   - news-crawler.toml (if has steps)
   - my-custom-crawler.toml (if has steps)

3. Run tests after conversion:
   ```bash
   go test ./test/api/... ./test/ui/... -v
   ```

## Acceptance
- [ ] All test TOML files use new [step.name] format
- [ ] Tests pass: `go test ./test/...`
- [ ] Jobs load correctly from test config
