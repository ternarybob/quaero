# Test Run 2
File: test\ui\job_definition_codebase_classify_test.go
Date: 2025-12-13T14:13:07

## Result: FAIL (2 failures, improvement from 3)

## Test Output Summary
```
- Job completed in ~19 seconds
- WebSocket refresh_logs: 33 total (30 job, 3 service)
- API calls: 204 job logs, 5 service logs
```

## Assertions

| # | Test | Result | Details |
|---|------|--------|---------|
| 0 | Progressive logs | FAIL | Log lines did not increase within first 30s - firstIncrease@-1ns |
| 1 | WebSocket message count | PASS | 33 total < 40 limit |
| 1b | Service logs API gating | FAIL | 5 calls vs 3 triggers (+1 allowed), expected ≤4, got 5 |
| 2 | Step icons | PASS | All 3 icons correct |
| 3 | All steps have logs | PASS | code_map:13, import_files:3, rule_classify_files:100 |
| 4 | Log line numbering | PASS | Sequential/monotonic for all steps |
| 5 | Auto-expand | PASS | All 3 steps auto-expanded |

## Analysis

### Assertion 0 (Progressive Logs) - FAIL
The test tracks DOM log counts every 2 seconds looking for an increase.
- First logs appeared at 2.58s
- No increase detected (firstIncrease@-1ns)
- But rule_classify_files ended with 2500+ logs (showing 100 of 2531)

**Possible causes:**
1. Logs appear initially when step expands, but subsequent refreshes don't add new visible logs (already at 100 limit)
2. The step with progressive logs (rule_classify_files) wasn't expanded early enough
3. The test's 2-second sampling missed the rapid increase

### Assertion 1b (Service Logs API Calls) - FAIL
- 5 API calls to /api/logs?scope=service
- 3 refresh_logs triggers with scope=service
- Test allows triggers + 1 = 4 max

**Root cause:**
- Test navigates to Jobs page → serviceLogs init() → API call 1
- Test navigates to Queue page → serviceLogs init() → API call 2
- 3 triggers from rate-adaptive aggregator → API calls 3-5
- Total: 5 (2 initial + 3 triggers)
- Expected: 4 (3 triggers + 1 initial)

The test expectation of +1 doesn't account for the 2-page navigation.

## Improvements from Previous Run
- Assertion 3: **Fixed** - rule_classify_files now has logs (was empty)
- WebSocket triggers increased from 4 to 33 (rate-adaptive working)
