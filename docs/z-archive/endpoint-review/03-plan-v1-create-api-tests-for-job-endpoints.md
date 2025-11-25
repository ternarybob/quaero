I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The codebase has well-established test patterns in `test/api/health_check_test.go` and `test/api/settings_system_test.go` that use `common.SetupTestEnvironment()` with Badger config for isolated test environments. The job handlers (`job_handler.go` and `job_definition_handler.go`) expose 24 endpoints covering job management (12 endpoints) and job definitions (12 endpoints). Job models include `Job` (runtime state) and `JobDefinition` (configuration), with complex structures including steps, config maps, metadata, and child job relationships. The handlers support advanced features like grouped job listings, aggregated logs, TOML export/import, validation, and quick crawl for Chrome extension integration. Test coverage should include pagination, filtering, job lifecycle transitions, parent/child job relationships, TOML workflows, and error handling for validation failures and missing resources.

### Approach

Create a comprehensive API test file `test/api/jobs_test.go` that covers all job management and job definition endpoints. The file will follow the established test patterns from `health_check_test.go` and `settings_system_test.go`, using `common.SetupTestEnvironment()` for test isolation and `HTTPTestHelper` for HTTP requests. Tests will be organized into logical groups with helper functions for common operations (create/delete jobs and job definitions), step-by-step logging for clarity, and comprehensive assertions covering success cases, validation errors, and edge cases. The implementation will test the complete job lifecycle including creation, execution, monitoring, cancellation, and cleanup, as well as job definition CRUD operations, TOML workflows, and validation.

### Reasoning

Reviewed the test template (`health_check_test.go`), comprehensive settings/system tests (`settings_system_test.go`), test setup utilities (`setup.go`), and both job handler files (`job_handler.go`, `job_definition_handler.go`). Examined job models (`job_model.go`, `job_definition.go`) to understand data structures and validation rules. Analyzed handler implementations to identify request/response formats, query parameters, and error cases. Confirmed test config files are available and the test infrastructure supports isolated environments with Badger storage.

## Mermaid Diagram

sequenceDiagram
    participant Test as Test Function
    participant Env as TestEnvironment
    participant Helper as HTTPTestHelper
    participant API as Quaero API
    participant DB as SQLite Database

    Note over Test,DB: Test Setup Phase
    Test->>Env: SetupTestEnvironment(name, config)
    Env->>Env: Build service binary
    Env->>Env: Start test server (port 18085)
    Env->>API: Wait for /api/health
    API-->>Env: 200 OK
    Env-->>Test: Environment ready
    Test->>Helper: NewHTTPTestHelper(t)

    Note over Test,DB: Job Definition Tests
    Test->>Helper: POST /api/job-definitions (create)
    Helper->>API: HTTP POST with job definition JSON
    API->>DB: INSERT job_definition
    DB-->>API: Success
    API-->>Helper: 201 Created + job definition
    Helper-->>Test: Response + assertions

    Test->>Helper: GET /api/job-definitions (list)
    Helper->>API: HTTP GET with filters
    API->>DB: SELECT with WHERE/LIMIT/OFFSET
    DB-->>API: Job definitions array
    API-->>Helper: 200 OK + paginated results
    Helper-->>Test: Response + assertions

    Test->>Helper: POST /api/job-definitions/{id}/execute
    Helper->>API: HTTP POST
    API->>DB: INSERT job (pending status)
    API->>API: Enqueue to job queue
    API-->>Helper: 202 Accepted + job_id
    Helper-->>Test: Response + job_id

    Note over Test,DB: Job Management Tests
    Test->>Helper: GET /api/jobs/{id} (poll status)
    Helper->>API: HTTP GET
    API->>DB: SELECT job by ID
    DB-->>API: Job with status
    API-->>Helper: 200 OK + job details
    Helper-->>Test: Response + status check

    Test->>Helper: GET /api/jobs/{id}/logs
    Helper->>API: HTTP GET with filters
    API->>DB: SELECT logs WHERE job_id
    DB-->>API: Logs array
    API-->>Helper: 200 OK + logs
    Helper-->>Test: Response + assertions

    Test->>Helper: POST /api/jobs/{id}/cancel
    Helper->>API: HTTP POST
    API->>DB: UPDATE job SET status=cancelled
    DB-->>API: Success
    API-->>Helper: 200 OK
    Helper-->>Test: Response + assertions

    Test->>Helper: DELETE /api/jobs/{id}
    Helper->>API: HTTP DELETE
    API->>DB: DELETE job WHERE id
    DB-->>API: Success
    API-->>Helper: 204 No Content
    Helper-->>Test: Response + assertions

    Note over Test,DB: TOML Workflow Tests
    Test->>Helper: POST /api/job-definitions/validate
    Helper->>API: HTTP POST with TOML content
    API->>API: Parse and validate TOML
    API-->>Helper: 200 OK + validation result
    Helper-->>Test: Response + assertions

    Test->>Helper: POST /api/job-definitions/upload
    Helper->>API: HTTP POST with TOML file
    API->>API: Parse TOML to JobDefinition
    API->>DB: INSERT job_definition
    DB-->>API: Success
    API-->>Helper: 201 Created
    Helper-->>Test: Response + assertions

    Test->>Helper: GET /api/job-definitions/{id}/export
    Helper->>API: HTTP GET
    API->>DB: SELECT job_definition
    DB-->>API: Job definition
    API->>API: Convert to TOML format
    API-->>Helper: 200 OK + TOML file
    Helper-->>Test: Response + TOML content

    Note over Test,DB: Cleanup Phase
    Test->>Env: Cleanup()
    Env->>API: Shutdown server
    Env->>Env: Close log files
    Env-->>Test: Cleanup complete

## Proposed File Changes

### test\api\jobs_test.go(NEW)

References: 

- test\api\health_check_test.go
- test\api\settings_system_test.go
- test\common\setup.go
- internal\handlers\job_handler.go
- internal\handlers\job_definition_handler.go
- internal\models\job_model.go
- internal\models\job_definition.go

Create comprehensive API tests for job management and job definition endpoints.

**File Structure:**
1. Package declaration and imports (testing, assert/require, common test helpers)
2. Helper functions section for common operations
3. Job Management tests (12 test functions)
4. Job Definition tests (12 test functions)

**Helper Functions:**
- `createTestJobDefinition(t, helper, id, name, jobType)` - Creates a minimal valid job definition and returns its ID
- `deleteJobDefinition(t, helper, id)` - Deletes a job definition
- `executeJobDefinition(t, helper, id)` - Executes a job definition and returns the job ID
- `waitForJobCompletion(t, helper, jobID, timeout)` - Polls job status until terminal state or timeout
- `createTestJob(t, helper)` - Creates a test job via job definition execution and returns job ID
- `deleteJob(t, helper, jobID)` - Deletes a job

**Job Management Tests:**

1. `TestJobManagement_ListJobs` - Test GET /api/jobs with pagination, filtering (status, source, entity), ordering, and grouped mode. Verify response structure includes jobs array, total_count, limit, offset. Test empty results, single page, multiple pages.

2. `TestJobManagement_GetJob` - Test GET /api/jobs/{id} for valid job ID. Verify response includes all job fields (id, name, type, status, config, metadata, timestamps, progress). Test 404 for nonexistent job, 400 for empty ID.

3. `TestJobManagement_JobStats` - Test GET /api/jobs/stats. Verify response includes counts by status (pending, running, completed, failed, cancelled). Create jobs in different states and verify counts update correctly.

4. `TestJobManagement_JobQueue` - Test GET /api/jobs/queue. Verify response includes only pending and running jobs. Create jobs in various states and verify only active jobs appear in queue.

5. `TestJobManagement_JobLogs` - Test GET /api/jobs/{id}/logs with level filtering (info, warn, error) and ordering. Verify response includes logs array with timestamp, level, message fields. Test empty logs, filtered logs, pagination.

6. `TestJobManagement_AggregatedLogs` - Test GET /api/jobs/{id}/logs/aggregated for parent jobs. Verify response includes logs from parent and all children, with job_id and job_name enrichment. Test pagination and ordering.

7. `TestJobManagement_RerunJob` - Test POST /api/jobs/{id}/rerun. Create completed job, rerun it, verify new job created with same config. Test 400 for running job (cannot rerun active job), 404 for nonexistent job.

8. `TestJobManagement_CancelJob` - Test POST /api/jobs/{id}/cancel. Create running job, cancel it, verify status changes to cancelled. Test 400 for already completed job, 404 for nonexistent job.

9. `TestJobManagement_CopyJob` - Test POST /api/jobs/{id}/copy. Create job, copy it, verify new job created with different ID but same config. Test 404 for nonexistent job.

10. `TestJobManagement_DeleteJob` - Test DELETE /api/jobs/{id}. Create job, delete it, verify 204 response and job no longer exists. Test 400 for running job (cannot delete active job), 404 for nonexistent job.

11. `TestJobManagement_JobResults` - Test GET /api/jobs/{id}/results for completed job. Verify response includes results array with documents/URLs processed. Test empty results, 404 for nonexistent job.

12. `TestJobManagement_JobLifecycle` - Integration test covering full lifecycle: create job via job definition → monitor progress → verify running status → wait for completion → verify completed status → check logs → get results → rerun → cancel → delete. Verify state transitions and data consistency throughout.

**Job Definition Tests:**

1. `TestJobDefinition_List` - Test GET /api/job-definitions with pagination (limit, offset), filtering (type, enabled), and ordering (order_by, order_dir). Verify response structure includes job_definitions array, total_count, limit, offset. Test empty results, filtering by type (crawler, summarizer, custom, places), filtering by enabled (true/false).

2. `TestJobDefinition_Create` - Test POST /api/job-definitions with valid job definition. Verify 201 response and created job definition returned. Test validation errors: missing ID (400), missing name (400), missing type (400), empty steps array (400), invalid type (400), invalid source_type (400), invalid cron schedule (400), invalid timeout duration (400).

3. `TestJobDefinition_Get` - Test GET /api/job-definitions/{id} for valid ID. Verify response includes all fields (id, name, type, description, source_type, base_url, auth_id, steps, schedule, timeout, enabled, config, tags, timestamps). Test 404 for nonexistent ID, 400 for empty ID.

4. `TestJobDefinition_Update` - Test PUT /api/job-definitions/{id}. Create job definition, update name/description/steps, verify changes persisted. Test 404 for nonexistent ID, 403 for system job (cannot edit), validation errors for invalid updates.

5. `TestJobDefinition_Delete` - Test DELETE /api/job-definitions/{id}. Create job definition, delete it, verify 204 response and no longer exists. Test 404 for nonexistent ID, 403 for system job (cannot delete).

6. `TestJobDefinition_Execute` - Test POST /api/job-definitions/{id}/execute. Create job definition, execute it, verify 202 response with job_id. Poll job status until completion. Test 404 for nonexistent ID, 400 for disabled job definition.

7. `TestJobDefinition_Export` - Test GET /api/job-definitions/{id}/export. Create crawler job definition, export as TOML, verify Content-Type is text/plain and Content-Disposition header includes filename. Parse TOML and verify structure matches job definition. Test 404 for nonexistent ID, 400 for non-crawler type (export only supports crawler).

8. `TestJobDefinition_Status` - Test GET /api/job-definitions/{id}/status. Execute job definition, get status, verify response includes job tree with parent and children. Test 404 for nonexistent job ID.

9. `TestJobDefinition_ValidateTOML` - Test POST /api/job-definitions/validate with valid and invalid TOML. Valid TOML should return 200 with validation_status=valid. Invalid TOML should return 400 with validation_status=invalid and error message. Test missing required fields, invalid cron syntax, invalid types.

10. `TestJobDefinition_UploadTOML` - Test POST /api/job-definitions/upload with valid TOML file. Verify job definition created from TOML. Test validation errors for invalid TOML, 409 for duplicate ID (system job conflict).

11. `TestJobDefinition_SaveInvalidTOML` - Test POST /api/job-definitions/save-invalid with invalid TOML. Verify job definition saved with validation_status=invalid. This endpoint is for testing/debugging only and bypasses validation.

12. `TestJobDefinition_QuickCrawl` - Test POST /api/job-definitions/quick-crawl (Chrome extension integration). Send request with url, auth cookies, and optional overrides (max_depth, max_pages). Verify job definition created and executed. Test validation errors for missing URL, invalid overrides.

**Test Data:**
- Use minimal valid job definitions with required fields only (id, name, type, steps)
- Crawler job definition example: type=crawler, source_type=web, single crawl step with start_urls
- Summarizer job definition example: type=summarizer, single summarize step
- Use realistic but simple config values (max_depth=2, max_pages=10, concurrency=1)
- Test with various job statuses: pending, running, completed, failed, cancelled

**Assertions:**
- Status codes: 200 OK, 201 Created, 204 No Content, 400 Bad Request, 403 Forbidden, 404 Not Found, 409 Conflict
- Response structure: Verify all expected fields present and correct types
- Data consistency: Verify created/updated data matches request
- Pagination: Verify limit/offset/total_count calculations
- Filtering: Verify only matching records returned
- State transitions: Verify job status changes correctly (pending → running → completed)
- Error messages: Verify error responses include descriptive messages

**Test Environment:**
- Use `common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")` for isolated test environment
- Each test function gets its own environment with cleanup via `defer env.Cleanup()`
- Use `env.NewHTTPTestHelper(t)` for HTTP requests
- Use `t.Log()` for step-by-step logging
- Use `require` for critical assertions (must pass), `assert` for non-critical (should pass)

**Edge Cases:**
- Empty query parameters (should use defaults)
- Invalid pagination values (negative offset, zero limit)
- Nonexistent IDs (404 responses)
- Invalid JSON payloads (400 responses)
- Concurrent operations (create/delete race conditions)
- Parent/child job relationships (verify child stats aggregation)
- System vs user jobs (verify system jobs cannot be edited/deleted)

**Integration with Existing Tests:**
- Follow same patterns as `settings_system_test.go` (helper functions, step logging, cleanup)
- Use same test config file (`test-quaero-badger.toml`)
- Reuse `HTTPTestHelper` methods (GET, POST, PUT, DELETE, AssertStatusCode, ParseJSONResponse)
- Maintain consistency with existing test naming conventions (TestGroupName_SpecificTest)