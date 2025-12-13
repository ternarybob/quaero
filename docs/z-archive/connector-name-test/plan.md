# Plan: Add connector_name support and tests for GitHub jobs

## Analysis
**Problem**: Job definitions (`github-repo-collector.toml`) only support `connector_id` but not `connector_name`. The connector service already has `GetConnectorByName` method but the managers don't use it.

**Dependencies**:
- `interfaces.ConnectorService.GetConnectorByName` - already exists
- `services/connectors/service.GetConnectorByName` - already exists
- Manager code needs modification to support both options

**Approach**:
1. Update GitHub managers to support `connector_name` as alternative to `connector_id`
2. Create new job definition config file using `connector_name`
3. Add tests for both configurations

**Risks**: Low - additive change, doesn't break existing connector_id usage

## Groups

| Task | Desc | Depends | Critical | Complexity | Model |
|------|------|---------|----------|------------|-------|
| 1 | Update github_repo_manager to support connector_name | none | no | low | sonnet |
| 2 | Update github_actions_manager to support connector_name | none | no | low | sonnet |
| 3 | Create github-repo-collector-by-name.toml config | 1 | no | low | sonnet |
| 4 | Add TestGitHubRepoCollectorByName test | 1,3 | no | medium | sonnet |
| 5 | Run tests and validate | 1,2,3,4 | no | low | sonnet |

## Order
Sequential: [1,2] (can be parallel) → Sequential: [3] → Sequential: [4] → Sequential: [5]
