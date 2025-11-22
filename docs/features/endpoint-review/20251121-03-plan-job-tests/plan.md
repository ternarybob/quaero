# Plan: Create API Tests for Job Management and Job Definition Endpoints

## Overview
Create comprehensive API tests in `test/api/jobs_test.go` covering all 24 job-related endpoints (12 job management + 12 job definitions). Tests will follow established patterns from `health_check_test.go` and `settings_system_test.go`, using `SetupTestEnvironment()` for isolation and `HTTPTestHelper` for requests. Implementation will cover complete job lifecycle, TOML workflows, validation, pagination, filtering, parent/child relationships, and comprehensive error handling.

## Steps

1. **Create jobs_test.go with helper functions and Job Management tests (6 functions)**
   - Skill: @test-writer
   - Files: `test/api/jobs_test.go` (NEW)
   - User decision: no
   - Implement helper functions: createTestJobDefinition, deleteJobDefinition, executeJobDefinition, waitForJobCompletion, createTestJob, deleteJob
   - Implement tests: TestJobManagement_ListJobs, TestJobManagement_GetJob, TestJobManagement_JobStats, TestJobManagement_JobQueue, TestJobManagement_JobLogs, TestJobManagement_AggregatedLogs
   - Test pagination, filtering (status/source/entity), ordering, grouped mode, log filtering, parent/child aggregation

2. **Add remaining Job Management tests (6 functions)**
   - Skill: @test-writer
   - Files: `test/api/jobs_test.go` (EDIT)
   - User decision: no
   - Implement tests: TestJobManagement_RerunJob, TestJobManagement_CancelJob, TestJobManagement_CopyJob, TestJobManagement_DeleteJob, TestJobManagement_JobResults, TestJobManagement_JobLifecycle
   - Test job operations (rerun/cancel/copy/delete), results retrieval, full lifecycle integration test
   - Verify state transitions, validation errors (cannot cancel completed, cannot delete running)

3. **Add Job Definition CRUD tests (6 functions)**
   - Skill: @test-writer
   - Files: `test/api/jobs_test.go` (EDIT)
   - User decision: no
   - Implement tests: TestJobDefinition_List, TestJobDefinition_Create, TestJobDefinition_Get, TestJobDefinition_Update, TestJobDefinition_Delete, TestJobDefinition_Execute
   - Test pagination, filtering (type/enabled), ordering, validation errors (missing fields, invalid types)
   - Verify system job protection (403 for edit/delete), execution workflow (202 Accepted + job_id)

4. **Add Job Definition TOML workflow tests (6 functions)**
   - Skill: @test-writer
   - Files: `test/api/jobs_test.go` (EDIT)
   - User decision: no
   - Implement tests: TestJobDefinition_Export, TestJobDefinition_Status, TestJobDefinition_ValidateTOML, TestJobDefinition_UploadTOML, TestJobDefinition_SaveInvalidTOML, TestJobDefinition_QuickCrawl
   - Test TOML export/import, validation, quick crawl (Chrome extension), job status tree
   - Verify Content-Type headers, TOML parsing, validation error messages

5. **Compile and verify test suite completeness**
   - Skill: @test-writer
   - Files: `test/api/jobs_test.go` (VERIFY)
   - User decision: no
   - Verify all 24 test functions implemented (12 job management + 12 job definitions)
   - Verify all 6 helper functions implemented
   - Compile test suite: `cd test/api && go test -c -o /tmp/jobs_test.exe`
   - Count functions and verify completeness

## Success Criteria
- File `test/api/jobs_test.go` created with all 24 test functions and 6 helper functions
- All tests follow health_check_test.go and settings_system_test.go patterns
- Tests use SetupTestEnvironment() with Badger config for isolation
- Tests cover all job management endpoints (list, get, stats, queue, logs, aggregated logs, rerun, cancel, copy, delete, results, lifecycle)
- Tests cover all job definition endpoints (list, create, get, update, delete, execute, export, status, validate, upload, save invalid, quick crawl)
- Helper functions reduce code duplication (create/delete jobs and job definitions, execute, wait for completion)
- Comprehensive validation error testing (missing fields, invalid types, system job protection)
- Pagination and filtering tested (limit/offset/total_count, status/type/enabled filters)
- TOML workflows tested (export, validate, upload with parsing)
- Parent/child job relationships tested (aggregated logs, status tree)
- All tests compile successfully with `go test -c`
- Job lifecycle integration test validates complete workflow from creation to cleanup
