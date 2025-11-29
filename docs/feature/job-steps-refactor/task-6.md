# Task 6: Update bin/ and deployments/ TOML Files

- Group: 6 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 5
- Sandbox: /tmp/3agents/task-6/ | Source: ./ | Output: ./docs/feature/job-steps-refactor/

## Files
- `bin/job-definitions/*.toml` - User-facing job definitions
- `deployments/local/job-definitions/*.toml` - Example/deployment job definitions

## Requirements

1. Convert each TOML file from old format to new format (same as task-5)

2. Files in bin/job-definitions/:
   - agent-document-generator.toml
   - agent-web-enricher.toml
   - keyword-extractor-agent.toml
   - Any other .toml files

3. Files in deployments/local/job-definitions/:
   - All .toml files

4. Preserve comments and documentation in files

## Acceptance
- [ ] All bin/ TOML files use new format
- [ ] All deployments/ TOML files use new format
- [ ] Application starts successfully with converted files
