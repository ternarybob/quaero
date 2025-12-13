# Task 5: Create codebase_classify job definition test

Workdir: ./docs/feature/20251212-job-definition-tests/ | Depends: 1 | Critical: no
Model: sonnet | Skill: go

## Context

This task is part of: Creating job definition test infrastructure for Quaero
Prior tasks completed: Task 1 - Framework helper methods added

## User Intent Addressed

Create test for codebase_classify job definition (local dir import + code map + rule classification) that runs end-to-end with monitoring and screenshots.

## Input State

Files that exist before this task:
- `test/ui/job_framework_test.go` - UITestContext with RunJobDefinitionTest method
- `test/config/job-definitions/codebase_classify.toml` - Codebase analysis pipeline definition

## Output State

Files after this task completes:
- `test/ui/job_definition_codebase_classify_test.go` - Complete test file for codebase analysis

## Skill Patterns to Apply

### From go/SKILL.md:
- **DO:** Use reasonable test timeouts (not production timeout)
- **DO:** Wrap errors with context using %w
- **DO:** Put integration tests in test/ui/
- **DON'T:** Hard-code paths that won't work across environments

## Implementation Steps

1. Create `test/ui/job_definition_codebase_classify_test.go`
2. Define TestJobDefinitionCodebaseClassify function
3. Configure JobDefinitionTestConfig for codebase_classify:
   - JobName: "Codebase Classify"
   - JobDefinitionPath: "../config/job-definitions/codebase_classify.toml"
   - Timeout: 15 minutes (reasonable test timeout, job has 4h but tests shouldn't wait that long)
   - RequiredEnvVars: none (rule_classifier doesn't need API keys)
   - AllowFailure: true (file paths may not exist in test env)
4. Call RunJobDefinitionTest with config
5. Log success

## Code Specifications

```go
package ui

import (
    "testing"
    "time"
)

// TestJobDefinitionCodebaseClassify tests the codebase analysis pipeline job definition
func TestJobDefinitionCodebaseClassify(t *testing.T) {
    utc := NewUITestContext(t, 20*time.Minute)
    defer utc.Cleanup()

    utc.Log("--- Testing Job Definition: Codebase Classify ---")

    config := JobDefinitionTestConfig{
        JobName:           "Codebase Classify",
        JobDefinitionPath: "../config/job-definitions/codebase_classify.toml",
        Timeout:           15 * time.Minute, // Job has 4h timeout but tests use shorter
        RequiredEnvVars:   nil, // rule_classifier doesn't need API keys
        AllowFailure:      true, // May fail if paths don't exist in test env
    }

    if err := utc.RunJobDefinitionTest(config); err != nil {
        t.Fatalf("Job definition test failed: %v", err)
    }

    utc.Log("✓ Codebase Classify job definition test completed successfully")
}
```

## Accept Criteria

- [ ] File `test/ui/job_definition_codebase_classify_test.go` exists
- [ ] Test function TestJobDefinitionCodebaseClassify defined
- [ ] Uses JobDefinitionTestConfig with correct values
- [ ] Timeout set to 15 minutes (reasonable for test, not 4h)
- [ ] No required env vars (rule_classifier is local)
- [ ] AllowFailure set to true (path issues possible)
- [ ] Code compiles: `go build ./test/ui/...`

## Handoff

After completion, next task(s): 6 (verification)
