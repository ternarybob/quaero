---
name: 3agents-tester
description: Test workflow - review 3agents implementation, create/update tests with @test-writer skill, execute, and provide feedback
---

Execute testing for: $ARGUMENTS

## RULES
**Integration:** Reads from 3agents `docs/{task}/` output - designed to run AFTER 3agents completes
**Test directories:** ONLY `/test/api` and `/test/ui` - maintain existing structure
**Test patterns:** Study and follow existing test patterns STRICTLY - don't invent new ones
**Test framework:** Use chromedp for UI tests, standard Go testing for API tests
**Results:** All results documented in `docs/{task}/test-results/`
**Common utilities:** Use existing `test/common` package - don't duplicate
**Skill:** Use @test-writer for test creation/updates
**Feedback loop:** Generate actionable feedback for 3agents if tests fail
**Auto-continue:** Run full workflow automatically, pause only on critical failures

## CONFIG
````yaml
test_api: /test/api
test_ui: /test/ui
test_common: /test/common
source_docs: docs/{task}
results_dir: docs/{task}/test-results

test_execution:
  timeout: 30s
  retries: 1  # Retry flaky tests once
  screenshot_on_failure: true

feedback:
  create_on_failure: true
  format: 3agents-compatible  # Matches 3agents step format
  auto_create_fix_steps: true  # Generate fix steps for 3agents
````

## SETUP
1. Verify 3agents docs exist at `docs/{task}/`
2. Read implementation from 3agents output:
   - `plan.md` - original plan
   - `progress.md` - what was implemented
   - `summary.md` - completion summary
3. Create test results directory: `docs/{task}/test-results/`
4. Review existing test structure and patterns

---

## PHASE 1: ANALYSIS

### 1.1 Read 3agents Implementation
**Read from `docs/{task}/`:**
- `plan.md` - understand original requirements and steps
- `progress.md` - see what was completed
- `summary.md` - review final implementation
- Any `step-{N}-validation-*.md` files - understand what was validated

**Document in `test-analysis.md`:**
````markdown
# Test Analysis: {task}

**Source:** 3agents output from `docs/{task}/`
**Analyzed:** {ISO8601}

---

## 3agents Implementation Summary

**Original task:** {from plan.md}
**Steps completed:** {N}/{N}
**Quality rating:** {from summary.md}

### Implementation Changes
{Parse from progress.md and summary.md}
- Step {N}: {what was implemented}
  - Files: {list}
  - Skill used: @{skill}
  - Risk: {low|medium|high}

### Key Artifacts Created/Modified
- `{file}`: {description of changes}
- `{file}`: {description of changes}

---

## Existing Test Coverage

### Current Test Structure
**UI Tests:** ({N} files in `/test/ui`)
- {test_file.go}: {what it tests} - {last modified}
- {test_file.go}: {what it tests} - {last modified}

**API Tests:** ({N} files in `/test/api`)
- {test_file.go}: {what it tests} - {last modified}
- {test_file.go}: {what it tests} - {last modified}

### Test Patterns Identified
**UI Test Pattern:**
```go
// Standard pattern from existing tests
func Test{Name}(t *testing.T) {
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
    
    // Test implementation
}
```

**API Test Pattern:**
{Document pattern found in /test/api}

**Common Utilities Available:**
- `common.SetupTestEnvironment(name)` - Test setup
- `env.LogTest(t, format, args...)` - Logging
- `env.TakeScreenshot(ctx, name)` - Screenshots
- `env.WaitForWebSocketConnection(ctx, timeout)` - WebSocket testing
- `env.Cleanup()` - Teardown
- {list other utilities found}

---

## Test Gap Analysis

### Features Requiring Test Coverage
{Map implementation steps to test requirements}

**From Step {N}:** {description}
- **Test needed:** UI test for {scenario}
- **Existing coverage:** None | Partial in {test_file}
- **Priority:** Critical | High | Medium | Low
- **Risk if untested:** {potential issues}

**From Step {N}:** {description}
- **Test needed:** API test for {scenario}
- **Existing coverage:** None | Partial in {test_file}
- **Priority:** Critical | High | Medium | Low
- **Risk if untested:** {potential issues}

### Tests Requiring Updates
- `{test_file.go}`: {existing test} needs update because {reason from implementation}
- `{test_file.go}`: {existing test} needs update because {reason from implementation}

---

## Test Plan

### New Tests to Create (Using @test-writer):
**UI Tests:**
- [ ] **Test{Name}** - `/test/ui/{file}.go`
  - **Covers:** Step {N} - {description}
  - **Scenarios:** {list key test scenarios}
  - **Pattern:** Based on {existing_test.go}
  - **Priority:** {Critical|High|Medium|Low}

- [ ] **Test{Name}** - `/test/ui/{file}.go`
  - **Covers:** Step {N} - {description}
  - **Scenarios:** {list key test scenarios}
  - **Pattern:** Based on {existing_test.go}
  - **Priority:** {Critical|High|Medium|Low}

**API Tests:**
- [ ] **Test{Name}** - `/test/api/{file}.go`
  - **Covers:** Step {N} - {description}
  - **Scenarios:** {list key test scenarios}
  - **Pattern:** Based on {existing_test.go}
  - **Priority:** {Critical|High|Medium|Low}

### Existing Tests to Update:
- [ ] **{test_file.go}**: Test{Name}
  - **Update needed:** {what to modify}
  - **Reason:** Implementation changed in Step {N}

### All Tests to Execute:
- [ ] Run `/test/ui` suite
- [ ] Run `/test/api` suite

**Analysis completed:** {ISO8601}
````

---

## PHASE 2: TEST CREATION/UPDATE (@test-writer)

### 2.1 Invoke @test-writer Skill
**For EACH new test identified in test plan:**

**Context to provide @test-writer:**
- Implementation step it's testing (from 3agents docs)
- Existing test pattern to follow (from test-analysis.md)
- Test scenario requirements
- Files affected by implementation

**@test-writer creates test following:**

**UI Test Template (`/test/ui/{feature}_test.go`):**
````go
package main

import (
    "context"
    "testing"
    "time"

    "github.com/chromedp/chromedp"
    "quaero/test/common"
)

func Test{Feature}{Scenario}(t *testing.T) {
    // Setup test environment with descriptive test name
    env, err := common.SetupTestEnvironment("{Feature}{Scenario}")
    if err != nil {
        t.Fatalf("Failed to setup test environment: %v", err)
    }
    defer env.Cleanup()

    startTime := time.Now()
    env.LogTest(t, "=== RUN Test{Feature}{Scenario}")
    defer func() {
        elapsed := time.Since(startTime)
        if t.Failed() {
            env.LogTest(t, "--- FAIL: Test{Feature}{Scenario} (%.2fs)", elapsed.Seconds())
        } else {
            env.LogTest(t, "--- PASS: Test{Feature}{Scenario} (%.2fs)", elapsed.Seconds())
        }
    }()

    env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
    env.LogTest(t, "Results directory: %s", env.GetResultsDir())

    ctx, cancel := chromedp.NewContext(context.Background())
    defer cancel()

    ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Test implementation - Testing: {description from 3agents Step N}
    env.LogTest(t, "Starting test: {scenario description}")
    
    // Navigate, interact, verify
    err = chromedp.Run(ctx,
        chromedp.Navigate(env.GetBaseURL()),
        // Test actions here following existing patterns
    )
    
    if err != nil {
        env.TakeScreenshot(ctx, "failure")
        t.Fatalf("Test failed: %v", err)
    }
    
    env.LogTest(t, "Test completed successfully")
}
````

**API Test Template (`/test/api/{feature}_test.go`):**
{Follow existing API test patterns in /test/api}

### 2.2 Update Existing Tests
**Using @test-writer:**
- Modify only what's necessary to reflect 3agents implementation changes
- Preserve test structure and patterns
- Document all changes

### 2.3 Document Test Creation
**Update `test-analysis.md`:**
````markdown
## Test Creation/Updates (@test-writer)

**Skill used:** @test-writer
**Created:** {ISO8601}

### Tests Created:
- `/test/ui/{file}.go`: **Test{Name}**
  - **Covers 3agents Step:** {N} - {description}
  - **Purpose:** {what specific scenario it tests}
  - **Pattern source:** Based on `{existing_test.go}`
  - **Test scenarios:**
    1. {scenario 1}
    2. {scenario 2}
  - **Expected behavior:** {what should happen}
  - **Priority:** {Critical|High|Medium|Low}

- `/test/api/{file}.go`: **Test{Name}**
  - **Covers 3agents Step:** {N} - {description}
  - **Purpose:** {what specific scenario it tests}
  - **Pattern source:** Based on `{existing_test.go}`
  - **Priority:** {Critical|High|Medium|Low}

### Tests Updated:
- `/test/ui/{file}.go`: **Test{Name}**
  - **Changes:** {what was modified}
  - **Reason:** 3agents Step {N} changed {what}
  - **Impact:** {what behavior changed}

### Tests Compilation Check:
```bash
cd /test/ui && go build
cd /test/api && go build
```
✅ All tests compile successfully
❌ Compilation errors: {details}

**Test prep completed:** {ISO8601}
````

---

## PHASE 3: TEST EXECUTION

### 3.1 Run UI Tests
````bash
cd /test/ui
go test -v -timeout 60s
````

**Capture:**
- Exit code
- Full output (stdout + stderr)
- Individual test results
- Test timing
- Screenshots generated

### 3.2 Run API Tests
````bash
cd /test/api
go test -v -timeout 60s
````

**Capture:**
- Exit code
- Full output
- Individual test results
- Test timing

### 3.3 Document Execution
**Create `test-execution.md`:**
````markdown
# Test Execution: {task}

**Testing 3agents implementation from:** `docs/{task}/`
**Executed:** {ISO8601}

---

## UI Tests (/test/ui)

**Command:** `cd /test/ui && go test -v -timeout 60s`
**Duration:** {MM:SS}
**Exit Code:** {code}

### Full Output
````
{complete go test -v output}
````

### Results Summary
**Total tests:** {N}
**Passed:** ✅ {N}
**Failed:** ❌ {N}
**Skipped:** ⏭️ {N}

### Individual Test Results
| Test Name | Status | Duration | 3agents Step | Notes |
|-----------|--------|----------|--------------|-------|
| Test{Name} | ✅ | {N}s | Step {N} | {notes} |
| Test{Name} | ❌ | {N}s | Step {N} | {error summary} |
| Test{Name} | ✅ | {N}s | Step {N} | {notes} |

### Screenshots Generated
````
test-results/{test-name}/
├── success.png
├── failure.png
└── ...
````

---

## API Tests (/test/api)

**Command:** `cd /test/api && go test -v -timeout 60s`
**Duration:** {MM:SS}
**Exit Code:** {code}

### Full Output
````
{complete go test -v output}
````

### Results Summary
**Total tests:** {N}
**Passed:** ✅ {N}
**Failed:** ❌ {N}
**Skipped:** ⏭️ {N}

### Individual Test Results
| Test Name | Status | Duration | 3agents Step | Notes |
|-----------|--------|----------|--------------|-------|
| Test{Name} | ✅ | {N}s | Step {N} | {notes} |
| Test{Name} | ❌ | {N}s | Step {N} | {error summary} |

---

## Overall Test Status

**Combined Results:**
- Total tests executed: {N}
- Pass rate: {XX.X}%
- Status: **{PASS ✅ | FAIL ❌ | PARTIAL ⚠️}**

**Execution completed:** {ISO8601}
````

---

## PHASE 4: FAILURE ANALYSIS & 3AGENTS FEEDBACK

### 4.1 Analyze Each Failure
**For EACH failed test, create detailed analysis:**
````markdown
## Failure Analysis

### Test{Name} - ❌ FAILED
**Test file:** `/test/{ui|api}/{file}.go`
**Testing 3agents Step:** {N} - {description}
**Duration:** {time}

**Error Output:**
````
{exact error message from test}
````

**Root Cause Analysis:**
{Detailed analysis of why test failed}

**Related Implementation:**
- **3agents Step:** {N}
- **Files changed:** {list from progress.md}
- **Skill used:** @{skill}
- **Hypothesis:** {why implementation might have caused this}

**Test Validity:**
- ✅ Test is correct - implementation has bug
- ❌ Test is incorrect - needs update
- ⚠️ Unclear - needs investigation

**Screenshots/Artifacts:**
{Reference to failure artifacts}

**Impact Assessment:**
- **Severity:** Critical | High | Medium | Low
- **Affects:** {what functionality is broken}
- **User impact:** {how would users be affected}

---
````

### 4.2 Generate 3agents Feedback
**IF tests failed, create `3agents-feedback.md`:**
````markdown
# Feedback for 3agents: Test Failures Found

**Testing task:** {task}
**Test date:** {ISO8601}
**3agents docs:** `docs/{task}/`
**Test results:** `docs/{task}/test-results/`

---

## Executive Summary

**Test Status:** ❌ FAILED - Action Required
**Failed tests:** {N}
**Affected 3agents steps:** {list step numbers}

---

## Issues Found

### Issue 1: {Description}
**Affects 3agents Step:** {N} - {description}
**Severity:** Critical | High | Medium | Low

**Test that failed:** `Test{Name}` in `/test/{ui|api}/{file}.go`

**Problem:**
{Clear description of what's wrong}

**Evidence:**
````
{relevant error output}
````

**Root cause:**
{Analysis of what in the implementation needs fixing}

**Suggested fix:**
{Specific actionable fix for implementer}

**Files to review:**
- `{file}` - {what to check/fix}
- `{file}` - {what to check/fix}

---

### Issue 2: {Description}
{Repeat structure for each issue}

---

## Recommended Next Steps for 3agents

### Option 1: Create Fix Plan (Recommended)
Resume 3agents with new plan to fix test failures:
````
Run 3agents with: "Fix test failures from docs/{task}/test-results/3agents-feedback.md"