# Market Worker Test Skill

**Scope:** Tests in `test/api/market_workers/`

**Reference Implementation:** `test/api/market_workers/announcements_test.go`

## When to Use

This skill MUST be followed when:
- Creating new tests in `test/api/market_workers/`
- Modifying existing market worker tests
- Creating tests that execute job pipelines with output validation

## Required Output Files

Every market worker test MUST produce these files in the results directory:

| File | Function to Call |
|------|------------------|
| `job_definition.json` | `SaveJobDefinition(t, env, body)` |
| `schema.json` | `SaveSchemaDefinition(t, env, Schema, "Name")` |
| `output.md` | `SaveWorkerOutput(t, env, helper, tags, ticker)` |
| `output.json` | `SaveWorkerOutput(t, env, helper, tags, ticker)` |

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

When creating or reviewing market worker tests:

- [ ] `SaveJobDefinition(t, env, body)` called BEFORE `CreateAndExecuteJob`
- [ ] `SaveSchemaDefinition(t, env, Schema, "Name")` called IF using schema validation
- [ ] `SaveWorkerOutput(t, env, helper, tags, ticker)` called AFTER job completion
- [ ] `AssertResultFilesExist(t, env, 1)` called AT END
- [ ] `AssertNoServiceErrors(t, env)` called AT END
- [ ] Job uses `config.variables` pattern for tickers
- [ ] Step names use underscores (e.g., `format_output`, `email_report`)
- [ ] Tags include `"worker-test"` for identification
