# Feature: Test Replication

Date: 2025-12-14
Request: "The test test\ui\job_definition_general_test.go needs to replicate test\ui\job_definition_codebase_classify_test.go assertions. 1. Testing whilst job is running, not page refresh and screenshots. 2. assessing/assert job status, and only finish once job is complete, with timeout 5 minutes. 3. The test should be configured to run the error_generator job, as now. 4. Add another step, same error_generator job, but with a different name."

## User Intent

Replicate the comprehensive test assertions from `job_definition_codebase_classify_test.go` into `job_definition_general_test.go` so that:
1. Tests monitor jobs in real-time via WebSocket updates (no page refresh)
2. Tests assess job status continuously and only finish when job reaches terminal state
3. Tests use the error_generator worker (as currently configured)
4. Tests include a second step with the same error_generator job but different name

## Success Criteria

- [ ] Test monitors job execution via WebSocket (no page refresh during execution)
- [ ] Test captures screenshots at status changes (not polling-based)
- [ ] Test waits for job completion with 5 minute timeout
- [ ] Test asserts API vs UI status consistency during execution
- [ ] Test asserts step expansion and log line numbering
- [ ] Job definition includes two error_generator steps with different names
- [ ] Build passes
- [ ] Test passes

## Applicable Architecture Requirements

| Doc | Section | Requirement |
|-----|---------|-------------|
| QUEUE_UI.md | Step Expansion | ALL steps should auto-expand when they start running |
| QUEUE_UI.md | Icon Standards | Step icons MUST match parent job icon standard |
| QUEUE_UI.md | API Calls | Step log API calls should be < 10 per job execution |
| QUEUE_LOGGING.md | Log Line Numbering | Log lines MUST start at line 1 and increment sequentially |
| QUEUE_LOGGING.md | WebSocket Events | UI uses trigger-based fetching via WebSocket events |
| QUEUE_SERVICES.md | Event Service | EventJobLog, EventJobStatusChange events for real-time updates |
