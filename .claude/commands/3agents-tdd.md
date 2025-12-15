---
name: test-iterate
description: TDD enforcement - tests are IMMUTABLE requirements, fix code until tests pass
---

Execute: $ARGUMENTS

## INPUT VALIDATION

**GATE: Must provide a Go test file**

```
IF $ARGUMENTS does not end with "_test.go":
  STOP with error: "ERROR: Must provide a Go test file (*_test.go)"
  Example: /test-iterate test/ui/job_definition_test.go
```

## CONFIG
```yaml
max_iterations: 5
architecture_docs: docs/architecture/
skills_docs: .claude/skills/
workdir: ./docs/test-fix/{YYYYMMDD}-{test-name}/
```

## FUNDAMENTAL PHILOSOPHY

```
┌─────────────────────────────────────────────────────────────────┐
│                    TESTS ARE IMMUTABLE LAW                       │
│                                                                  │
│ Tests define REQUIREMENTS. Tests define BEHAVIOR.               │
│ Tests are NEVER wrong. Code that fails tests IS wrong.          │
│                                                                  │
│ If you touch a test file, YOU HAVE FAILED.                       │
│ If you weaken an assertion, YOU HAVE FAILED.                     │
│ If you skip a test, YOU HAVE FAILED.                             │
│ If you change expected values, YOU HAVE FAILED.                  │
└─────────────────────────────────────────────────────────────────┘
```

## RULES (NON-NEGOTIABLE)

### Test File Protection
- **NEVER modify test files** - period, no exceptions
- **NEVER weaken assertions** - `assert.Equal` stays `assert.Equal`
- **NEVER change expected values** - if test expects `5`, code returns `5`
- **NEVER skip tests** - `t.Skip()` is a FAILURE
- **NEVER rename tests** - the test name is part of the requirement
- **NEVER delete tests** - deleting requirements is WRONG

### What Tests Tell You
- **Test expects X, code returns Y** → Code is WRONG, fix the code
- **Test calls function that doesn't exist** → Create the function
- **Test expects behavior A, code does B** → Change code to do A
- **Test assertion seems "wrong"** → Test is the REQUIREMENT, not the bug

### Code Changes Only
- Modify implementation files ONLY
- Create new implementation if needed
- Refactor implementation to match test expectations
- Fix bugs in implementation
- NEVER "fix" tests to match code

---

## PHASE 0: SETUP & UNDERSTANDING

### Step 0.1: Validate Input
```bash
test -f "$ARGUMENTS" || STOP "File not found: $ARGUMENTS"
echo "$ARGUMENTS" | grep -q "_test.go$" || STOP "Not a test file"
```

### Step 0.2: Create Workdir
```bash
TEST_NAME=$(basename "$ARGUMENTS" _test.go)
mkdir -p ./docs/test-fix/{YYYYMMDD}-{TEST_NAME}/
```

### Step 0.3: Load Reference Documents
```bash
# Architecture requirements
cat docs/architecture/manager_worker_architecture.md
cat docs/architecture/QUEUE_LOGGING.md
cat docs/architecture/QUEUE_UI.md
cat docs/architecture/QUEUE_SERVICES.md

# Skill patterns
cat .claude/skills/go/SKILL.md
cat .claude/skills/frontend/SKILL.md
```

### Step 0.4: Understand Test Intent (CRITICAL)
**Before ANY implementation, understand what the test DEMANDS:**

**WRITE `{workdir}/test-analysis.md`:**
```markdown
# Test Analysis
File: {test_file}
Date: {timestamp}

## Test Inventory
| Test Name | Purpose | Key Assertions | Status |
|-----------|---------|----------------|--------|
| TestXxx | {what it verifies} | {expected values/behaviors} | ? |

## Test Requirements Extracted
For each test, extract the REQUIREMENTS it defines:

### TestXxx
**Input:** {what the test provides}
**Expected Output:** {what the test asserts}
**Behavior Required:** {what code MUST do}

## Implementation Files Referenced
| File | Functions/Types Used | Exists? |
|------|---------------------|---------|
| `{path}` | {functions} | Y/N |

## Gaps Identified
| Test Expects | Current State | Fix Needed |
|--------------|---------------|------------|
| {expectation} | {reality} | {implementation} |
```

---

## PHASE 1: EXECUTE TEST

### Step 1.1: Run Test
```bash
go test -v -run "Test.*" {test_file_path} 2>&1
```

### Step 1.2: Capture Results (COMPLETE OUTPUT)
**WRITE `{workdir}/test-run-{iteration}.md`:**
```markdown
# Test Run {N}
File: {test_file}
Date: {timestamp}
Iteration: {N}

## Result: PASS | FAIL

## Complete Test Output
```
{FULL test output - do not truncate}
```

## Test Results
| Test | Status | Error (if failed) |
|------|--------|-------------------|
| TestXxx | PASS/FAIL | {exact error message} |

## Failure Analysis (if any)
| Test | Expected | Got | Root Cause |
|------|----------|-----|------------|
| TestXxx | {expected value} | {actual value} | {why} |
```

### If ALL PASS:
Go to PHASE 4: COMPLETE

### If ANY FAIL:
Continue to PHASE 2

---

## PHASE 2: ANALYZE & FIX (DEVIL'S ADVOCATE)

### Step 2.1: Devil's Advocate Analysis
**Assume your instinct to "fix the test" is WRONG. The test is RIGHT.**

For each failing test:
1. **Read the test assertion character by character**
2. **Extract the EXACT expected value**
3. **Understand WHY the test expects this** (it's a requirement)
4. **Find where the code produces the WRONG value**
5. **Fix the CODE to produce the RIGHT value**

### Step 2.2: Implementation Analysis
Before fixing, understand:
- What function/method is being tested?
- What is the current implementation doing?
- Why does it produce the wrong result?
- What change makes it produce the right result?

### Step 2.3: Architecture & Pattern Compliance
Verify the fix will comply with:
- `docs/architecture/*.md` - Architecture patterns
- `.claude/skills/go/SKILL.md` - Go patterns
- Existing codebase patterns - Consistency

### Step 2.4: Implement Fix (CODE ONLY)
**WRITE `{workdir}/fix-{iteration}.md`:**
```markdown
# Fix {N}
Iteration: {N}

## Test File Status
**UNCHANGED** - Tests are requirements (if changed, this fix is INVALID)

## Failures Being Fixed
| Test | Assertion | Expected | Currently Returning | Fix |
|------|-----------|----------|---------------------|-----|
| TestXxx | {assert line} | {expected} | {actual} | {code change} |

## Root Cause Analysis
| Test | Why Code Is Wrong | Correct Behavior |
|------|-------------------|------------------|
| TestXxx | {what code does wrong} | {what code should do} |

## Implementation Changes
| File | Function/Method | Before | After |
|------|-----------------|--------|-------|
| `{path}` | {func} | {old behavior} | {new behavior} |

## Pattern Compliance Check
| Pattern | Source | Followed? | Evidence |
|---------|--------|-----------|----------|
| Error wrapping | .claude/skills/go/SKILL.md | Y/N | `{code}` |
| Arbor logging | .claude/skills/go/SKILL.md | Y/N | `{code}` |
| DI pattern | .claude/skills/go/SKILL.md | Y/N | `{code}` |

## Files Modified
| File | Action | Lines Changed |
|------|--------|---------------|
| `{path}` | modified | +X/-Y |

## Files NOT Modified (VERIFY)
- `{test_file}` ✓ NOT TOUCHED
- All other *_test.go files ✓ NOT TOUCHED

## Self-Check Questions
1. Did I touch any test file? **MUST BE NO**
2. Did I weaken any assertion? **MUST BE NO**
3. Does my fix make the test pass for the RIGHT reason? **MUST BE YES**
4. Am I making the code do what the test expects? **MUST BE YES**
```

---

## PHASE 3: VALIDATE & ITERATE

### Step 3.1: Pre-Validation Audit

**CRITICAL CHECK: Were any test files modified?**
```bash
git diff --name-only | grep "_test.go"
# If ANY output → FAILURE - revert test changes and try again
```

### Step 3.2: Re-run Tests
Return to PHASE 1: EXECUTE TEST

### Step 3.3: Check Iteration Count
```
IF iteration > max_iterations (5):
  STOP with:
  - Remaining failing tests
  - What each test expects
  - Why code cannot currently provide it
  - Specific user guidance needed
```

### Step 3.4: Regression Check
If a previously passing test now fails:
- The "fix" broke something else
- Revert and find a different approach
- The test requirements are consistent - find a solution that satisfies ALL

---

## PHASE 4: COMPLETE

**WRITE `{workdir}/summary.md`:**
```markdown
# Test Fix Complete
File: {test_file}
Iterations: {N}

## Result: ALL TESTS PASS

## Test File Integrity
- Test file modified: **NO** ✓
- Assertions weakened: **NO** ✓
- Tests skipped: **NO** ✓
- Expected values changed: **NO** ✓

## Implementation Fixes Applied
| Iteration | Files Changed | Tests Fixed |
|-----------|---------------|-------------|
| 1 | {files} | {tests} |
| 2 | {files} | {tests} |

## Architecture Compliance
All fixes comply with:
- docs/architecture/ requirements
- .claude/skills/ patterns
- Existing codebase patterns

## Final Test Output
```
{passing test output - all green}
```
```

---

## WORKFLOW DIAGRAM
```
┌─────────────────────────────────────────────────────────────────┐
│ INPUT VALIDATION                                                 │
│ - Must be *_test.go file                                         │
│ - STOP if not a test file                                        │
└─────────────────┬───────────────────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 0: SETUP & UNDERSTANDING                                   │
│ - Create workdir                                                 │
│ - Load docs/architecture/*.md + .claude/skills/*.md              │
│ - UNDERSTAND what each test REQUIRES                             │
│ - Write test-analysis.md                                         │
└─────────────────┬───────────────────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 1: EXECUTE TEST                                            │◄─────┐
│ - Run: go test -v {file}                                         │      │
│ - Capture COMPLETE output                                        │      │
│ - Write test-run-{N}.md                                          │      │
├─────────────────────────────────────────────────────────────────┤      │
│ ALL PASS → PHASE 4                                               │      │
│ ANY FAIL → PHASE 2                                               │      │
└─────────────────┬───────────────────────────────────────────────┘      │
                  ▼                                                      │
┌─────────────────────────────────────────────────────────────────┐      │
│ PHASE 2: ANALYZE & FIX (DEVIL'S ADVOCATE)                        │      │
│ - Test is RIGHT, code is WRONG                                   │      │
│ - Extract EXACT expected values from test                        │      │
│ - Find why code produces WRONG values                            │      │
│ - Fix CODE to produce RIGHT values                               │      │
│ - NEVER touch test files                                         │      │
│ - Write fix-{N}.md                                               │      │
└─────────────────┬───────────────────────────────────────────────┘      │
                  ▼                                                      │
┌─────────────────────────────────────────────────────────────────┐      │
│ PHASE 3: VALIDATE & ITERATE                                      │      │
│ - VERIFY no test files were modified (git diff)                  │      │
│ - Check iteration count                                          │      │
│ - If < 5: return to PHASE 1 ────────────────────────────────────┼──────┘
│ - If >= 5: STOP with remaining failures + user guidance          │
└─────────────────┬───────────────────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 4: COMPLETE                                                │
│ - Verify test file integrity                                     │
│ - Write summary.md                                               │
│ - ALL tests pass                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## CRITICAL ENFORCEMENT RULES

### Tests Are Immutable
```
┌─────────────────────────────────────────────────────────────────┐
│ IF AT ANY POINT YOU CONSIDER MODIFYING A TEST FILE:             │
│                                                                  │
│ 1. STOP immediately                                              │
│ 2. The test is the REQUIREMENT                                   │
│ 3. Your code is WRONG, not the test                              │
│ 4. Find a different approach to fix the CODE                     │
│                                                                  │
│ Common wrong thoughts to override:                               │
│ - "This test expectation doesn't make sense" → It's the spec    │
│ - "The test is checking the wrong thing" → It's the requirement │
│ - "If I just change this one value..." → NO. Fix the code.      │
│ - "The test was written incorrectly" → Code to match the test.  │
└─────────────────────────────────────────────────────────────────┘
```

### Forbidden Actions (AUTO-FAIL)
1. **Modifying any `*_test.go` file** - FAILURE
2. **Adding `t.Skip()`** - FAILURE
3. **Changing assertion expected values** - FAILURE
4. **Weakening assertions** (e.g., `assert.Equal` → `assert.Contains`) - FAILURE
5. **Deleting test cases** - FAILURE
6. **Renaming tests** - FAILURE
7. **Commenting out assertions** - FAILURE

### Allowed Actions
1. Modify implementation files
2. Create new implementation files (if tests reference them)
3. Add new functions/methods (if tests call them)
4. Fix bugs in existing implementation
5. Refactor implementation (without changing behavior tests verify)

### Devil's Advocate Mindset
- When a test fails, ask: "What is the test telling me the code should do?"
- Never ask: "What is wrong with this test?"
- The test is always right. The code is always suspect.

---

## INVOKE
```
/test-iterate test/ui/job_definition_codebase_classify_test.go
/test-iterate test/api/queue_test.go
```

