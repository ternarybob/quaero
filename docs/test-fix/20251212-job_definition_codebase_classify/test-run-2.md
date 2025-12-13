# Test Run 2
File: test/ui/job_definition_codebase_classify_test.go
Date: 2025-12-12 19:00:08

## Result: FAIL (partial improvement)

## Test Output
```
=== RUN   TestJobDefinitionCodebaseClassify
    setup.go:1367: --- Testing Job Definition: Codebase Classify (with assertions) ---
    [... job triggered and monitored ...]
    setup.go:1367: Status change: running -> completed (at 2m38s)
    setup.go:1367: ✓ Job reached terminal status: completed
    setup.go:1367: --- Running Assertions ---
    setup.go:1367: Assertion 1: Step Log API calls = 3 (max allowed: 10)
    setup.go:1367: ✓ PASS: Step Log API calls within limit
    setup.go:1367: Assertion 2: Checking step icons match parent job icon standard...
    setup.go:1367: Found 3 step icons to verify
    setup.go:1367: ✓ Step 'code_map' icon correct: fa-spinner for status running
    setup.go:1367: ✓ Step 'import_files' icon correct: fa-check-circle for status completed
    setup.go:1367: ✓ Step 'rule_classify_files' icon correct: fa-clock for status pending
    setup.go:1367: ✓ PASS: All step icons match parent job icon standard
    setup.go:1367: Assertion 3: Checking log line numbering for all steps...
    setup.go:1367: Checking log line numbering for 2 steps
    setup.go:1367: Step 'code_map' log lines: [1 2 3 4 5 6 7 8 9 10 11 12 13]
    setup.go:1367: Step 'rule_classify_files' log lines: [1 2 3 ... 100]
    setup.go:1367: ✓ PASS: All steps have correct sequential log line numbering starting at 1
    setup.go:1367: Assertion 4: Step expansion order = [code_map rule_classify_files]
    setup.go:1367: Total steps in job: 3, Steps auto-expanded: 2
    setup.go:1367: All step names: [code_map import_files rule_classify_files]
    setup.go:1367: Auto-expanded steps: [code_map rule_classify_files]
    job_definition_codebase_classify_test.go:697: FAIL: Not all steps auto-expanded. Missing: [import_files] (expected 3, got 2)
--- FAIL: TestJobDefinitionCodebaseClassify (175.04s)
```

## Fixes Applied (Working)
| Test | Status |
|------|--------|
| Step icon mismatch | ✓ PASS - fa-clock now used for pending |
| Log line numbering | ✓ PASS - All steps start at line 1 |

## Remaining Failure
| Test | Error | Location |
|------|-------|----------|
| Auto-expand import_files | Step 'import_files' did not auto-expand | job_definition_codebase_classify_test.go:714 |

## Root Cause Analysis
The `import_files` step completes extremely fast (< 1s). By the time the test:
1. Triggers the job
2. Navigates to Queue page
3. Loads tree data via loadJobTreeData()

...the `import_files` step has already completed. The auto-expand logic only triggers on status CHANGE events. Since the step is already completed when the tree data loads, no status change event fires.

The `loadJobTreeData` function at line 4245-4263 was updated to auto-expand completed steps, but the issue is that `import_files` may not have logs yet when tree loads (fast step), so it may not be captured by the test's `checkStepExpansionState` function which checks `jobTreeExpandedSteps[key]`.
