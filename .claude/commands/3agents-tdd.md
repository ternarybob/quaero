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

## FUNDAMENTAL RULE
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
│ Exception: If test is genuinely misaligned with requirements,   │
│ document the issue and SUGGEST changes (do not apply)           │
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
1. Re-read the test file to extract requirements
2. Check iteration count from git diff or file state
3. Resume PHASE 3 loop

## WORKFLOW

### PHASE 0: RESET CONTEXT

---
### ⟲ COMPACT POINT: START

**Run `/compact` before starting.** Clear context for maximum iteration headroom.

---

### PHASE 1: UNDERSTAND
1. Read test file - extract ALL test function names in order
2. Read skills for applicable patterns:
   - `.claude/skills/refactoring/SKILL.md` - Core patterns
   - `.claude/skills/go/SKILL.md` - Go changes
   - `.claude/skills/frontend/SKILL.md` - Frontend changes
   - `.claude/skills/monitoring/SKILL.md` - UI tests (screenshots, monitoring, results)
3. **For UI job tests** - validate against template: `test/ui/job_definition_general_test.go`

### PHASE 2: BUILD TEST LIST
````bash
# Extract ALL test names from file IN ORDER
TEST_LIST=$(grep "^func Test" {test_file} | sed 's/func \(Test[^(]*\).*/\1/')
TEST_PKG=$(dirname {test_file})

# Store as ordered array
TESTS=($TEST_LIST)
echo "Found ${#TESTS[@]} tests to run sequentially"
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
│       if FAIL → break loop, go to FIX                           │
│   done                                                          │
│                                                                 │
│   ALL PASSED → COMPLETE                                         │
└─────────────────────────────────────────────────────────────────┘
            │
         FAILURE at test N
            │
            ▼
┌─────────────────────────────────────────────────────────────────┐
│ ANALYZE FAILURE                                                 │
│                                                                 │
│ • Which test failed: ${TESTS[N]}                                │
│ • Error message/stack trace                                     │
│ • Expected vs Actual                                            │
│                                                                 │
│ DECISION:                                                       │
│   Code bug? → FIX THE CODE                                      │
│   Test misaligned? → DOCUMENT (suggest only, don't modify)      │
└───────────┬─────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────┐
│ FIX (if code bug)                                               │
│                                                                 │
│ • Apply skills (EXTEND > MODIFY > CREATE)                       │
│ • Follow Go/Frontend patterns                                   │
│ • Run build - must pass                                         │
│ • NO test file modifications                                    │
└───────────┬─────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────┐
│ OR DOCUMENT (if test misaligned)                                │
│                                                                 │
│ Write to: $WORKDIR/test_issues.md                               │
│                                                                 │
│ ## Test: {test_name}                                            │
│ ### Issue                                                       │
│ <why test is misaligned with requirements>                      │
│ ### Suggested Fix                                               │
│ <proposed test change - DO NOT APPLY>                           │
│ ### Evidence                                                    │
│ <requirements reference, code behavior>                         │
│                                                                 │
│ Then: SKIP this test for remaining iterations                   │
└───────────┬─────────────────────────────────────────────────────┘
            │
            ▼
       ITERATION++
            │
            ▼
    ┌───────┴───────┐
    │               │
ITERATION < 3    ITERATION = 3
    │               │
    ▼               ▼
 RESTART         STOP
 from test 1     Report status
````

### Execution Script
````bash
#!/bin/bash
TEST_FILE="{test_file}"
TEST_PKG=$(dirname "$TEST_FILE")
MAX_ITERATIONS=3

# Extract test names in order
mapfile -t TESTS < <(grep "^func Test" "$TEST_FILE" | sed 's/func \(Test[^(]*\).*/\1/')

echo "=== Sequential TDD Run ==="
echo "File: $TEST_FILE"
echo "Tests: ${#TESTS[@]}"
echo "Max iterations: $MAX_ITERATIONS"
echo ""

for ((iteration=1; iteration<=MAX_ITERATIONS; iteration++)); do
    echo "=== ITERATION $iteration ==="
    all_passed=true
    
    for ((i=0; i<${#TESTS[@]}; i++)); do
        test_name="${TESTS[$i]}"
        echo "--- Running test $((i+1))/${#TESTS[@]}: $test_name ---"
        
        if go test -v -run "^${test_name}$" "./$TEST_PKG/..." 2>&1; then
            echo "✓ PASS: $test_name"
        else
            echo "✗ FAIL: $test_name"
            echo ""
            echo ">>> FIX REQUIRED - then restart from test 1 <<<"
            all_passed=false
            break
        fi
    done
    
    if $all_passed; then
        echo ""
        echo "=== ALL TESTS PASSED ==="
        exit 0
    fi
    
    if [[ $iteration -lt $MAX_ITERATIONS ]]; then
        echo ""
        echo "Waiting for fix before iteration $((iteration+1))..."
        # Claude applies fix here, then continues
    fi
done

echo ""
echo "=== MAX ITERATIONS REACHED ==="
echo "Some tests still failing after $MAX_ITERATIONS attempts"
exit 1
````

---
### ⟲ COMPACT POINT: ITERATION 2

**Run `/compact` when iteration count reaches 2.**

Each fix attempt adds significant context. Compact before final iteration.

Recovery context:
- Test file: `{test_file}`
- Iteration: 2
- Failed test: Re-run sequential to find current failure
- Misaligned tests: Check `$WORKDIR/test_issues.md`

---

### PHASE 4: COMPLETE

**Success criteria:**
- All tests pass in sequential order
- No test files modified
- Build passes

**Partial success (iteration limit reached):**
- Document passing tests
- Document failing tests with analysis
- Document misaligned tests in `$WORKDIR/test_issues.md`

**Write `$WORKDIR/tdd_summary.md`:**
````markdown
# TDD Summary

## Test File
`{test_file}`

## Iterations
- Total: {n}
- Final status: PASS/PARTIAL/FAIL

## Test Results (in order)
| # | Test Name | Status | Notes |
|---|-----------|--------|-------|
| 1 | TestFirst | ✓ PASS | |
| 2 | TestSecond | ✓ PASS | |
| 3 | TestThird | ✗ FAIL | <reason> |
| 4 | TestFourth | ⚠ MISALIGNED | See test_issues.md |

## Code Changes Made
- `file.go`: <change description>

## Misaligned Tests (if any)
See: `$WORKDIR/test_issues.md`
````

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

## MISALIGNED TEST HANDLING

When a test appears to be wrong (not the code):

1. **DO NOT modify the test**
2. **Document in `$WORKDIR/test_issues.md`:**
````markdown
   ## TestFunctionName
   
   ### Issue Type
   - [ ] Test expects wrong value
   - [ ] Test logic doesn't match requirements
   - [ ] Test has race condition
   - [ ] Test setup is incorrect
   - [ ] Other: <describe>
   
   ### Evidence
   - Requirement says: <quote>
   - Test expects: <value>
   - Code correctly returns: <value>
   
   ### Suggested Test Change
```go
   // Current (incorrect)
   assert.Equal(t, "wrong", result)
   
   // Suggested (correct)
   assert.Equal(t, "right", result)
```
   
   ### Action Required
   Human review needed before test modification.
````
3. **Skip this test in subsequent iterations**
4. **Continue with remaining tests**

## UI JOB TEST TEMPLATE

When test involves job monitoring, code MUST follow `test/ui/job_definition_general_test.go`:

### Progressive Screenshots (REQUIRED)
````go
screenshotTimes := []int{1, 2, 5, 10, 20, 30} // seconds from start
screenshotIdx := 0
lastPeriodicScreenshot := time.Now()

for {
    elapsed := time.Since(startTime)

    // Progressive screenshots: 1s, 2s, 5s, 10s, 20s, 30s
    if screenshotIdx < len(screenshotTimes) &&
       int(elapsed.Seconds()) >= screenshotTimes[screenshotIdx] {
        utc.Screenshot(fmt.Sprintf("%s_%ds", prefix, screenshotTimes[screenshotIdx]))
        screenshotIdx++
    }

    // After 30s: screenshot every 30 seconds
    if elapsed > 30*time.Second && time.Since(lastPeriodicScreenshot) >= 30*time.Second {
        utc.Screenshot(fmt.Sprintf("%s_%ds", prefix, int(elapsed.Seconds())))
        lastPeriodicScreenshot = time.Now()
    }
    // ... monitoring loop
}
````

### Job Status Assertion (REQUIRED)
````go
// Assert EXPECTED terminal status (success OR failure depending on test intent)
expectedStatus := "completed" // or "failed" for failure tests
if currentStatus != expectedStatus {
    utc.Screenshot("unexpected_status")
    t.Fatalf("Expected status %s, got: %s", expectedStatus, currentStatus)
}
````

### Job Config in Results (REQUIRED)
````go
// Log job configuration at start
utc.Log("Job config: %+v", body)

// Add to test results/artifacts
utc.AddResult("job_config", body)
````

## COMPACTION SUMMARY

| Point | When | Why |
|-------|------|-----|
| Start | Phase 0 | Maximum headroom for iterations |
| Iteration 2 | During Phase 3 | Compact before final iteration |
| Complete | Phase 4 | Clean slate for next task |

**Emergency recovery:** If `/compact` fails:
1. Press Escape twice, retry
2. If still failing: `/clear`
3. Restart with: `/test-iterate {test_file}`

## INVOKE
````
/test-iterate test/ui/job_definition_test.go
````