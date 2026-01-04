---
name: 3agents-tdd
description: TDD enforcement - tests are IMMUTABLE, fix code until tests pass. Sequential execution with full restart on fix. Output captured to files to prevent context overflow.
---

Execute: $ARGUMENTS

**Read first:** `.claude/skills/refactoring/SKILL.md`

## INPUT VALIDATION
````
1. Normalize path: replace \ with /
2. Must be *_test.go file or STOP
````

## SETUP (MANDATORY - DO FIRST)

**Create workdir BEFORE any other action:**
````bash
TEST_FILE="$ARGUMENTS"                          # e.g., test/ui/job_definition_test.go
TEST_FILE="${TEST_FILE//\\//}"                  # normalize: replace \ with /
TASK_SLUG=$(basename "$TEST_FILE" "_test.go")   # e.g., "job_definition"
DATE=$(date +%Y-%m-%d)
TIME=$(date +%H%M)
WORKDIR=".claude/workdir/${DATE}-${TIME}-tdd-${TASK_SLUG}"
mkdir -p "$WORKDIR"
echo "Created workdir: $WORKDIR"
````

**STOP if workdir creation fails.**

## FUNDAMENTAL RULES
````
┌─────────────────────────────────────────────────────────────────┐
│ TESTS ARE IMMUTABLE LAW                                         │
│                                                                  │
│ • Touch a test file = FAILED                                    │
│ • Weaken an assertion = FAILED                                  │
│ • Skip/delete a test = FAILED                                   │
│                                                                  │
│ Test expects X, code returns Y → FIX THE CODE                   │
│                                                                  │
│ Exception: If test expects DEPRECATED/OLD behavior,             │
│ document it as MISALIGNED and suggest TEST should change        │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ BACKWARD COMPATIBILITY IS NOT REQUIRED                          │
│                                                                  │
│ • New code defines the CORRECT behavior                         │
│ • Old/deprecated behavior should NOT be preserved               │
│ • NEVER add backward compatibility shims                        │
│ • NEVER keep old APIs/types/functions for compatibility         │
│ • If test expects deprecated behavior → TEST IS WRONG           │
│                                                                  │
│ If a test expects old behavior, the TEST needs updating,        │
│ not the code. Document this as a misaligned test.               │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ ARTIFACTS ARE MANDATORY                                         │
│                                                                  │
│ • $WORKDIR/tdd_state.md - MUST create in Phase 2                │
│ • $WORKDIR/test_issues.md - MUST create if tests misaligned     │
│ • $WORKDIR/summary.md - MUST create in Phase 4 (ALWAYS)         │
│                                                                  │
│ Task is NOT complete without summary.md in workdir.             │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ OUTPUT CAPTURE IS MANDATORY                                     │
│                                                                  │
│ • ALL test output → $WORKDIR/test_*.log files                   │
│ • Claude sees ONLY pass/fail + last 30 lines on failure         │
│ • NEVER let full test output into context                       │
│ • Reference log files by path, don't paste contents             │
│                                                                  │
│ This prevents context overflow during long-running tests.       │
└─────────────────────────────────────────────────────────────────┘
````

## CONTEXT MANAGEMENT

**TDD iterations accumulate context fast. Compact aggressively.**

### Compaction Rules
- **MANDATORY compaction points** marked with `⟲ COMPACT`
- Run `/compact` at each marked point - do NOT skip
- If `/compact` fails: press Escape twice, move up, retry
- If still failing: `/clear` and restart with test file path

### Recovery Protocol
If context is lost mid-iteration:
1. Read `$WORKDIR/tdd_state.md` for current state
2. Re-read the test file to extract requirements
3. Resume PHASE 3 loop from recorded iteration

### Output Limits (CRITICAL)
| Output Type | Max Lines in Context | Action |
|-------------|---------------------|--------|
| Test stdout/stderr | 0 (captured to file) | Always redirect to $WORKDIR/*.log |
| Error summary | 30 | Use `tail -30` on failure |
| Stack traces | 20 | Extract key frames only |
| File reads | 500 | Use grep/head/tail for large files |
| Build output | 50 | Capture full to file, show summary |

## WORKFLOW

### PHASE 0: RESET CONTEXT

---
### ⟲ COMPACT POINT: START

**Run `/compact` before starting.** Clear context for maximum iteration headroom.

---

### PHASE 1: SETUP & UNDERSTAND

**Step 1.1: Create workdir (MANDATORY)**
````bash
mkdir -p "$WORKDIR"
mkdir -p "$WORKDIR/logs"
````
Verify directory exists before continuing.

**Step 1.2: Read test file**
- Extract ALL test function names in order

**Step 1.3: Read skills**
- `.claude/skills/refactoring/SKILL.md` - Core patterns
- `.claude/skills/go/SKILL.md` - Go changes
- `.claude/skills/frontend/SKILL.md` - Frontend changes
- `.claude/skills/monitoring/SKILL.md` - UI tests

**Step 1.4: Read test architecture**
- `docs/TEST_ARCHITECTURE.md`

**Step 1.5: For UI job tests**
- Validate against template: `test/ui/job_definition_general_test.go`

### PHASE 2: BUILD TEST LIST
````bash
# Extract ALL test names from file IN ORDER
TEST_LIST=$(grep "^func Test" "$TEST_FILE" | sed 's/func \(Test[^(]*\).*/\1/')
TEST_PKG=$(dirname "$TEST_FILE")

# Store as ordered array
TESTS=($TEST_LIST)
echo "Found ${#TESTS[@]} tests to run sequentially"
````

**MUST write `$WORKDIR/tdd_state.md`:**
````markdown
# TDD State

## Test File
`{test_file}`

## Test Package
`{test_pkg}`

## Workdir
`{workdir}`

## Tests (in order)
1. TestFirst
2. TestSecond
3. TestThird
...

## Current State
- Iteration: 0
- Last failed test: N/A
- Status: STARTING

## Log Files
- Test logs: $WORKDIR/logs/test_*.log
- Build logs: $WORKDIR/logs/build_*.log
````

### PHASE 3: TEST LOOP (max 3 iterations)

**SINGLE PROCESS EXECUTION**
Run ALL tests from the file in ONE `go test` command. This ensures:
- All tests share one results directory
- Tests run sequentially (Go's default behavior)
- Tests "graduate" naturally - test 1 passes, test 2 runs, etc.

**OUTPUT CAPTURE (MANDATORY)**
All test output MUST go to file. Claude only sees pass/fail + brief error summary.

````
ITERATION = 0
     │
     ▼
┌─────────────────────────────────────────────────────────────────┐
│ RUN ALL TESTS (iteration $ITERATION)                            │
│                                                                 │
│   TEST_LOG="$WORKDIR/logs/test_iter${ITERATION}.log"            │
│                                                                 │
│   # Run ALL tests from file in SINGLE process                   │
│   # Tests run sequentially and share one results directory      │
│   go test -v -timeout 30m ./$TEST_PKG/... \                     │
│       > "$TEST_LOG" 2>&1                                        │
│   RESULT=$?                                                     │
│                                                                 │
│   if [ $RESULT -eq 0 ]; then                                    │
│       echo "✓ ALL TESTS PASSED"                                 │
│       → PHASE 4 (COMPLETE)                                      │
│   fi                                                            │
│                                                                 │
│   # FAIL - extract which test failed                            │
│   echo "✗ TESTS FAILED"                                         │
│   echo "Log: $TEST_LOG"                                         │
│   echo "=== Failed tests ==="                                   │
│   grep "^--- FAIL:" "$TEST_LOG"                                 │
│   echo "=== Last 30 lines ==="                                  │
│   tail -30 "$TEST_LOG"                                          │
│   echo "=== End of summary ==="                                 │
│                                                                 │
│   # Extract first failed test name for analysis                 │
│   FAILED_TEST=$(grep "^--- FAIL:" "$TEST_LOG" | head -1 | \     │
│       sed 's/--- FAIL: \([^ ]*\).*/\1/')                        │
└─────────────────────────────────────────────────────────────────┘
            │
         FAILURE at test N
            │
            ▼
┌─────────────────────────────────────────────────────────────────┐
│ ANALYZE FAILURE - FROM LOG FILE                                 │
│                                                                 │
│ Read error details from $WORKDIR/logs/test_iter${ITERATION}.log │
│                                                                 │
│   # List all failed tests                                       │
│   grep "^--- FAIL:" "$TEST_LOG"                                 │
│                                                                 │
│   # Extract assertion failure (grep for key patterns)           │
│   grep -A5 "FAIL\|Error\|assert\|expected\|got:" "$TEST_LOG" \  │
│       | head -20                                                │
│                                                                 │
│ DO NOT paste entire log into context!                           │
│ Extract only:                                                   │
│   • Test name(s) that failed                                    │
│   • Assertion that failed (expected vs actual)                  │
│   • File:line of failure                                        │
│                                                                 │
│ ASK: Is the test expecting CURRENT or DEPRECATED behavior?      │
│                                                                 │
│ TEST EXPECTS CURRENT BEHAVIOR                                   │
│ → Code has a bug → FIX THE CODE                                 │
│                                                                 │
│ TEST EXPECTS DEPRECATED/OLD BEHAVIOR                            │
│ → TEST IS MISALIGNED → DOCUMENT (do not add compat!)            │
└───────────┬─────────────────────────────────────────────────────┘
            │
            ▼
     ┌──────┴──────┐
     │             │
  CODE BUG    TEST MISALIGNED
     │             │
     ▼             ▼
┌─────────────┐ ┌─────────────────────────────────────────────────┐
│ FIX CODE    │ │ DOCUMENT MISALIGNED TEST                        │
│             │ │                                                 │
│ • EXTEND >  │ │ MUST write to: $WORKDIR/test_issues.md          │
│   MODIFY >  │ │                                                 │
│   CREATE    │ │ • Why test is wrong (expects deprecated)        │
│ • Build     │ │ • What test SHOULD expect (new behavior)        │
│   must pass │ │ • Suggested test change                         │
│             │ │                                                 │
│ Build check:│ │ Then: SKIP this test, continue with next        │
│ go build    │ │                                                 │
│ ./... 2>&1  │ │ DO NOT add backward compatibility!              │
│ | tail -20  │ │                                                 │
└─────────────┘ └─────────────────────────────────────────────────┘
            │
            ▼
       ITERATION++
            │
            ▼
   UPDATE $WORKDIR/tdd_state.md
            │
            ▼
    ┌───────┴───────┐
    │               │
ITERATION < 3    ITERATION = 3
    │               │
    ▼               ▼
 RESTART         PHASE 4
 from test 1     (COMPLETE)
````

**Build verification with output capture:**
````bash
BUILD_LOG="$WORKDIR/logs/build_iter${ITERATION}.log"
go build ./... > "$BUILD_LOG" 2>&1
BUILD_RESULT=$?

if [ $BUILD_RESULT -ne 0 ]; then
    echo "✗ BUILD FAILED"
    echo "=== Last 20 lines ==="
    tail -20 "$BUILD_LOG"
else
    echo "✓ BUILD PASSED"
fi
````

**Error extraction helper (use instead of reading full log):**
````bash
# Extract failure info from test log
# Usage: extract_failure "$WORKDIR/logs/test_iter0.log"
extract_failure() {
    local LOG_FILE=$1
    echo "--- Failure Summary ---"

    # List all failed tests
    echo "Failed tests:"
    grep "^--- FAIL:" "$LOG_FILE"

    # Get first failure with context
    grep -B2 -A10 "^--- FAIL:" "$LOG_FILE" | head -15

    # Get assertion errors
    grep -A3 "Error:\|assert\|expected\|got:" "$LOG_FILE" | head -10

    echo "--- End Summary ---"
    echo "Full log: $LOG_FILE"
}
````

**MUST update `$WORKDIR/tdd_state.md` after each iteration:**
````markdown
## Current State
- Iteration: {n}
- Last failed test: {test_name}
- Status: IN_PROGRESS
- Log file: $WORKDIR/logs/test_iter{n}.log

## Iteration History
### Iteration 1
- Failed at: TestSecond
- Log: $WORKDIR/logs/test_iter1.log
- Error summary: <2-3 line description of failure>
- Action: CODE_FIX / TEST_MISALIGNED
- Details: <what was changed or documented>
````

---
### ⟲ COMPACT POINT: AFTER EACH ITERATION

**Run `/compact` after completing each iteration fix.**

Include in compact summary:
- Current iteration number
- Tests passed so far
- Current failure being addressed
- Log file locations

Recovery context:
- Read: `$WORKDIR/tdd_state.md`
- Misaligned tests: Check `$WORKDIR/test_issues.md`
- Last error: `tail -30 $WORKDIR/logs/test_iter*.log | tail -1` (most recent)

---

### PHASE 4: COMPLETE (MANDATORY)

**This phase MUST execute. Task is incomplete without it.**

**Step 4.1: Verify final state**
````bash
# Run all tests one final time with output capture
FINAL_LOG="$WORKDIR/logs/final_run.log"
go test -v -timeout 30m ./$TEST_PKG/... > "$FINAL_LOG" 2>&1
FINAL_RESULT=$?

if [ $FINAL_RESULT -eq 0 ]; then
    echo "✓ ALL TESTS PASSED"
else
    echo "✗ SOME TESTS FAILED"
    echo "=== Failures ==="
    grep "^--- FAIL:" "$FINAL_LOG"
fi
````

**Step 4.2: MUST write `$WORKDIR/summary.md`:**
````markdown
# TDD Summary

## Test File
`{test_file}`

## Workdir
`{workdir}`

## Iterations
- Total: {n}
- Final status: PASS/PARTIAL/FAIL

## Test Results (in order)
| # | Test Name | Status | Notes |
|---|-----------|--------|-------|
| 1 | TestFirst | ✓ PASS | |
| 2 | TestSecond | ✓ PASS | Fixed in iter 1 |
| 3 | TestThird | ✗ FAIL | See logs/test_iter2.log |
| 4 | TestFourth | ⚠ MISALIGNED | See test_issues.md |

## Code Changes Made
| File | Change | Reason |
|------|--------|--------|
| `file.go` | Modified `funcName()` | Test expected different return |
| `other.go` | Added error handling | Test checked error case |

## Breaking Changes Made
| Change | Justification |
|--------|---------------|
| Changed `Foo()` signature | Test expects new parameter |
| Removed `Bar()` | No longer needed, not tested |

## Cleanup Performed
| Type | Item | File | Reason |
|------|------|------|--------|
| Function removed | `oldHelper()` | util.go | Replaced by new impl |
| Dead code deleted | unused branch | handler.go | Tests don't cover it |

## Tests Requiring Updates (MISALIGNED)
| Test | Issue | Suggested Change |
|------|-------|------------------|
| TestWorkerType | Expects deprecated value | Update expected value |

See full details: `$WORKDIR/test_issues.md`

## Log Files
| File | Purpose |
|------|---------|
| logs/test_iter0.log | First test run (all tests) |
| logs/test_iter1.log | Second test run after fix (if needed) |
| logs/build_*.log | Build verification output |
| logs/final_run.log | Final test suite run |

## Final Build
- Command: `go build ./...`
- Log: `$WORKDIR/logs/build_final.log`
- Result: PASS/FAIL

## Action Required
- [ ] Human review needed for misaligned tests listed above
- [ ] Update tests to expect current behavior (not deprecated)
````

**Step 4.3: Verify summary was written**
````bash
ls -la "$WORKDIR/summary.md"
````

**Step 4.4: Copy TDD workdir to test results (if applicable)**

If the test creates a results directory (e.g., orchestrator/worker integration tests),
copy the entire TDD workdir to that results directory for archival:

````bash
# Tests that create results dirs will have them in test/results/api/
# Find the most recent results directory for this test
RESULTS_DIR=$(ls -td test/results/api/*${TASK_SLUG}* 2>/dev/null | head -1)

if [ -n "$RESULTS_DIR" ] && [ -d "$RESULTS_DIR" ]; then
    # Copy entire TDD workdir to results
    TDD_DEST="$RESULTS_DIR/tdd-workdir"
    cp -r "$WORKDIR" "$TDD_DEST"
    echo "Copied TDD workdir to: $TDD_DEST"
fi
````

The `common.CopyTDDSummary()` function in Go tests will also copy `summary.md` automatically,
but the full workdir copy above includes all artifacts (tdd_state.md, test_issues.md, logs/, etc.).

---
### ⟲ COMPACT POINT: TASK COMPLETE

**Run `/compact` at completion.** Clean slate for next task.

---

## FORBIDDEN (AUTO-FAIL)

| Action | Result |
|--------|--------|
| Modify `*_test.go` | FAILURE |
| Add `t.Skip()` | FAILURE |
| Change expected values | FAILURE |
| Weaken assertions | FAILURE |
| **Add backward compatibility** | FAILURE |
| **Keep deprecated types/APIs** | FAILURE |
| **Skip writing summary.md** | FAILURE |
| **Let full test output into context** | FAILURE |
| **Paste log file contents (>30 lines)** | FAILURE |

## ALLOWED (explicitly permitted)

| Action | Rationale |
|--------|-----------|
| Break existing APIs | New behavior is correct |
| Change function signatures | If current design needs it |
| Remove deprecated behavior | Old behavior should not exist |
| Modify return values | Current implementation is truth |
| Restructure code | Cleaner is better |
| Delete dead code | Cleaner codebase |
| Remove unused functions | If not tested with current behavior, not needed |
| Document test as misaligned | Tests expecting deprecated behavior need updating |
| Read log files with tail/head/grep | Bounded output extraction |

## MISALIGNED TEST HANDLING

**When a test expects DEPRECATED/OLD behavior:**

1. **DO NOT modify the test**
2. **DO NOT add backward compatibility**
3. **MUST document in `$WORKDIR/test_issues.md`:**
````markdown
## TestFunctionName

### Issue Type
- [x] Test expects deprecated value/type/constant
- [ ] Test expects removed API
- [ ] Test expects legacy behavior

### What Test Expects (DEPRECATED)
- Test expects: `old_value`
- This is deprecated because: <reason>

### What Test SHOULD Expect (CURRENT)
- Correct value: `new_value`
- Why: <rationale for new behavior>

### Suggested Test Change
```go
// Current (expects deprecated)
assert.Equal(t, "old_value", result)

// Should be (expects current)
assert.Equal(t, "new_value", result)
```

### Action Required
**Human must update test** to expect current behavior.
DO NOT add backward compatibility to make old test pass.
````

4. **Skip this test in subsequent iterations**
5. **Continue with remaining tests**
6. **Include in summary as "Tests Requiring Updates"**

## UI JOB TEST TEMPLATE

When test involves job monitoring, code MUST follow `test/ui/job_definition_general_test.go`:

### Progressive Screenshots (REQUIRED)
````go
screenshotTimes := []int{1, 2, 5, 10, 20, 30} // seconds from start
screenshotIdx := 0
lastPeriodicScreenshot := time.Now()

for {
    elapsed := time.Since(startTime)

    if screenshotIdx < len(screenshotTimes) &&
       int(elapsed.Seconds()) >= screenshotTimes[screenshotIdx] {
        utc.Screenshot(fmt.Sprintf("%s_%ds", prefix, screenshotTimes[screenshotIdx]))
        screenshotIdx++
    }

    if elapsed > 30*time.Second && time.Since(lastPeriodicScreenshot) >= 30*time.Second {
        utc.Screenshot(fmt.Sprintf("%s_%ds", prefix, int(elapsed.Seconds())))
        lastPeriodicScreenshot = time.Now()
    }
}
````

### Job Status Assertion (REQUIRED)
````go
expectedStatus := "completed" // or "failed" for failure tests
if currentStatus != expectedStatus {
    utc.Screenshot("unexpected_status")
    t.Fatalf("Expected status %s, got: %s", expectedStatus, currentStatus)
}
````

## COMPACTION SUMMARY

| Point | When | Why |
|-------|------|-----|
| Start | Phase 0 | Maximum headroom for iterations |
| After each iteration | During Phase 3 | Prevent accumulation |
| Complete | Phase 4 | Clean slate for next task |

## WORKDIR ARTIFACTS (MANDATORY)

| File | Purpose | When Created | Required |
|------|---------|--------------|----------|
| `tdd_state.md` | Current iteration state | Phase 2, updated each iteration | **YES** |
| `test_issues.md` | Misaligned tests | Phase 3, when tests expect deprecated | If applicable |
| `summary.md` | Final summary | Phase 4 | **YES - ALWAYS** |
| `logs/` | All captured output | Throughout | **YES** |
| `logs/test_iter*.log` | Per-iteration test runs (all tests) | Phase 3 | **YES** |
| `logs/build_*.log` | Build verifications | Phase 3 | **YES** |
| `logs/final_run.log` | Final test suite | Phase 4 | **YES** |

**Task is NOT complete until `summary.md` exists in workdir.**

## OUTPUT CAPTURE QUICK REFERENCE

````bash
# CORRECT: Run ALL tests in single process, output to file
go test -v -timeout 30m ./$TEST_PKG/... > "$WORKDIR/logs/test_iter0.log" 2>&1
tail -30 "$WORKDIR/logs/test_iter0.log"

# CORRECT: Build output to file, summary to Claude
go build ./... > "$WORKDIR/logs/build.log" 2>&1
tail -20 "$WORKDIR/logs/build.log"

# CORRECT: Extract specific error from log
grep -A5 "FAIL\|Error:" "$WORKDIR/logs/test_iter0.log" | head -15

# WRONG: Run tests individually (creates multiple processes/result dirs)
for TEST in ${TESTS[@]}; do
    go test -v -run "^${TEST}$" ./pkg/...  # DON'T DO THIS
done

# WRONG: Direct output to Claude (will overflow context)
go test -v ./$TEST_PKG/...

# WRONG: Cat entire log file
cat "$WORKDIR/logs/test_iter0.log"
````

## RESULTS INTEGRATION

When running tests that produce results directories (e.g., `test/results/api/orchestrator-*`):
- The TDD workdir is copied to `{results_dir}/tdd-workdir/` in Phase 4.4
- Go tests call `common.CopyTDDSummary()` which copies: `summary.md`, `tdd_state.md`, `test_issues.md`
- Log files in `logs/` provide full debugging context without bloating Claude's context
- This provides complete traceability from test results back to TDD session

## INVOKE
````
/3agents-tdd test/ui/job_definition_test.go
# → .claude/workdir/2024-12-17-1430-tdd-job_definition/
#    ├── tdd_state.md      (created Phase 2)
#    ├── test_issues.md    (if misaligned tests found)
#    ├── summary.md        (created Phase 4 - REQUIRED)
#    └── logs/
#        ├── test_iter0.log     (all tests, first run)
#        ├── test_iter1.log     (all tests, after fix - if needed)
#        ├── build_iter0.log
#        └── final_run.log

# Result: ONE results directory per iteration
# test/results/api/job_definition_20241217-143000/
#    ├── TestJobDefinitionFirst/
#    ├── TestJobDefinitionSecond/
#    └── TestJobDefinitionThird/
````