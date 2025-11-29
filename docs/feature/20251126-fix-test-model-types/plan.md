# Plan: Fix Test Model Types

## Dependency Analysis

The test files reference old model types (`models.Job`, `models.JobModel`) that no longer exist. The current types are:
- `queue.Job` - Internal job record (in `internal/queue/lifecycle.go`)
- `models.QueueJob` - Immutable queue job model
- `models.QueueJobState` - Runtime job state
- `models.CrawlJob` - Crawler-specific job (in `internal/models/crawler_job.go`)

Test files need to be updated to use the correct types.

## Critical Path Flags
No critical flags - these are test fixes only.

## Execution Groups

### Group 1 (Parallel - Independent Test Fixes)
These can run simultaneously as they modify different test files:

1a. **Fix crawler logging_test.go**
    - Skill: @test-writer
    - Files: internal/services/crawler/logging_test.go
    - Critical: no
    - Depends on: none
    - User decision: no
    - Sandbox: worker-a
    - Errors: models.Job, models.JobModel undefined

1b. **Fix logs service_test.go**
    - Skill: @test-writer
    - Files: internal/logs/service_test.go
    - Critical: no
    - Depends on: none
    - User decision: no
    - Sandbox: worker-b
    - Errors: models.Job, models.JobModel undefined

1c. **Fix config service_test.go**
    - Skill: @test-writer
    - Files: internal/services/config/config_service_test.go
    - Critical: no
    - Depends on: none
    - User decision: no
    - Sandbox: worker-c
    - Errors: mockKVStorage missing Upsert method

1d. **Fix badger job_storage_test.go**
    - Skill: @test-writer
    - Files: internal/storage/badger/job_storage_test.go
    - Critical: no
    - Depends on: none
    - User decision: no
    - Sandbox: worker-d
    - Errors: NewJobStorage undefined

### Group 2 (Sequential - Verification)
Runs after Group 1 completes:

2. **Run full test suite**
   - Skill: @test-writer
   - Files: all
   - Critical: no
   - Depends on: 1a, 1b, 1c, 1d
   - User decision: no

## Parallel Execution Map
```
[Step 1a: crawler/logging_test.go] ──┐
[Step 1b: logs/service_test.go]     ──┼──> [Step 2: Run tests]
[Step 1c: config/service_test.go]   ──┤
[Step 1d: badger/job_storage_test]  ──┘
```

## Success Criteria
- All test files compile
- All tests pass (or document pre-existing failures)
- No references to models.Job or models.JobModel remain
