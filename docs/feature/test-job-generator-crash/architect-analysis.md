# Architect Analysis: Test Job Generator Crash Investigation

## Issue Description

Executing `bin/job-definitions/test_job_generator.toml` causes the service to crash. The crash occurred during the "slow_generator" step processing.

## Log Analysis

**Log File:** `bin/logs/quaero.2025-12-17T07-39-56.log` (12786 lines, 3.1MB)

**Findings:**
1. Log ends abruptly at 07:41:49 during "slow_generator" processing
2. No FATAL/panic/error messages at end - indicates sudden termination
3. No "Status changed:" entries (new feature from current session not in that build)
4. Last entries show normal event publishing and SSE log routing
5. Two workers (job IDs `964cbc17...` and `06e16614...`) were actively processing

**Pattern:**
- Job started at 07:41:15
- Completed fast_generator and high_volume_generator steps
- Crashed ~34 seconds into slow_generator (500ms delay × ~68 iterations)
- slow_generator config: 2 workers, 300 logs each, 500ms delay = ~5 min total expected

**Root Cause Hypothesis:**

Without a stack trace, the crash is likely one of:
1. **Unrecovered panic in goroutine** - Most likely cause
2. **Out of Memory (OOM)** - Less likely given moderate log volume
3. **Badger database corruption** - Would show error messages

Given that the SSE handler has many goroutines running for event handling, and the recent changes to buffer sizes may stress memory, the crash is likely in event publishing code.

## Task 2: Create Functional Test

**Existing Pattern:** `test/ui/job_definition_general_test.go`
- Uses `UITestContext` with chromedp
- Creates job definitions via API
- Triggers jobs and monitors completion
- Has timeouts and cleanup

**Target:** Create test that runs `test/config/job-definitions/test_job_generator.toml` and monitors for completion.

### Files to Examine
- `test/ui/job_definition_general_test.go` - Pattern for job definition tests
- `test/config/job-definitions/test_job_generator.toml` - Test config to use

### Recommended Approach
EXTEND existing test patterns from `job_definition_general_test.go` to create a new test that:
1. Loads the test_job_generator.toml from test/config
2. Executes it via API
3. Monitors for completion using existing patterns
4. Asserts job completes successfully

## Summary

| Task | Type | Action |
|------|------|--------|
| Crash analysis | Research | No direct fix - crash in pre-change build |
| Functional test | CREATE | New test function in existing test file |

## CREATE Justification (Test)
- No existing test runs the full test_job_generator.toml definition
- Must create new test function following existing patterns
- Will be placed in `test/ui/job_definition_general_test.go` or new file
