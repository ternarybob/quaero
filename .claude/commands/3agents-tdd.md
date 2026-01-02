---
name: 3agents-tdd
description: TDD enforcement - tests are IMMUTABLE, fix code until tests pass. Sequential execution with full restart on fix.
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
````

### PHASE 3: SEQUENTIAL TEST LOOP (max 3 iterations)
````
ITERATION = 0
     │
     ▼
┌─────────────────────────────────────────────────────────────────┐
│ START SEQUENTIAL RUN (iteration $ITERATION)                     │
│                                                                 │
│   for TEST in ${TESTS[@]}; do                                   │
│       go test -v -run "^${TEST}$" ./$TEST_PKG/...               │
│                                                                 │
│       if PASS → continue to next test                           │
│       if FAIL → break loop, go to ANALYZE                       │
│   done                                                          │
│                                                                 │
│   ALL PASSED → PHASE 4 (COMPLETE)                               │
└─────────────────────────────────────────────────────────────────┘
            │
         FAILURE at test N
            │
            ▼
┌─────────────────────────────────────────────────────────────────┐
│ ANALYZE FAILURE - CRITICAL DECISION POINT                       │
│                                                                 │
│ • Which test failed: ${TESTS[N]}                                │
│ • Error message/stack trace                                     │
│ • Expected vs Actual                                            │
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
│ NEVER:      │ │ Then: SKIP this test, continue with next        │
│ • Add compat│ │                                                 │
│ • Keep old  │ │ DO NOT add backward compatibility!              │
│   behavior  │ │                                                 │
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

**MUST update `$WORKDIR/tdd_state.md` after each iteration:**
````markdown
## Current State
- Iteration: {n}
- Last failed test: {test_name}
- Status: IN_PROGRESS

## Iteration History
### Iteration 1
- Failed at: TestSecond
- Error: <brief error>
- Action: CODE_FIX / TEST_MISALIGNED
- Details: <what was changed or documented>
````

---
### ⟲ COMPACT POINT: ITERATION 2

**Run `/compact` when iteration count reaches 2.**

Recovery context:
- Read: `$WORKDIR/tdd_state.md`
- Misaligned tests: Check `$WORKDIR/test_issues.md`

---

### PHASE 4: COMPLETE (MANDATORY)

**This phase MUST execute. Task is incomplete without it.**

**Step 4.1: Verify final state**
- All tests pass in sequential order (or documented as misaligned)
- No test files modified
- Build passes

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
| 2 | TestSecond | ✓ PASS | |
| 3 | TestThird | ✗ FAIL | <reason> |
| 4 | TestFourth | ⚠ MISALIGNED | Test expects deprecated behavior |

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

## Final Build
- Command: `./scripts/build.sh` or `go build ./...`
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
but the full workdir copy above includes all artifacts (tdd_state.md, test_issues.md, etc.).

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
| Iteration 2 | During Phase 3 | Compact before final iteration |
| Complete | Phase 4 | Clean slate for next task |

## WORKDIR ARTIFACTS (MANDATORY)

| File | Purpose | When Created | Required |
|------|---------|--------------|----------|
| `tdd_state.md` | Current iteration state | Phase 2, updated each iteration | **YES** |
| `test_issues.md` | Misaligned tests | Phase 3, when tests expect deprecated | If applicable |
| `summary.md` | Final summary | Phase 4 | **YES - ALWAYS** |

**Task is NOT complete until `summary.md` exists in workdir.**

## RESULTS INTEGRATION

When running tests that produce results directories (e.g., `test/results/api/orchestrator-*`):
- The TDD workdir is copied to `{results_dir}/tdd-workdir/` in Phase 4.4
- Go tests call `common.CopyTDDSummary()` which copies: `summary.md`, `tdd_state.md`, `test_issues.md`
- This provides complete traceability from test results back to TDD session

## INVOKE
````
/test-iterate test/ui/job_definition_test.go
# → .claude/workdir/2024-12-17-1430-tdd-job_definition/
#    ├── tdd_state.md      (created Phase 2)
#    ├── test_issues.md    (if misaligned tests found)
#    └── summary.md        (created Phase 4 - REQUIRED)
````