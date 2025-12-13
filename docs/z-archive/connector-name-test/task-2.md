# Task 2: Update github_actions_manager to support connector_name

- Group: 1 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: none
- Sandbox: /tmp/3agents/task-2/ | Source: . | Output: docs/fixes/connector-name-test/

## Files
- `internal/queue/managers/github_actions_manager.go` - add connector_name support

## Requirements
1. Add `connector_name` config extraction alongside `connector_id`
2. If `connector_id` is empty, check for `connector_name`
3. If `connector_name` is provided, use `connectorService.GetConnectorByName()`
4. If neither is provided, return error
5. Maintain backward compatibility with existing `connector_id` usage

## Acceptance
- [ ] connector_id continues to work as before
- [ ] connector_name can be used as alternative
- [ ] Error message is clear when neither is provided
- [ ] Compiles
