# API Test Architecture Skill

**Scope:** ALL tests in `test/api/*` (any subdirectory or directly in test/api/)

**Reference Implementations:**
- `test/api/market_workers/announcements_test.go` - Market worker pattern
- `test/api/portfolio/stock_deep_dive_test.go` - Portfolio orchestrator pattern

## When to Use

This skill MUST be followed when:
- Creating ANY new test in `test/api/*` (any subdirectory)
- Modifying ANY existing test in `test/api/*`
- Creating tests that execute job pipelines with output validation

This applies to ALL test/api/ tests including:
- `test/api/market_workers/*`
- `test/api/portfolio/*`
- `test/api/*_test.go` (tests directly in test/api/)

## Required Output Files

Every API test (market worker or portfolio) MUST produce these files in the results directory:

| File | Purpose | Function to Call | Required |
|------|---------|------------------|----------|
| `job_definition.json` | Job config for reproducibility | `SaveJobDefinition(t, env, body)` | YES |
| `output.md` | Worker-generated content (document content_markdown) | `SaveWorkerOutput(t, env, helper, tags, ticker)` | YES |
| `output.json` | Document metadata | `SaveWorkerOutput(t, env, helper, tags, ticker)` | YES |
| `schema.json` | Schema definition used for validation | `SaveSchemaDefinition(t, env, Schema, "Name")` | IF validating schema |
| `test.log` | Test execution logs | `WriteTestLog(t, resultsDir, entries)` | YES |
| `service.log` | Service output logs | Automatic via TestEnvironment | YES |
| `timing_data.json` | Execution timing data | `common.SaveTimingData(t, resultsDir, timing)` | RECOMMENDED |

### Portfolio-Specific Files

For portfolio tests using TOML job definitions:

| File | Purpose | Function to Call |
|------|---------|------------------|
| `job_definition.toml` | Original TOML job config | Manual copy from config directory |
| `multi_document_summary.md` | Summary of multi-document outputs | `saveMultiDocumentSummary()` |

## MANDATORY: TestOutputGuard Pattern (ENFORCED)

**CRITICAL:** Every API test MUST use `TestOutputGuard` to guarantee outputs are saved on ALL exit paths.

```go
func TestMyWorker(t *testing.T) {
    // 1. Environment setup
    env := common.SetupFreshEnvironment(t)
    if env == nil {
        return
    }
    defer env.Cleanup()

    resultsDir := env.GetResultsDir()

    // 2. MANDATORY: Create guard EARLY and defer Close
    guard := common.NewPortfolioTestOutputGuard(t, resultsDir)  // or NewMarketWorkerTestOutputGuard
    defer guard.Close()

    guard.LogWithTimestamp("Test started: TestMyWorker")

    // 3. Save job definition BEFORE execution
    SaveJobDefinition(t, resultsDir, body)
    guard.LogWithTimestamp("Job definition saved")

    // ... test logic ...

    // 4. Save outputs UNCONDITIONALLY after job completes
    // Even if subsequent assertions fail, outputs are saved
    SaveWorkerOutput(t, env, helper, outputTags, ticker)
    guard.MarkOutputSaved()
    guard.LogWithTimestamp("Outputs saved")

    // 5. Validate outputs at end
    common.RequireTestOutputs(t, resultsDir)  // Fails test if outputs missing
}
```

### Guard Types

| Guard | Use When | Config |
|-------|----------|--------|
| `NewPortfolioTestOutputGuard(t, resultsDir)` | Portfolio/orchestrator tests | Requires output.md, output.json, job_definition, test.log, service.log |
| `NewMarketWorkerTestOutputGuard(t, resultsDir)` | Market worker tests | Same as portfolio + schema.json |
| `NewTestOutputGuard(t, resultsDir, config)` | Custom requirements | Use `DefaultTestOutputConfig()` as base |

### Why Guard Pattern is MANDATORY

The guard pattern solves the recurring problem of missing test outputs:

1. **defer guard.Close()** - Ensures test.log is written even on panic/failure
2. **guard.LogWithTimestamp()** - Accumulates log entries that are written at the end
3. **guard.MarkOutputSaved()** - Tracks whether outputs were saved
4. **Close() validates** - Logs warnings if outputs are missing (doesn't fail - test may have already failed)

## Mandatory Function Calls

Tests MUST call these functions in order:

```go
// 1. BEFORE CreateAndExecuteJob - save job definition
SaveJobDefinition(t, env, body)

// 2. AFTER job completion - save schema (if validating)
SaveSchemaDefinition(t, env, WorkerSchema, "WorkerSchema")

// 3. AFTER job completion - save worker output
SaveWorkerOutput(t, env, helper, outputTags, ticker)

// 4. AT END - assert files exist
AssertResultFilesExist(t, env, 1)

// 5. AT END - check for service errors
AssertNoServiceErrors(t, env)
```

## Required Test Structure

### Single Stock Test

```go
func TestWorkerNameSingle(t *testing.T) {
    // 1. Environment Setup
    env := SetupFreshEnvironment(t)
    if env == nil {
        return
    }
    defer env.Cleanup()

    // 2. Service Requirements
    RequireLLM(t, env)  // or RequireEODHD, RequireAllMarketServices

    helper := env.NewHTTPTestHelper(t)
    ticker := "EXR"

    // 3. Job Definition (use config.variables pattern)
    defID := fmt.Sprintf("test-worker-single-%d", time.Now().UnixNano())
    body := map[string]interface{}{
        "id":          defID,
        "name":        "Worker Single Stock Test",
        "description": "Test description",
        "type":        "manager",
        "enabled":     true,
        "tags":        []string{"worker-test", "worker-name", "single-stock"},
        "config": map[string]interface{}{
            "variables": []map[string]interface{}{
                {"ticker": ticker},
            },
        },
        "steps": []map[string]interface{}{
            {
                "name": "step-name",
                "type": "worker_type",
            },
        },
    }

    // 4. MANDATORY: Save Job Definition
    SaveJobDefinition(t, env, body)

    // 5. Execute Job
    jobID, _ := CreateAndExecuteJob(t, helper, body)
    if jobID == "" {
        return
    }
    t.Logf("Executing job: %s", jobID)

    // 6. Wait for Completion
    finalStatus := WaitForJobCompletion(t, helper, jobID, 3*time.Minute)
    if finalStatus != "completed" {
        t.Skipf("Job ended with status %s", finalStatus)
        return
    }

    // 7. Assert Output
    outputTags := []string{"output-tag", strings.ToLower(ticker)}
    metadata, content := AssertOutputNotEmpty(t, helper, outputTags)

    // 8. Assert Content
    expectedSections := []string{"Section1", "Section2"}
    AssertOutputContains(t, content, expectedSections)

    // 9. Validate Schema
    isValid := ValidateSchema(t, metadata, WorkerSchema)
    assert.True(t, isValid, "Output should comply with schema")

    // 10. MANDATORY: Save Schema Definition
    SaveSchemaDefinition(t, env, WorkerSchema, "WorkerSchema")

    // 11. MANDATORY: Save Worker Output
    SaveWorkerOutput(t, env, helper, outputTags, ticker)

    // 12. MANDATORY: Assert Result Files Exist
    AssertResultFilesExist(t, env, 1)
    AssertSchemaFileExists(t, env)

    // 13. MANDATORY: Check for Service Errors
    AssertNoServiceErrors(t, env)

    t.Log("PASS: worker single stock test completed")
}
```

### Multi Stock Test

```go
func TestWorkerNameMulti(t *testing.T) {
    env := SetupFreshEnvironment(t)
    if env == nil {
        return
    }
    defer env.Cleanup()

    RequireLLM(t, env)
    helper := env.NewHTTPTestHelper(t)

    stocks := []string{"EXR", "GNP", "SKS", "TWR"}

    for _, stock := range stocks {
        t.Run(stock, func(t *testing.T) {
            defID := fmt.Sprintf("test-worker-%s-%d", strings.ToLower(stock), time.Now().UnixNano())

            body := map[string]interface{}{
                "id":   defID,
                "type": "manager",
                "config": map[string]interface{}{
                    "variables": []map[string]interface{}{
                        {"ticker": stock},
                    },
                },
                "steps": []map[string]interface{}{
                    {
                        "name": "step-name",
                        "type": "worker_type",
                    },
                },
            }

            // Save job definition for FIRST stock only
            if stock == stocks[0] {
                SaveJobDefinition(t, env, body)
            }

            jobID, _ := CreateAndExecuteJob(t, helper, body)
            if jobID == "" {
                return
            }

            finalStatus := WaitForJobCompletion(t, helper, jobID, 2*time.Minute)
            if finalStatus != "completed" {
                t.Logf("Job for %s ended with status %s", stock, finalStatus)
                return
            }

            // Assertions
            outputTags := []string{"output-tag", strings.ToLower(stock)}
            metadata, content := AssertOutputNotEmpty(t, helper, outputTags)
            assert.NotEmpty(t, content, "Content for %s should not be empty", stock)

            isValid := ValidateSchema(t, metadata, WorkerSchema)
            assert.True(t, isValid, "Output for %s should comply with schema", stock)

            // MANDATORY: Save output for each stock
            SaveWorkerOutput(t, env, helper, outputTags, stock)

            t.Logf("PASS: Validated worker for %s", stock)
        })
    }

    // MANDATORY: Check for service errors
    AssertNoServiceErrors(t, env)

    t.Log("PASS: worker multi-stock test completed")
}
```

## Job Definition Patterns

### config.variables (REQUIRED)

```go
"config": map[string]interface{}{
    "variables": []map[string]interface{}{
        {"ticker": "EXR"},
    },
},
```

### Step-Based Tag Routing

```go
"steps": []map[string]interface{}{
    {
        "name": "analyze_competitors",
        "type": "market_competitor",
        "config": map[string]interface{}{
            "output_tags": []string{"format_output"},  // Next step name
        },
    },
    {
        "name": "format_output",
        "type": "output_formatter",
        "config": map[string]interface{}{
            "output_tags": []string{"email_report"},
        },
    },
    {
        "name": "email_report",
        "type": "email",
        "config": map[string]interface{}{
            "to":      "{email_recipient}",
            "subject": "Report Subject",
        },
    },
}
```

### Variable Substitution

| Variable | Description |
|----------|-------------|
| `{google_gemini_api_key}` | Gemini API key |
| `{email_recipient}` | Email recipient address |
| `{eodhd_api_key}` | EODHD API key |

## Anti-Patterns (AUTO-FAIL)

### Test Structure Anti-Patterns

```go
// ❌ WRONG: No TestOutputGuard - outputs may not be saved on failure
func TestMyWorker(t *testing.T) {
    env := SetupFreshEnvironment(t)
    defer env.Cleanup()
    // Missing guard! If test panics, no test.log is written
}

// ✓ CORRECT: TestOutputGuard ensures outputs on all exit paths
func TestMyWorker(t *testing.T) {
    env := SetupFreshEnvironment(t)
    defer env.Cleanup()
    guard := common.NewPortfolioTestOutputGuard(t, env.GetResultsDir())
    defer guard.Close()
}

// ❌ WRONG: Conditional output save - outputs missing on failure
if len(docs) > 0 {
    SaveWorkerOutput(t, env, helper, tags, ticker)
}

// ✓ CORRECT: Unconditional output save with require assertion
require.Greater(t, len(docs), 0, "Must have documents")
SaveWorkerOutput(t, env, helper, tags, ticker)

// ❌ WRONG: Saving outputs AFTER validation (may never execute)
validateAllTheThings(t, docs)  // If this fails, outputs not saved!
SaveWorkerOutput(t, env, helper, tags, ticker)

// ✓ CORRECT: Save outputs BEFORE detailed validation
SaveWorkerOutput(t, env, helper, tags, ticker)
guard.MarkOutputSaved()
validateAllTheThings(t, docs)  // Outputs already saved if this fails
```

### Job Definition Anti-Patterns

```go
// ❌ Missing SaveJobDefinition
jobID, _ := CreateAndExecuteJob(t, helper, body)
// Must call SaveJobDefinition(t, env, body) BEFORE this!

// ❌ Missing SaveWorkerOutput
AssertOutputNotEmpty(t, helper, tags)
// Must call SaveWorkerOutput(t, env, helper, tags, ticker) AFTER!

// ❌ Missing AssertResultFilesExist
AssertNoServiceErrors(t, env)
// Must call AssertResultFilesExist(t, env, 1) BEFORE!

// ❌ Hardcoded ticker in step config (use variables)
"config": map[string]interface{}{
    "asx_code": "EXR",  // WRONG - use config.variables
}

// ❌ Missing AssertNoServiceErrors at end
t.Log("PASS: test completed")
// Must call AssertNoServiceErrors(t, env) BEFORE this!
```

## Checklist

When creating or reviewing API tests (market workers or portfolio):

### Test Structure (MANDATORY)
- [ ] `TestOutputGuard` created EARLY in test
- [ ] `defer guard.Close()` called immediately after guard creation
- [ ] `guard.LogWithTimestamp()` used for test progress logging
- [ ] `guard.MarkOutputSaved()` called after saving outputs

### Required Output Validation
- [ ] `SaveJobDefinition(t, env, body)` called BEFORE `CreateAndExecuteJob`
- [ ] `SaveSchemaDefinition(t, env, Schema, "Name")` called IF using schema validation
- [ ] `SaveWorkerOutput(t, env, helper, tags, ticker)` called AFTER job completion (UNCONDITIONALLY)
- [ ] `RequireTestOutputs(t, resultsDir)` OR `AssertResultFilesExist(t, env, 1)` called AT END
- [ ] `AssertNoServiceErrors(t, env)` called AT END

### Job Configuration
- [ ] Job uses `config.variables` pattern for tickers
- [ ] Step names use underscores (e.g., `format_output`, `email_report`)
- [ ] Tags include `"worker-test"` for identification

### Result Files Verification
After test completion, verify these files exist in `test/results/api/{test_name}/`:
- [ ] `output.md` - Contains worker-generated content, NOT empty
- [ ] `output.json` - Contains document metadata, NOT empty
- [ ] `job_definition.json` OR `job_definition.toml` - Job configuration
- [ ] `test.log` - Test execution logs (guard ensures this is always written)
- [ ] `service.log` - Service output

### Output Validation Functions

| Function | When to Use |
|----------|-------------|
| `RequireTestOutputs(t, resultsDir)` | Strict validation - fails test if outputs missing |
| `RequirePortfolioTestOutputs(t, resultsDir)` | Portfolio tests with timing requirements |
| `RequireMarketWorkerTestOutputs(t, resultsDir)` | Market worker tests with schema requirements |
| `AssertTestOutputs(t, resultsDir, config)` | Custom validation with specific config |

## See Also

- `docs/architecture/TEST_ARCHITECTURE.md` - Full test architecture documentation
- `test/common/result_helpers.go` - Output save and validation helpers
- `test/api/market_workers/common_test.go` - Market worker helper functions
- `test/api/portfolio/common_test.go` - Portfolio helper functions
