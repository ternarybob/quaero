# Test Run 1
File: test/ui/job_definition_codebase_classify_test.go
Date: 2025-12-12 18:56:08

## Result: FAIL

## Test Output
```
=== RUN   TestJobDefinitionCodebaseClassify
    setup.go:1367: --- Testing Job Definition: Codebase Classify (with assertions) ---
    setup.go:1367: Copying job definition: ../config/job-definitions/codebase_classify.toml
    setup.go:1367: ✓ Job definition copied to: ..\..\test\results\ui\job-20251212-185608\TestJobDefinitionCodebaseClassify\codebase_classify.toml
    setup.go:1367: Enabling network request tracking...
    setup.go:1367: Triggering job: Codebase Classify
    setup.go:1367: Looking for button with ID: codebase-classify-run
    setup.go:1367: Waiting for confirmation modal
    setup.go:1367: Confirming run
    setup.go:1367: ✓ Job triggered: Codebase Classify
    setup.go:1367: Waiting for job to appear in queue...
    setup.go:1367: Starting job monitoring (NO page refresh)...
    setup.go:1367: Status change:  -> running (at 1s)
    setup.go:1367: Status change: running -> completed (at 9s)
    setup.go:1367: ✓ Job reached terminal status: completed
    setup.go:1367: --- Running Assertions ---
    setup.go:1367: Assertion 1: Step Log API calls = 3 (max allowed: 10)
    setup.go:1367: ✓ PASS: Step Log API calls within limit
    setup.go:1367: Assertion 2: Checking step icons match parent job icon standard...
    setup.go:1367: Found 3 step icons to verify
    setup.go:1367: ✓ Step 'code_map' icon correct: fa-spinner for status running
    setup.go:1367: ✓ Step 'import_files' icon correct: fa-check-circle for status completed
    job_definition_codebase_classify_test.go:551: FAIL: Step 'rule_classify_files' icon mismatch - status=pending, expected=fa-clock, actual=fa-circle
    job_definition_codebase_classify_test.go:559: FAIL: 1 step icon(s) do not match parent job icon standard
    setup.go:1367: Assertion 3: Checking log line numbering for all steps...
    setup.go:1367: Checking log line numbering for 2 steps
    setup.go:1367: Step 'code_map' log lines: [1 2 3 4 5 6 7 8 9 10 11 12 13]
    setup.go:1367: Step 'rule_classify_files' log lines: [4439 4440 4441 4442 4443 4444 4445 4446 4447 4448 4449 4450 4451 4452 4453 4454 4455 4456 4457 4458 4459 4460 4461 4462 4463 4464 4465 4466 4467 4468 4469 4470 4471 4472 4473 4474 4475 4476 4477 4478 4479 4480 4481 4482 4483 4484 4485 4486 4487 4488 4489 4490 4491 4492 4493 4494 4495 4496 4497 4498 4499 4500 4501 4502 4503 4504 4505 4506 4507 4508 4509 4510 4511 4512 4513 4514 4515 4516 4517 4518 4519 4520 4521 4522 4523 4524 4525 4526 4527 4528 4529 4530 4531 4532 4533 4534 4535 4536 4537 4538]
    job_definition_codebase_classify_test.go:623: FAIL: Step 'rule_classify_files' logs do NOT start at line 1 (starts at 4439)
    job_definition_codebase_classify_test.go:640: FAIL: 1 step(s) have incorrect log line numbering
    setup.go:1367: Assertion 4: Step expansion order = [code_map rule_classify_files]
    setup.go:1367: Total steps in job: 3, Steps auto-expanded: 2
    setup.go:1367: All step names: [code_map import_files rule_classify_files]
    setup.go:1367: Auto-expanded steps: [code_map rule_classify_files]
    job_definition_codebase_classify_test.go:697: FAIL: Not all steps auto-expanded. Missing: [import_files] (expected 3, got 2)
    job_definition_codebase_classify_test.go:714: FAIL: Expected step 'import_files' did not auto-expand
    setup.go:1367: ✓ Codebase Classify job definition test completed with all assertions
--- FAIL: TestJobDefinitionCodebaseClassify (26.12s)
FAIL
```

## Failures
| Test | Error | Location |
|------|-------|----------|
| Step icon mismatch | Step 'rule_classify_files' icon mismatch - status=pending, expected=fa-clock, actual=fa-circle | job_definition_codebase_classify_test.go:551 |
| Log line numbering | Step 'rule_classify_files' logs do NOT start at line 1 (starts at 4439) | job_definition_codebase_classify_test.go:623 |
| Auto-expand | Not all steps auto-expanded. Missing: [import_files] (expected 3, got 2) | job_definition_codebase_classify_test.go:697 |
| Auto-expand import_files | Expected step 'import_files' did not auto-expand | job_definition_codebase_classify_test.go:714 |
