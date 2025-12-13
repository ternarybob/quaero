# Plan: Test Assertions for Codebase Classify
Type: fix | Workdir: ./docs/fix/20251212-test-assertions/ | Date: 2025-12-12

## Context
Project: Quaero
Related files:
- `test/ui/job_definition_codebase_classify_test.go` - main test file to update
- `test/ui/job_framework_test.go` - test framework with helper methods

## User Intent (from manifest)
1. Test WITHOUT page refresh - monitor in real-time via WebSocket
2. API call count assertion - Step Log API < 10 requests (excluding service logs)
3. Auto-expand verification - steps expand in order as they complete
4. Log display assertion - Steps show logs 1→15 (not 5→15)

## Success Criteria (from manifest)
- [ ] Test monitors job WITHOUT page refresh (uses WebSocket/real-time updates)
- [ ] Test asserts Step Log API request count < 10 (excluding service logs)
- [ ] Test asserts all steps auto-expand in order of completion
- [ ] Test asserts Step 1 (import_files) shows logs 1 → 15 (not 5 → 15)
- [ ] Test asserts Step 2 (code_map) shows logs 1 → 15
- [ ] Test passes when run

## Active Skills
| Skill | Key Patterns to Apply |
|-------|----------------------|
| go | Table-driven tests, context passing, error wrapping |

## Technical Approach
1. Rewrite the test to not use the generic `RunJobDefinitionTest` which refreshes the page
2. Add custom monitoring that tracks API calls via Chrome DevTools Protocol
3. Add assertions for step expansion state via JavaScript evaluation
4. Add assertions for log line numbers displayed in the UI

## Files to Change
| File | Action | Purpose |
|------|--------|---------|
| test/ui/job_definition_codebase_classify_test.go | modify | Add detailed assertions |

## Tasks
| # | Desc | Depends | Critical | Model | Skill | Est. Files |
|---|------|---------|----------|-------|-------|------------|
| 1 | Rewrite test with API call tracking and no-refresh monitoring | - | no | opus | go | 1 |
| 2 | Run test and verify all assertions pass | 1 | no | opus | - | 0 |

## Execution Order
[1] → [2]

## Risks/Decisions
- Chrome DevTools Protocol for network monitoring may need specific setup
- Step names must match exactly (import_files, code_map, rule_classify_files)
- Timing is critical - must wait for WebSocket updates without refreshing
