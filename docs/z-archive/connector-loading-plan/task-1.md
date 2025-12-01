# Task 1: Add ConnectorType for GitLab

- Group: 1 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: none
- Sandbox: /tmp/3agents/task-1/ | Source: . | Output: docs/fixes/

## Files
- `internal/models/connector.go` - Add GitLab connector type constant

## Requirements
Add GitLab as a supported connector type. The connectors.toml file shows GitLab as a type but the model only has GitHub defined.

Add:
1. `ConnectorTypeGitLab ConnectorType = "gitlab"` constant
2. `GitLabConnectorConfig` struct with Token field
3. `Validate()` method for GitLabConnectorConfig

## Acceptance
- [ ] GitLab connector type constant defined
- [ ] GitLabConnectorConfig struct with Token field
- [ ] Validate method returns error if Token is empty
- [ ] Compiles
- [ ] Tests pass
