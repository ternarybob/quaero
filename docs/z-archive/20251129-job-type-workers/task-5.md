# Task 5: Update Test Job Definitions

- Group: 5 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 4
- Sandbox: /tmp/3agents/task-5/ | Source: ./ | Output: ./docs/feature/20251129-job-type-workers/

## Files
- `test/config/job-definitions/github-actions-collector.toml`
- `test/config/job-definitions/github-repo-collector.toml`
- `test/config/job-definitions/github-repo-collector-batch.toml`
- `test/config/job-definitions/github-repo-collector-by-name.toml`
- `test/config/job-definitions/keyword-extractor-agent.toml`
- `test/config/job-definitions/nearby-restaurants-places.toml`
- `test/config/job-definitions/test-agent-job.toml`
- `test/config/job-definitions/web-search-asx.toml`

## Requirements

1. Update each job definition to new format:
   - Replace `action = "..."` with `type = "..."`
   - Add `description = "..."` field
   - Remove redundant `name = "..."` in step config (step name is in `[step.{name}]`)

2. New step format example:
   ```toml
   [step.extract_keywords]
   type = "agent"
   description = "Extract keywords from documents using AI"
   on_error = "fail"
   agent_type = "keyword_extractor"
   # ... other config
   ```

3. Remove parent-level `type` field if it only duplicates step type

4. Ensure all placeholder references remain intact

## Acceptance
- [ ] All test job definitions updated to new format
- [ ] `type` field used instead of `action`
- [ ] `description` field added to all steps
- [ ] Redundant fields removed
- [ ] Compiles: `go build ./...`
- [ ] Job definitions parse without errors
