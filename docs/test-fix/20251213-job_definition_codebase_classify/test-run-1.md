# Test Run 1
File: test\ui\job_definition_codebase_classify_test.go
Date: 2025-12-13T14:01:22

## Result: FAIL

## Test Output
```
=== RUN   TestJobDefinitionCodebaseClassify
    setup.go:1367: --- Testing Job Definition: Codebase Classify (with assertions) ---
    setup.go:1367: Status change:  -> running (at 0s)
    setup.go:1367: [10s] Monitoring... (status: running, WebSocket refresh_logs: 3)
    setup.go:1367: Status change: running -> completed (at 16s)
    setup.go:1367: ✓ Job reached terminal status: completed

    Assertion 0: FAIL - Log lines did not increase within first 30 seconds
    Assertion 1: PASS - WebSocket refresh_logs messages within limit (4 total)
    Assertion 1b: FAIL - /api/logs?scope=service called 4 times but only 2 triggers
    Assertion 2: PASS - All step icons match parent job icon standard
    Assertion 3: FAIL - Step 'rule_classify_files' has no logs
    Assertion 4: PASS - All steps have correct log line numbering
    Assertion 5: PASS - All 3 steps auto-expanded
```

## Failures

| Test | Error | Location |
|------|-------|----------|
| Assertion 0 | Log lines did not increase within first 30 seconds after first logs appeared - expected progressive streaming | job_definition_codebase_classify_test.go:582 |
| Assertion 1b | /api/logs?scope=service called 4 times but only 2 refresh_logs(scope=service) triggers observed | job_definition_codebase_classify_test.go:603 |
| Assertion 3 | Step 'rule_classify_files' has no logs (reason: unknown) | job_definition_codebase_classify_test.go:959 |

## Analysis

1. **Assertion 0 Failure (Progressive Logs)**: The test expects logs to increase progressively during job execution. The job completed in 16 seconds, and the first logs appeared at 3.29s, but no subsequent increase was detected. This could be:
   - The job completed too quickly for multiple samples
   - The polling interval (2s) missed intermediate updates
   - Logs are only fetched on status changes, not progressively

2. **Assertion 1b Failure (Service Logs API Calls)**: The UI made 4 service log API calls but only 2 refresh_logs triggers were received. This suggests:
   - UI is making an initial load call (allowed +1)
   - One extra call beyond allowed tolerance
   - May be a race condition with WebSocket vs API timing

3. **Assertion 3 Failure (rule_classify_files No Logs)**: The step 'rule_classify_files' showed no logs. This is likely because:
   - The step had completed but logs weren't fetched
   - The step's logs section was not properly expanded
   - The DOM query didn't find log lines for this step
