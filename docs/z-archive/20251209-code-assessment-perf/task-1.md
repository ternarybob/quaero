# Task 1: Add debug logging to category filter

Depends: - | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent
Helps diagnose why classify_files is processing all files instead of just unknown ones.

## Skill Patterns to Apply
- Use arbor structured logging with key-value pairs
- Wrap errors with context

## Do
- Add logging in agent_worker.go queryDocuments to log filter values
- Add logging in search/common.go filterByMetadata to log filter/match details
- Log how many documents before/after metadata filter

## Accept
- [ ] Logs show filter values being passed
- [ ] Logs show before/after counts for metadata filtering
- [ ] Build compiles
