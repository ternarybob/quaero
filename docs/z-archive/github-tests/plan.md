# Plan: GitHub API and UI Tests Configuration and Execution

## Analysis

### Dependencies
- Test config files: `test/config/job-definitions/github-actions-collector.toml`, `test/config/job-definitions/github-repo-collector.toml`
- Environment: `test/config/.env.test` contains `github_test_token`
- Target repo: `https://github.com/ternarybob/quaero` (owner: `ternarybob`, repo: `quaero`)
- API test: `test/api/github_jobs_test.go` - already exists
- UI test template: `test/ui/queue_test.go` - use as template for GitHub jobs UI test

### Approach
1. Configure test TOML files with correct owner/repo values
2. Fix `type` in github-repo-collector.toml (custom → fetch)
3. Run existing API tests to identify any issues
4. Create new UI test file for GitHub jobs
5. Iterate on code fixes (not tests) to make tests pass

### Risks
- GitHub API rate limiting during tests
- Token permissions may be insufficient
- Connector creation/lookup may fail if `github_test_token` isn't properly mapped

## Groups

| Task | Desc | Depends | Critical | Complexity | Model |
|------|------|---------|----------|------------|-------|
| 1 | Configure github-actions-collector.toml with ternarybob/quaero | none | no | low | sonnet |
| 2 | Configure github-repo-collector.toml with ternarybob/quaero and fix type | none | no | low | sonnet |
| 3 | Run API tests and capture results | 1,2 | no | medium | sonnet |
| 4 | Create UI test for GitHub jobs | 1,2 | no | medium | sonnet |
| 5 | Run UI tests and iterate on code fixes | 3,4 | no | high | sonnet |

## Order
Concurrent: [1,2] → Sequential: [3] → Sequential: [4] → Sequential: [5]
