# Step 3: Create github-repo-collector-by-name.toml config

- Task: task-3.md | Group: 2 | Model: sonnet

## Actions
1. Created new job definition file
2. Used `connector_name = "Test GitHub Connector"` instead of connector_id
3. Set unique id: "github-repo-collector-by-name"
4. Set unique name: "GitHub Repository Collector (By Name)"
5. Kept same owner/repo settings for ternarybob/quaero
6. Added "by-name-test" tag for identification

## Files
- `test/config/job-definitions/github-repo-collector-by-name.toml` - new config

## Decisions
- Used same connector name that test creates: "Test GitHub Connector"
- max_files = 10 for faster test execution

## Verify
TOML syntax: ✅

## Status: ✅ COMPLETE
