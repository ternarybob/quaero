# Test Architecture Skill

**Prerequisite:** Read `.codebuff/skills/refactoring/SKILL.md` first.

## Test Structure

```
test/
├── api/              # API integration tests
│   ├── market_workers/   # Market worker tests
│   └── portfolio/        # Portfolio tests
├── ui/               # UI/browser tests (chromedp)
├── unit/             # Unit tests
└── common/           # Shared test utilities
```

## Required Output Files

API tests MUST produce these files in the results directory:

| File | Purpose |
|------|--------|
| `job_definition.json` | Job config for reproducibility |
| `output.md` | Worker-generated content |
| `output.json` | Document metadata |
| `test.log` | Test execution logs |
| `service.log` | Service output logs |

## Test Environment Setup

```go
func TestMyFeature(t *testing.T) {
    // 1. Setup fresh environment
    env := common.SetupFreshEnvironment(t)
    if env == nil {
        return
    }
    defer env.Cleanup()  // ALWAYS cleanup

    // 2. Get helpers
    helper := env.NewHTTPTestHelper(t)
    resultsDir := env.GetResultsDir()

    // 3. Initialize test log
    var testLog []string
    testLog = append(testLog, fmt.Sprintf("[%s] Test started", time.Now().Format(time.RFC3339)))

    // 4. Ensure test log written on all exit paths
    defer func() {
        WriteTestLog(t, resultsDir, testLog)
    }()

    // ... test logic ...
}
```

## Output Guard Pattern (MANDATORY)

Ensure outputs are saved on ALL exit paths:

```go
// Create guard early
guard := common.NewTestOutputGuard(t, resultsDir)
defer guard.Close()

guard.LogWithTimestamp("Test started")

// Save outputs UNCONDITIONALLY after job completes
SaveWorkerOutput(t, env, helper, outputTags, ticker)
guard.MarkOutputSaved()

// Validate at end
common.RequireTestOutputs(t, resultsDir)
```

## Job Execution Pattern

```go
// 1. Save job definition BEFORE execution
SaveJobDefinition(t, env, body)

// 2. Create and execute job
jobID, _ := CreateAndExecuteJob(t, helper, body)
if jobID == "" {
    return
}

// 3. Wait for completion with timeout
finalStatus := WaitForJobCompletion(t, helper, jobID, 3*time.Minute)
if finalStatus != "completed" {
    t.Skipf("Job ended with status %s", finalStatus)
    return
}

// 4. Assert and save outputs
outputTags := []string{"output-tag", strings.ToLower(ticker)}
metadata, content := AssertOutputNotEmpty(t, helper, outputTags)
SaveWorkerOutput(t, env, helper, outputTags, ticker)

// 5. Validate result files exist
AssertResultFilesExist(t, env, 1)

// 6. Check for service errors
AssertNoServiceErrors(t, env)
```

## UI Test Pattern (chromedp)

```go
func TestUIFeature(t *testing.T) {
    utc := NewUITestContext(t, 5*time.Minute)
    defer utc.Cleanup()  // ALWAYS cleanup

    utc.Log("Starting test")
    utc.Screenshot("initial")

    // Navigate and interact
    utc.Navigate("/page")
    utc.Click(".button")
    utc.Screenshot("after_click")

    // Assert
    text := utc.GetText(".result")
    assert.Contains(t, text, "expected")
}
```

## Anti-Patterns (AUTO-FAIL)

```go
// ❌ Missing cleanup
env := SetupFreshEnvironment(t)
// Missing: defer env.Cleanup()

// ❌ No output guard
func TestMyWorker(t *testing.T) {
    // If test panics, no outputs saved!
}

// ❌ Conditional output save
if len(docs) > 0 {
    SaveWorkerOutput(...)  // May not execute!
}

// ❌ Save outputs AFTER validation
validateAllTheThings(t, docs)  // If fails, outputs not saved
SaveWorkerOutput(...)

// ❌ Missing job definition save
jobID, _ := CreateAndExecuteJob(t, helper, body)
// Must call SaveJobDefinition BEFORE!

// ❌ No service error check
t.Log("PASS")
// Must call AssertNoServiceErrors!
```

## Checklist

### Test Structure
- [ ] Uses `SetupFreshEnvironment(t)`
- [ ] Has `defer env.Cleanup()`
- [ ] Uses `TestOutputGuard` pattern
- [ ] Test log written on all exit paths

### Output Validation
- [ ] `SaveJobDefinition()` called BEFORE job execution
- [ ] `SaveWorkerOutput()` called AFTER job completion (unconditionally)
- [ ] `AssertResultFilesExist()` called at end
- [ ] `AssertNoServiceErrors()` called at end

### Result Files
- [ ] `output.md` - Contains content, NOT empty
- [ ] `output.json` - Contains metadata, NOT empty
- [ ] `job_definition.json` - Job configuration
- [ ] `test.log` - Test execution logs
- [ ] `service.log` - Service output
