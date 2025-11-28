# Step 2: Configure github-repo-collector.toml

- Task: task-2.md | Group: 1 | Model: sonnet

## Actions
1. Changed type from "custom" to "fetch" (semantic correctness)
2. Updated owner from "" to "ternarybob"
3. Updated repo from "" to "quaero"
4. Reduced max_files from 1000 to 10 for faster tests

## Files
- `test/config/job-definitions/github-repo-collector.toml` - updated type, owner, repo, max_files

## Decisions
- Changed type to "fetch": Matches JobDefinitionTypeFetch semantic meaning for API-based data collection
- Reduced max_files to 10: Faster test execution while still validating functionality

## Verify
Compile: N/A (TOML config) | Tests: pending

## Status: ✅ COMPLETE
