# Task 1: Rewrite test with API call tracking and no-refresh monitoring
Workdir: ./docs/fix/20251212-test-assertions/ | Depends: none | Critical: no
Model: opus | Skill: go

## Context
This task is part of: Adding proper assertions to the Codebase Classify UI test

## User Intent Addressed
- Test monitors job WITHOUT page refresh
- API call count assertion < 10
- Auto-expand verification in order
- Log display shows 1→15 not 5→15

## Input State
Files that exist before this task:
- `test/ui/job_definition_codebase_classify_test.go` - Simple test that uses generic RunJobDefinitionTest
- `test/ui/job_framework_test.go` - Framework with UITestContext and helpers

## Output State
Files after this task completes:
- `test/ui/job_definition_codebase_classify_test.go` - Enhanced test with:
  - Network request tracking for API call counting
  - Custom monitoring loop without page refresh
  - Assertions for step expansion order
  - Assertions for log line numbers

## Skill Patterns to Apply
### From go/SKILL.md:
- **DO:** Use context.Context for all operations
- **DO:** Use structured logging with utc.Log()
- **DO:** Wrap errors with context
- **DON'T:** Panic on errors
- **DON'T:** Use fmt.Println for logging

## Implementation Steps
1. Read current test file to understand structure
2. Implement network request tracking using chromedp's network events
3. Create custom monitoring function that:
   - Does NOT refresh the page
   - Tracks step expansion state via JavaScript
   - Counts API calls to /api/jobs/*/tree/logs
4. Add assertion functions:
   - AssertAPICallCount() - verifies < 10 calls
   - AssertStepsExpanded() - verifies steps expanded in order
   - AssertLogLineNumbers() - verifies logs show 1→15
5. Run test and verify

## Code Specifications
```go
// Key functions to implement:

// trackNetworkRequests enables network tracking and returns a counter
func (utc *UITestContext) trackNetworkRequests() (*NetworkTracker, error)

// NetworkTracker tracks API requests
type NetworkTracker struct {
    stepLogRequests []string // URLs of step log API calls
}

// monitorJobWithAssertions monitors without refresh and collects assertion data
func (utc *UITestContext) monitorJobWithAssertions(jobName string, opts MonitorJobOptions) (*JobMonitorResult, error)

// JobMonitorResult contains data for assertions
type JobMonitorResult struct {
    StepExpansionOrder []string // Order steps were expanded
    StepLogCounts      map[string]int // Step name -> first log line number
    FinalStatus        string
}

// Assertions
func assertAPICallCount(t *testing.T, tracker *NetworkTracker, maxCalls int)
func assertStepsExpandedInOrder(t *testing.T, result *JobMonitorResult, expectedOrder []string)
func assertLogStartsAtLine1(t *testing.T, result *JobMonitorResult, stepName string)
```

## Accept Criteria
- [ ] Test runs without page refresh during monitoring
- [ ] Test counts API calls to step log endpoint
- [ ] Test asserts API calls < 10
- [ ] Test tracks step expansion order
- [ ] Test asserts steps expand in completion order
- [ ] Test asserts log lines start at 1 (not 5)
- [ ] Build passes
- [ ] Test passes

## Handoff
After completion, next task(s): task-2 (run test and verify)
