---
name: testfix
description: Two-agent workflow - run test, fix failures, iterate until passing. Test-driven code fixing.
---

Fix failing tests using test file: $ARGUMENTS

## INPUT HANDLING

**$ARGUMENTS must be a test file path** (e.g., `test/api/config_test.go` or `test/ui/settings_test.js`)

1. **Parse the test file path** to extract:
   - Test file location
   - Test type (go test, npm test, etc.)
   - Package/module being tested

2. **Determine working folder**:
   - Extract filename without extension
   - Create folder: `docs/testfix/{timestamp}-{test-name}/`
   - Example: `docs/testfix/20250514-120000-config-test/`

3. **Initial test run** to capture baseline failures

**Output Location:** All markdown files (iteration-*.md, progress.md, summary.md) go into the working folder.

## RULES
**Tests:** Run tests exactly as specified in test file
**Binaries:** Never in root - use `go build -o /tmp/` or `go run`
**Beta mode:** Breaking changes allowed to fix tests
**Complete:** Run all iterations until tests pass or max retries reached
**Max Iterations:** 5 attempts to fix failing tests

## CONFIG
```yaml
limits:
  max_iterations: 5  # Total attempts to fix the code

agents:
  implementer: claude-sonnet-4-20250514
  reviewer: claude-sonnet-4-20250514

test_types:
  go: "cd {dir} && go test -v"
  javascript: "npm test {file}"
  python: "pytest {file} -v"
```

## SETUP

1. **Read the test file** from $ARGUMENTS
2. **Analyze test structure:**
   - What is being tested (functions, handlers, modules)
   - Test descriptions and expectations
   - Required imports/dependencies

3. **Identify source files:**
   - Parse test file for imported packages
   - Locate corresponding source files
   - Map test functions to source functions

4. **Create working folder:**
   - Format: `docs/testfix/{timestamp}-{test-name}/`
   - Example: `docs/testfix/20250514-120000-config-test/`

5. **Run initial test:**
   - Capture full output
   - Identify all failing tests
   - Document in `baseline.md`

---

## BASELINE ASSESSMENT

`baseline.md`:
```markdown
# Baseline Test Results

**Test File:** {path}
**Test Command:** {command used}
**Timestamp:** {ISO8601}

## Test Output
```
{full test output}
```

## Failures Identified
1. **Test:** {test name}
   - **Error:** {error message}
   - **Expected:** {expected behavior}
   - **Actual:** {actual behavior}
   - **Source:** {likely source file/function}

2. **Test:** {test name}
   - **Error:** {error message}
   - **Expected:** {expected behavior}
   - **Actual:** {actual behavior}
   - **Source:** {likely source file/function}

## Source Files to Fix
- `{file}` - {why this needs fixing}
- `{file}` - {why this needs fixing}

## Dependencies
- {list any missing or broken dependencies}

## Test Statistics
- **Total Tests:** {N}
- **Passing:** {N}
- **Failing:** {N}
- **Skipped:** {N}

**→ Starting Iteration 1**
```

---

## AGENT 1 & 2 - FIX & TEST LOOP

**For each iteration (max 5), create:** `iteration-{N}.md`

### Iteration File Format: `iteration-{N}.md`

```markdown
# Iteration {N}

**Goal:** Fix failing tests from {previous iteration or baseline}

---

## Agent 1 - Implementation

### Failures to Address
{List of specific test failures being fixed in this iteration}

### Analysis
{Why tests are failing - root cause analysis}

### Proposed Fixes
**File: `{path}`**
- {what will be changed}
- {reasoning for the change}

**File: `{path}`**
- {what will be changed}
- {reasoning for the change}

### Changes Made

**`{file}`:**
```{language}
{code changes with context}
```
{explanation of changes}

**`{file}`:**
```{language}
{code changes with context}
```
{explanation of changes}

### Compilation Check
```bash
{compile command}
```
**Result:** ✅ Compiles | ❌ Compilation error

---

## Agent 2 - Review & Test

### Test Execution
**Command:**
```bash
{test command}
```

**Output:**
```
{full test output}
```

### Test Results
- **Total Tests:** {N}
- **Passing:** {N} (+{delta} from previous)
- **Failing:** {N} (-{delta} from previous)
- **New Failures:** {N} (if any)
- **Fixed:** {N}

### Analysis

**Tests Fixed:**
- ✅ {test name} - {what was fixed}
- ✅ {test name} - {what was fixed}

**Tests Still Failing:**
- ❌ {test name} - {error message}
  - **Root Cause:** {analysis}
  - **Recommended Fix:** {suggestion}

**New Failures Introduced:**
- ⚠️ {test name} - {error message}
  - **Caused By:** {which change caused this}

### Code Quality Review
**Changes Assessment:**
- ✅ Follows existing patterns
- ✅ Proper error handling
- ✅ No breaking changes to other code
- ⚠️ {any concerns}

**Quality Score:** {X}/10

### Decision
- **ALL TESTS PASS** → ✅ SUCCESS - Stop iterating
- **SOME PROGRESS** → 🔄 CONTINUE to Iteration {N+1}
- **NO PROGRESS / WORSE** → ⚠️ NEEDS RETHINK
- **MAX ITERATIONS REACHED** → ⏹️ STOP WITH ISSUES

**Next Action:** {decision}

---

## Iteration Summary

**Status:** ✅ Success | 🔄 Continue | ⚠️ Issues | ⏹️ Max Retries

**Progress:**
- Tests Fixed: {N}
- Tests Remaining: {N}
- Quality: {score}/10

{If continuing: **→ Continuing to Iteration {N+1}**}
{If done: **→ All tests passing - Creating summary**}
{If stopped: **→ Max iterations reached - Creating summary with issues**}
```

---

## AGENT 1 - IMPLEMENTER RULES

**For each iteration:**

1. **Analyze test failures:**
   - Read test output from previous iteration (or baseline)
   - Identify root causes
   - Determine which source files need changes

2. **Plan the fixes:**
   - What needs to change in each file
   - Why each change will fix the test
   - Consider side effects

3. **Implement changes:**
   - Make targeted fixes to source code
   - Follow existing code patterns
   - Add missing functionality if needed
   - Fix logic errors
   - Handle edge cases

4. **Verify compilation:**
   - Build/compile the code
   - Fix any syntax errors
   - Document compilation status

5. **Document in iteration-{N}.md:**
   - Failures being addressed
   - Analysis of root cause
   - Files modified and why
   - Code changes made
   - Compilation results

6. **Wait for Agent 2 to run tests**

---

## AGENT 2 - REVIEWER RULES

**For each iteration:**

1. **Run the test file:**
   - Use appropriate test command for file type
   - Capture full output
   - Count pass/fail/skip

2. **Analyze results:**
   - Which tests now pass (were fixed)
   - Which tests still fail (and why)
   - Any new failures introduced
   - Overall progress vs previous iteration

3. **Review code quality:**
   - Are fixes proper and maintainable?
   - Do they follow existing patterns?
   - Any potential side effects?
   - Quality score 1-10

4. **Document in iteration-{N}.md:**
   - Test command and full output
   - Test statistics (pass/fail counts)
   - Analysis of what was fixed
   - Analysis of remaining failures
   - Code quality review
   - Quality score

5. **Make decision:**
   - **ALL TESTS PASS** → ✅ SUCCESS - Create summary and stop
   - **PROGRESS MADE** → 🔄 CONTINUE to next iteration
   - **NO PROGRESS** → ⚠️ Needs rethinking - but continue one more try
   - **MAX ITERATIONS** → ⏹️ STOP - Create summary with remaining issues

6. **Update progress.md** after each iteration

---

## PROGRESS TRACKING

**Update after each iteration:** `progress.md`

```markdown
# Test Fix Progress

## Test Information
**Test File:** {path}
**Test Command:** {command}
**Started:** {ISO8601}

## Baseline
- **Total Tests:** {N}
- **Failing:** {N}
- **Passing:** {N}

## Iterations

### Iteration 1
- **Tests Fixed:** {N}
- **Tests Failing:** {N}
- **New Failures:** {N}
- **Quality:** {X}/10
- **Status:** 🔄 Continue

### Iteration 2
- **Tests Fixed:** {N}
- **Tests Failing:** {N}
- **New Failures:** {N}
- **Quality:** {X}/10
- **Status:** 🔄 Continue

### Iteration 3
- **Tests Fixed:** {N}
- **Tests Failing:** {N}
- **New Failures:** {N}
- **Quality:** {X}/10
- **Status:** ✅ Success | 🔄 Continue | ⏹️ Stopped

## Current Status
- **Tests Passing:** {N}/{total} ({percent}%)
- **Average Quality:** {avg}/10
- **Total Iterations:** {N}

**Last Updated:** {ISO8601}
```

---

## WORKFLOW

```
# Parse test file from $ARGUMENTS
Read test file
Analyze structure and determine test type
Identify source files to fix

# Create working folder
Format: docs/testfix/{timestamp}-{test-name}/
Create working folder

# Run baseline test
Execute test command
Capture output
Analyze failures
Document in baseline.md
Create initial progress.md

# Initialize iteration counter
iteration = 1
max_iterations = 5

WHILE tests_failing AND iteration <= max_iterations:
  
  Create iteration-{N}.md
  
  # Agent 1 - Implement Fixes
  Analyze test failures from previous iteration
  Identify root causes
  Plan fixes for source files
  Implement changes
  Verify compilation
  Document in iteration-{N}.md "Agent 1 - Implementation"
  
  # Agent 2 - Review & Test
  Run test file
  Capture full output
  Count pass/fail/new failures
  Analyze progress
  Review code quality
  Document in iteration-{N}.md "Agent 2 - Review & Test"
  
  IF all tests pass:
    Mark SUCCESS
    Update progress.md
    Break loop
  
  ELSIF some progress made:
    Update progress.md
    iteration++
    Continue loop
  
  ELSIF no progress:
    Mark NEEDS_RETHINK
    Update progress.md
    iteration++
    Continue loop (one more try)
  
  ELSIF iteration == max_iterations:
    Mark MAX_RETRIES_REACHED
    Update progress.md
    Break loop

END WHILE

# Create summary
Generate summary.md
Report final status
```

---

## COMPLETION

`summary.md`:
```markdown
# Test Fix Summary: {test file name}

## Overview
**Test File:** {path}
**Test Command:** {command}
**Duration:** {start time} to {end time}
**Total Iterations:** {N}

## Final Results
- **Total Tests:** {N}
- **Passing:** {N} ({percent}%)
- **Failing:** {N} ({percent}%)
- **Fixed:** {N} tests

## Status
✅ **ALL TESTS PASSING** | ⚠️ **PARTIAL SUCCESS** | ❌ **TESTS STILL FAILING**

## Baseline vs Final
| Metric | Baseline | Final | Delta |
|--------|----------|-------|-------|
| Passing | {N} | {N} | +{N} |
| Failing | {N} | {N} | -{N} |
| Success Rate | {N}% | {N}% | +{N}% |

## Files Modified
{List all source files that were changed}

**`{file}`:**
- {summary of changes}
- {why these changes were needed}

**`{file}`:**
- {summary of changes}
- {why these changes were needed}

## Iteration Summary
| Iteration | Tests Fixed | Tests Failing | Quality | Status |
|-----------|-------------|---------------|---------|--------|
| Baseline | - | {N} | - | - |
| 1 | {N} | {N} | {X}/10 | 🔄 |
| 2 | {N} | {N} | {X}/10 | 🔄 |
| 3 | {N} | {N} | {X}/10 | ✅ |

## Tests Fixed
{List each test that was fixed}

### {Test Name}
- **Original Error:** {error message}
- **Fixed In:** Iteration {N}
- **Solution:** {brief description of fix}

## Remaining Issues
{If any tests still failing}

### {Test Name}
- **Error:** {error message}
- **Root Cause:** {analysis}
- **Attempted Fixes:** {what was tried}
- **Recommendation:** {what might work}

## Code Quality
**Average Quality Score:** {avg}/10

**Patterns Followed:**
- ✅ Matches existing code style
- ✅ Proper error handling
- ✅ No breaking changes

**Concerns:**
- {any code quality issues to note}

## Recommended Next Steps
{Based on final status}

**If all passing:**
1. Review changes in git diff
2. Run full test suite to ensure no regressions
3. Commit changes

**If partial success:**
1. Review remaining test failures (documented above)
2. Consider manual investigation of root causes
3. Run full test suite before committing

**If tests still failing:**
1. Review iteration-*.md files for attempted solutions
2. Consider architectural changes may be needed
3. Consult with team on approach

## Test Output (Final Iteration)
```
{final test run output}
```

## Documentation
All iteration details available in working folder:
- `baseline.md` (initial test run)
- `iteration-{1..N}.md` (each fix attempt)
- `progress.md` (ongoing status)

**Completed:** {ISO8601}
```

---

## STOP CONDITIONS

**ONLY stop workflow for:**
- ✅ All tests passing (SUCCESS)
- ⏹️ Maximum iterations reached (5)
- ❌ Cannot parse test file
- ❌ Test file not found
- ❌ Cannot determine test type

**NEVER stop for:**
- ❌ Asking "would you like me to continue?"
- ❌ Asking "shall I try another iteration?"
- ❌ Test failures (document and iterate)
- ❌ Compilation errors (fix and iterate)
- ❌ "Let me know if you want me to continue"

**Golden Rule:** 
Tests are failing, so FIX THEM. Iterate automatically until they pass or you hit max iterations. Don't ask, just fix.

---

## ANTI-PATTERNS TO AVOID

**❌ DON'T:**
```
Iteration 1 complete. Some tests still failing. Would you like me to continue?
```

**✅ DO:**
```markdown
# Iteration 1
[... implementation and review ...]
Status: 🔄 Continue (fixed 3/7 tests)

→ Continuing to Iteration 2
```

**❌ DON'T:**
```
The test is failing with error X. Should I try fixing file Y?
```

**✅ DO:**
```markdown
### Agent 1 - Implementation
Test failing with error X. Root cause: missing validation in file Y.
Implementing fix in Y to add proper validation.
```

**❌ DON'T:**
```
I've fixed some issues but tests are still failing. What should I do?
```

**✅ DO:**
```markdown
### Agent 2 - Review & Test
Fixed 3 tests, 4 still failing.
Analysis shows remaining failures are due to Z.
Quality: 7/10
Decision: 🔄 CONTINUE to Iteration 2
```

---

## TEST TYPE DETECTION

**Automatically detect test type:**

1. **Go Tests:**
   - Pattern: `*_test.go`
   - Command: `cd {dir} && go test -v`
   - Output parsing: Look for `PASS` / `FAIL` / `ok` / `FAIL:`

2. **JavaScript/TypeScript:**
   - Pattern: `*.test.js`, `*.test.ts`, `*.spec.js`
   - Command: `npm test {file}` or `jest {file}`
   - Output parsing: Look for test results summary

3. **Python:**
   - Pattern: `test_*.py`, `*_test.py`
   - Command: `pytest {file} -v`
   - Output parsing: Look for `passed` / `failed` / `error`

4. **Other:**
   - Ask user for test command if cannot detect
   - Document custom command in baseline.md

---

## SOURCE FILE DISCOVERY

**For Go tests:**
```go
// Test file: config_test.go
package config

import (
    "testing"
    "github.com/user/app/pkg/config"  // <- Source package
)

func TestLoadConfig(t *testing.T) {  // <- Testing LoadConfig function
    // Source file likely: config.go or load.go in same package
}
```

**Strategy:**
1. Parse import statements
2. Identify package being tested
3. Look for source files in same directory
4. Map test function names to source functions
5. Include all relevant source files in fix scope

---

## ERROR ANALYSIS PATTERNS

**Common failure patterns and fixes:**

1. **Nil pointer / missing initialization:**
   - Add proper initialization before use
   - Check for nil before dereferencing

2. **Type mismatch:**
   - Convert types properly
   - Update function signatures if needed

3. **Logic error:**
   - Review algorithm
   - Fix conditional logic
   - Handle edge cases

4. **Missing functionality:**
   - Implement missing functions/methods
   - Add required fields to structs

5. **Incorrect mock/stub:**
   - Update test mocks to match implementation
   - Fix test expectations

---

**Task:** Fix failing tests in $ARGUMENTS
**Mode:** Iterate automatically until all tests pass or max 5 iterations reached
**Working Folder:** `docs/testfix/{timestamp}-{test-name}/`
**No asking permission:** Just fix the code iteratively