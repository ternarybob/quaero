---
name: 3agents-tdd
description: TDD enforcement - tests are IMMUTABLE, fix code until tests pass
---

Execute: $ARGUMENTS

**Read first:** `.claude/skills/refactoring/SKILL.md`

## INPUT VALIDATION
```
1. Normalize path: replace \ with /
2. Must be *_test.go file or STOP
```

## FUNDAMENTAL RULE
```
┌─────────────────────────────────────────────────────────────────┐
│ TESTS ARE IMMUTABLE LAW                                         │
│                                                                  │
│ • Touch a test file = FAILED                                    │
│ • Weaken an assertion = FAILED                                  │
│ • Skip/delete a test = FAILED                                   │
│                                                                  │
│ Test expects X, code returns Y → FIX THE CODE                   │
└─────────────────────────────────────────────────────────────────┘
```

## WORKFLOW

### PHASE 0: RESET CONTEXT
```
/compact
```
Clear context before starting to ensure maximum headroom for iterations.

### PHASE 1: UNDERSTAND
1. Read test file - extract requirements
2. Read skills for applicable patterns:
   - `.claude/skills/refactoring/SKILL.md` - Core patterns
   - `.claude/skills/go/SKILL.md` - Go changes
   - `.claude/skills/frontend/SKILL.md` - Frontend changes
   - `.claude/skills/monitoring/SKILL.md` - UI tests (screenshots, monitoring, results)
3. **For UI job tests** - validate against template: `test/ui/job_definition_general_test.go`

### PHASE 2: RUN TEST
```bash
# Extract ALL test names from file, build regex pattern
TEST_PATTERN=$(grep "^func Test" {test_file} | sed 's/func \(Test[^(]*\).*/\1/' | paste -sd'|')
TEST_PKG=$(dirname {test_file})
go test -v -run "^($TEST_PATTERN)$" ./$TEST_PKG/...
```
- Run ONLY tests defined in the specified file
- **ALL PASS →** Complete
- **ANY FAIL →** Fix & Iterate

### PHASE 3: FIX & ITERATE (max 5)
```
TEST FAILS
     │
     ▼
┌─────────────────────────┐
│ ANALYZE                 │
│ • Test is RIGHT         │
│ • Code is WRONG         │
└───────────┬─────────────┘
            ▼
┌─────────────────────────┐
│ FIX (apply skills)      │
│ • Modify code ONLY      │
│ • EXTEND > MODIFY > CREATE│
│ • Follow Go/Frontend skill│
│ • Run build             │
└───────────┬─────────────┘
            ▼
┌─────────────────────────┐
│ VALIDATE TEMPLATE       │
│ • UI job test? Check:   │
│   - Progressive screenshots│
│   - Assert expected status│
│   - Config in results   │
└───────────┬─────────────┘
            ▼
┌─────────────────────────┐
│ VERIFY                  │
│ • No test files changed │
│ • Re-run tests          │
└───────────┬─────────────┘
     ┌──────┴──────┐
     ▼             ▼
   FAIL          PASS → Complete
     │
     └──► Loop
```

### PHASE 4: COMPLETE
- All tests pass
- No test files modified

## FORBIDDEN (AUTO-FAIL)

| Action | Result |
|--------|--------|
| Modify `*_test.go` | FAILURE |
| Add `t.Skip()` | FAILURE |
| Change expected values | FAILURE |
| Weaken assertions | FAILURE |

## UI JOB TEST TEMPLATE

When test involves job monitoring, code MUST follow `test/ui/job_definition_general_test.go`:

### Progressive Screenshots (REQUIRED)
```go
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
```

### Job Status Assertion (REQUIRED)
```go
// Assert EXPECTED terminal status (success OR failure depending on test intent)
expectedStatus := "completed" // or "failed" for failure tests
if currentStatus != expectedStatus {
    utc.Screenshot("unexpected_status")
    t.Fatalf("Expected status %s, got: %s", expectedStatus, currentStatus)
}
```
**Note:** Tests validating failure scenarios (error tolerance, invalid config, etc.) should expect `"failed"` status.

### Job Config in Results (REQUIRED)
```go
// Log job configuration at start
utc.Log("Job config: %+v", body)

// Add to test results/artifacts
utc.AddResult("job_config", body)
```

## INVOKE
```
/test-iterate test/ui/job_definition_test.go
```