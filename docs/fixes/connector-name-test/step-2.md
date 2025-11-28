# Step 2: Update github_actions_manager to support connector_name

- Task: task-2.md | Group: 1 | Model: sonnet

## Actions
1. Added `connectorName` config extraction
2. Changed validation to require either `connector_id` OR `connector_name`
3. Added logic to resolve connector by name using `GetConnectorByName()`
4. Set connectorID from resolved connector for downstream metadata/child jobs
5. Added connector_name to debug logging

## Files
- `internal/queue/managers/github_actions_manager.go` - added connector_name support

## Decisions
- Same pattern as github_repo_manager for consistency
- Priority given to connector_id if both are provided

## Verify
Compile: ⏳ | Tests: ⏳

## Status: ✅ COMPLETE
