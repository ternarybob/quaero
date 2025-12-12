# Task 1: Add JobDefinitionTestConfig and helper methods to framework

Workdir: ./docs/feature/20251212-job-definition-tests/ | Depends: none | Critical: no
Model: sonnet | Skill: go

## Context

This task is part of: Creating job definition test infrastructure for Quaero
Prior tasks completed: none - this is first

## User Intent Addressed

Create shared utilities for job definition tests so all tests use common code for startup/monitor/screenshots and copying job definitions to results.

## Input State

Files that exist before this task:
- `test/ui/job_framework_test.go` - UITestContext with TriggerJob, MonitorJob, Screenshot methods

## Output State

Files after this task completes:
- `test/ui/job_framework_test.go` - Extended with JobDefinitionTestConfig, RunJobDefinitionTest, CopyJobDefinitionToResults

## Skill Patterns to Apply

### From go/SKILL.md:
- **DO:** Use context.Context for I/O operations
- **DO:** Wrap errors with context using %w
- **DO:** Keep functions focused and small
- **DON'T:** Use bare errors without context
- **DON'T:** Panic on errors

## Implementation Steps

1. Read current `test/ui/job_framework_test.go` to understand existing patterns
2. Add `JobDefinitionTestConfig` struct with fields:
   - JobName string
   - JobDefinitionPath string (relative path to TOML)
   - Timeout time.Duration
   - RequiredEnvVars []string (API keys needed)
   - AllowFailure bool
3. Add `CopyJobDefinitionToResults(jobDefPath string)` method to UITestContext
4. Add `RefreshAndScreenshot(name string)` method to UITestContext
5. Add `RunJobDefinitionTest(config JobDefinitionTestConfig)` method that:
   - Checks required env vars, skips if missing
   - Copies job definition TOML to results dir
   - Triggers job
   - Monitors until completion with screenshots
   - Refreshes page and takes final screenshot

## Code Specifications

```go
// JobDefinitionTestConfig configures a job definition end-to-end test
type JobDefinitionTestConfig struct {
    JobName           string        // Name as shown in UI (e.g., "News Crawler")
    JobDefinitionPath string        // Path to TOML file (relative to test/ui/)
    Timeout           time.Duration // Max time to wait for job completion
    RequiredEnvVars   []string      // Env vars that must be set (skip if missing)
    AllowFailure      bool          // If true, don't fail test if job fails
}

// CopyJobDefinitionToResults copies the job definition TOML to test results directory
func (utc *UITestContext) CopyJobDefinitionToResults(jobDefPath string) error

// RefreshAndScreenshot refreshes the page and takes a screenshot
func (utc *UITestContext) RefreshAndScreenshot(name string) error

// RunJobDefinitionTest runs a complete job definition test with monitoring and screenshots
func (utc *UITestContext) RunJobDefinitionTest(config JobDefinitionTestConfig) error
```

## Accept Criteria

- [ ] JobDefinitionTestConfig struct defined with all fields
- [ ] CopyJobDefinitionToResults copies file to Env.ResultsDir
- [ ] RefreshAndScreenshot navigates to current URL and screenshots
- [ ] RunJobDefinitionTest orchestrates: env check, copy TOML, trigger, monitor, refresh, screenshot
- [ ] All new methods have error handling with context
- [ ] Code compiles: `go build ./test/ui/...`

## Handoff

After completion, next task(s): 2, 3, 4, 5 (job-specific tests)
