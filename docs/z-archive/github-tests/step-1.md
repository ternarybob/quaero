# Step 1: Configure github-actions-collector.toml

- Task: task-1.md | Group: 1 | Model: sonnet

## Actions
1. Updated owner from "" to "ternarybob"
2. Updated repo from "" to "quaero"
3. Reduced limit from 20 to 5 for faster tests

## Files
- `test/config/job-definitions/github-actions-collector.toml` - updated owner, repo, limit

## Decisions
- Reduced limit to 5: Faster test execution while still validating functionality

## Verify
Compile: N/A (TOML config) | Tests: pending

## Status: ✅ COMPLETE
