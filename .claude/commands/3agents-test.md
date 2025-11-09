---
name: 3agents-tester
description: Test workflow - review implementation docs, create/update tests, execute, and summarize results
---

Execute testing for: $ARGUMENTS

## RULES
**Test directories:** ONLY `/test/api` and `/test/ui` - maintain existing structure
**Test patterns:** Study and follow existing test patterns STRICTLY - don't invent new ones
**Test framework:** Use chromedp for UI tests, standard Go testing for API tests
**Results:** All results documented in `docs/{same-folder-as-implementation}/test-results/`
**Common utilities:** Use existing `test/common` package - don't duplicate
**Auto-continue:** Run full test suite and generate results automatically

## CONFIG
```yaml
test_api: /test/api
test_ui: /test/ui
test_common: /test/common
results_dir: docs/{task}/test-results

test_execution:
  timeout: 30s
  retries: 1  # Retry flaky tests once
  screenshot_on_failure: true
```

## SETUP
1. Read implementation docs from `docs/{task}/`
2. Create test results directory: `docs/{task}/test-results/`
3. Review existing test structure and patterns

---

## PHASE 1: ANALYSIS

### 1.1 Read Implementation Docs
Read from `docs/{task}/`:
- `work-log.md` - what was implemented
- `implementation-summary.md` - complete change list
- Any other relevant docs

### 1.2 Review Test Structure
Study existing test patterns in:
- `/test/ui/` - UI tests (chromedp-based)
- `/test/api/` - API tests
- `/test/common/` - shared test utilities

Document findings in `test-analysis.md`:
```markdown
# Test Analysis: {task}

## Implementation Changes
- {file}: {what changed}
- {file}: {what changed}

## Existing Test Coverage
**UI Tests:** ({N} files)
- {test_file.go}: {what it tests}
- {test_file.go}: {what it tests}

**API Tests:** ({N} files)
- {test_file.go}: {what it tests}
- {test_file.go}: {what it tests}

## Test Patterns Identified
**UI Test Pattern:**
- Setup: `common.SetupTestEnvironment(testName)`
- Logging: `env.LogTest(t, message)`
- WebSocket: `env.WaitForWebSocketConnection(ctx, timeout)`
- Screenshots: `env.TakeScreenshot(ctx, name)`
- Cleanup: `defer env.Cleanup()`

**API Test Pattern:**
- {pattern description}

## Test Gaps
- {feature}: No test coverage
- {feature}: Incomplete coverage
- {feature}: Test needs update due to implementation changes

## Test Plan
### New Tests Required:
- [ ] Test{Name}: {what it should test}
- [ ] Test{Name}: {what it should test}

### Tests to Update:
- [ ] {existing_test}: {why it needs update}

### Tests to Run:
- [ ] All UI tests in /test/ui
- [ ] All API tests in /test/api

**Analysis completed:** {ISO8601}
```

---

## PHASE 2: TEST CREATION/UPDATE

### 2.1 Create New Tests (if needed)
**CRITICAL:** Follow existing patterns EXACTLY

For UI tests in `/test/ui/`:
```go
func Test{Name}(t *testing.T) {
    // Setup test environment with test name
    env, err := common.SetupTestEnvironment("{TestName}")
    if err != nil {
        t.Fatalf("Failed to setup test environment: %v", err)
    }
    defer env.Cleanup()

    startTime := time.Now()
    env.LogTest(t, "=== RUN Test{Name}")
    defer func() {
        elapsed := time.Since(startTime)
        if t.Failed() {
            env.LogTest(t, "--- FAIL: Test{Name} (%.2fs)", elapsed.Seconds())
        } else {
            env.LogTest(t, "--- PASS: Test{Name} (%.2fs)", elapsed.Seconds())
        }
    }()

    env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
    env.LogTest(t, "Results directory: %s", env.GetResultsDir())

    ctx, cancel := chromedp.NewContext(context.Background())
    defer cancel()

    ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Test implementation here
    // Use env.LogTest() for all logging
    // Use env.TakeScreenshot() for visual verification
    // Use env.WaitForWebSocketConnection() if testing real-time features
}
```

For API tests in `/test/api/`:
- Follow existing patterns in that directory
- Study existing tests first

### 2.2 Update Existing Tests (if needed)
- Modify only what's necessary to reflect implementation changes
- Preserve test structure and patterns
- Document changes in test-analysis.md

### 2.3 Document Test Changes
Update `test-analysis.md` with:
```markdown
## Test Creation/Updates

### Created:
- `/test/ui/{file}.go`: Test{Name}
  - **Purpose:** {what it tests}
  - **Pattern used:** {which existing test was used as template}
  - **Coverage:** {what scenarios are covered}

### Updated:
- `/test/ui/{file}.go`: Test{Name}
  - **Changes:** {what was modified}
  - **Reason:** {why it was needed}

**Test prep completed:** {ISO8601}
```

---

## PHASE 3: TEST EXECUTION

### 3.1 Run UI Tests
```bash
cd /test/ui
go test -v
```

Capture:
- Exit code
- Full output
- Test timing
- Pass/fail counts

### 3.2 Run API Tests
```bash
cd /test/api
go test -v
```

Capture:
- Exit code
- Full output
- Test timing
- Pass/fail counts

### 3.3 Document Execution
Create `test-execution.md`:
```markdown
# Test Execution: {task}

**Executed:** {ISO8601}

## UI Tests (/test/ui)

**Command:** `cd /test/ui && go test -v`
**Duration:** {time}
**Exit Code:** {code}

### Results
```
{full go test output}
```

**Summary:**
- Total: {N}
- Passed: ✅ {N}
- Failed: ❌ {N}
- Skipped: ⏭️ {N}

### Individual Test Results
- ✅ Test{Name} ({duration}s)
- ❌ Test{Name} ({duration}s) - {error summary}
- ✅ Test{Name} ({duration}s)

### Screenshots Generated
- {test-name}/{screenshot-name}.png
- {test-name}/{screenshot-name}.png

---

## API Tests (/test/api)

**Command:** `cd /test/api && go test -v`
**Duration:** {time}
**Exit Code:** {code}

### Results
```
{full go test output}
```

**Summary:**
- Total: {N}
- Passed: ✅ {N}
- Failed: ❌ {N}
- Skipped: ⏭️ {N}

### Individual Test Results
- ✅ Test{Name} ({duration}s)
- ❌ Test{Name} ({duration}s) - {error summary}

---

## Overall Status
**Status:** {PASS | FAIL | PARTIAL}
**Total Tests:** {N}
**Pass Rate:** {XX.X}%

**Execution completed:** {ISO8601}
```

---

## PHASE 4: RESULTS ANALYSIS

### 4.1 Analyze Failures
For each failed test, create detailed analysis:

```markdown
## Failure Analysis

### Test{Name} - FAILED
**Duration:** {time}
**Error:** {error message}

**Root Cause:**
{Analysis of why it failed}

**Related Changes:**
- {implementation change that may have caused this}

**Screenshots:**
{Reference to failure screenshots if available}

**Recommendation:**
- [ ] Implementation needs fix: {what needs fixing}
- [ ] Test needs update: {what needs updating}
- [ ] Known issue: {describe if expected}

---
```

### 4.2 Coverage Assessment
```markdown
## Coverage Assessment

**Implementation Changes:** {N}
**Tests Created:** {N}
**Tests Updated:** {N}
**Tests Executed:** {N}

### Coverage by Feature:
- {feature}: ✅ Fully tested
- {feature}: ⚠️ Partially tested ({reason})
- {feature}: ❌ Not tested ({reason})

### Coverage Gaps:
- {feature/scenario}: {why not covered}
- {feature/scenario}: {why not covered}

**Recommendations:**
{What additional tests would improve coverage}
```

---

## PHASE 5: SUMMARY

Create `TEST-SUMMARY.md` in `docs/{task}/test-results/`:

```markdown
# Test Summary: {task}

**Date:** {ISO8601}
**Implementation Docs:** `docs/{task}/`
**Test Results:** `docs/{task}/test-results/`

---

## Executive Summary

**Overall Status:** {PASS ✅ | FAIL ❌ | PARTIAL ⚠️}

- Implementation changes tested: {N}/{N}
- Tests executed: {N}
- Tests passed: {N}
- Tests failed: {N}
- Pass rate: {XX.X}%

---

## Test Execution Results

### UI Tests (/test/ui)
**Status:** {PASS | FAIL}
**Duration:** {time}
**Pass Rate:** {XX.X}%

| Test | Status | Duration | Notes |
|------|--------|----------|-------|
| Test{Name} | ✅ | {time}s | {notes} |
| Test{Name} | ❌ | {time}s | {error summary} |
| Test{Name} | ✅ | {time}s | {notes} |

### API Tests (/test/api)
**Status:** {PASS | FAIL}
**Duration:** {time}
**Pass Rate:** {XX.X}%

| Test | Status | Duration | Notes |
|------|--------|----------|-------|
| Test{Name} | ✅ | {time}s | {notes} |
| Test{Name} | ❌ | {time}s | {error summary} |

---

## Test Coverage

**Changes Requiring Tests:** {N}
**Tests Created:** {N}
**Tests Updated:** {N}

### New Tests Added:
- `/test/ui/{file}.go`: Test{Name} - {purpose}
- `/test/ui/{file}.go`: Test{Name} - {purpose}

### Tests Updated:
- `/test/ui/{file}.go`: Test{Name} - {what changed}

### Coverage Gaps:
- {feature}: {reason for gap}

---

## Failures & Issues

{If no failures:}
✅ **All tests passed - no issues found**

{If failures exist:}
### Critical Failures (Block Release)
1. **Test{Name}** - {error}
   - **Impact:** {description}
   - **Fix Required:** {what needs to be done}

### Non-Critical Failures (Can Release)
1. **Test{Name}** - {error}
   - **Impact:** {description}
   - **Fix Recommended:** {what should be done}

### Known Issues (Expected)
- {description of expected failures}

---

## Test Artifacts

### Screenshots
{List of screenshot directories/files generated}
- `test-results/{test-name}/`

### Logs
- `test-execution.md` - Full test output
- `test-analysis.md` - Test planning and analysis

---

## Recommendations

### Immediate Actions Required:
- [ ] {action based on test results}
- [ ] {action based on test results}

### Future Test Improvements:
- {suggestion for better coverage}
- {suggestion for test refactoring}

### Implementation Feedback:
- {any issues found in implementation that need fixing}
- {any suggested improvements}

---

## Sign-Off

**Testing completed:** {ISO8601}
**Tested by:** Claude Sonnet 4.5 (tester command)

**Status for Release:**
- ✅ APPROVED - All tests passing
- ⚠️ APPROVED WITH ISSUES - See non-critical failures
- ❌ BLOCKED - Critical failures must be resolved

---

## Next Steps

{If all passed:}
✅ Implementation is ready
- Review TEST-SUMMARY.md
- Run implementation command again if needed
- Deploy/merge changes

{If failures exist:}
❌ Implementation needs fixes
1. Review failure analysis above
2. Fix issues in implementation
3. Rerun implementer command with fixes
4. Rerun tester command to verify

**Resume command:** `Continue` or re-run `implementer` then `tester`
```

---

## VALIDATION CHECKS

Before completing:
- ✅ All test directories reviewed (`/test/api`, `/test/ui`)
- ✅ Test patterns followed (studied existing tests)
- ✅ Both test suites executed (`go test -v` in both directories)
- ✅ Results documented in `docs/{task}/test-results/`
- ✅ Failures analyzed (if any)
- ✅ Summary created (`TEST-SUMMARY.md`)
- ✅ Screenshots/logs preserved
- ✅ Recommendations provided

---

## TEST STRUCTURE MAINTENANCE

**Directory Structure:**
```
/test/
├── api/           # API tests (Go standard testing)
│   └── *.go
├── ui/            # UI tests (chromedp-based)
│   └── *.go
└── common/        # Shared test utilities
    └── *.go
```

**Pattern Preservation:**
- Always use `common.SetupTestEnvironment()`
- Always use `env.LogTest()` for logging
- Always use `env.TakeScreenshot()` for visual verification
- Always use `defer env.Cleanup()`
- Follow existing test naming: `Test{Feature}{Scenario}`
- Follow existing test structure (setup, execute, verify, cleanup)

---

**Task:** $ARGUMENTS  
**Docs:** `docs/{task}/test-results/`  
**Mode:** Full test suite with comprehensive results