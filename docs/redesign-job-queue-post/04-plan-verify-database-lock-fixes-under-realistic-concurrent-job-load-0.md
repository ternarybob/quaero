I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Key Findings:**

1. **Test Infrastructure Available:**
   - `LoadTestConfig()` and `InitializeTestApp()` provide full app initialization with real database
   - `test/helpers.go` provides HTTP test helpers for API interaction
   - Existing integration tests demonstrate direct database access patterns

2. **Job Creation Flow:**
   - Job definitions execute via `POST /api/job-definitions/{id}/execute`
   - Execution is asynchronous (returns execution_id immediately)
   - Parent jobs spawn child crawler_url jobs via queue messages
   - Worker pool (2 workers after our changes) processes queue messages

3. **Validation Requirements:**
   - **SQLITE_BUSY errors**: Must parse log files or capture during test execution
   - **Queue message deletion**: Can query goqite table directly or monitor queue stats
   - **Job hierarchy**: Use `GetChildJobs()` and `GetJobChildStats()` from JobStorage
   - **Worker staggering**: Verify via log timestamps or queue processing metrics

4. **Test Data Generation:**
   - Need to create sources with realistic crawl configurations
   - Job definitions should have crawl actions with high concurrency settings
   - Seed URLs should be numerous (50+ per parent) to stress the system

5. **Monitoring Approach:**
   - Capture logs during test execution (redirect or read from file)
   - Query database directly for job counts and status
   - Use queue manager's `GetQueueStats()` for queue metrics
   - Track timing and throughput metrics

### Approach

Create a **comprehensive load testing framework** to validate the SQLITE_BUSY database lock fixes implemented in previous phases. The test will simulate realistic high-concurrency scenarios (10+ parent jobs with 50+ child URLs each) and provide detailed validation of:

1. **Database lock resilience** - Verify retry logic prevents SQLITE_BUSY errors
2. **Queue message lifecycle** - Confirm successful deletion after processing
3. **Job hierarchy integrity** - Validate parent-child relationships remain intact
4. **Worker pool efficiency** - Measure staggering effectiveness
5. **System performance** - Document throughput and error rates

The test will use the existing integration test infrastructure (`LoadTestConfig`, `InitializeTestApp`) and provide comprehensive metrics and documentation of findings.

### Reasoning

Explored the codebase to understand:
- Queue and worker pool architecture (`internal/queue/worker.go`, `internal/queue/manager.go`)
- Job creation and execution flow (`internal/handlers/job_definition_handler.go`, `internal/services/jobs/executor.go`)
- Test infrastructure patterns (`test/helpers.go`, `test/api/job_error_tolerance_integration_test.go`)
- Database access in tests (`test/api/job_cascade_test.go`)
- Log file structure and location (`bin/logs/quaero.*.log`)
- Job storage and hierarchy validation (`internal/storage/sqlite/job_storage.go`)

Identified that the test needs to create job definitions with crawl actions, execute them to spawn parent jobs, and monitor the resulting child job creation and processing under high concurrency.

## Mermaid Diagram

sequenceDiagram
    participant Test as Load Test
    participant App as Test App
    participant Queue as Queue Manager
    participant Workers as Worker Pool (2)
    participant DB as SQLite DB
    participant Logs as Log Files

    Note over Test: Initialize Test Environment
    Test->>App: LoadTestConfig()
    Test->>App: InitializeTestApp()
    App->>DB: Create temp database
    App->>Queue: Start queue manager
    App->>Workers: Start 2 workers (staggered)
    
    Note over Test: Create Test Data (10 parents, 50 children each)
    loop For each parent job
        Test->>App: Create source
        Test->>App: Create job definition
        Test->>App: Execute job definition
        App->>Queue: Enqueue parent job
        App->>DB: SaveJob(parent) with retry
    end
    
    Note over Workers,DB: Concurrent Job Processing
    par Worker 1 Processing
        Workers->>Queue: Receive message
        Workers->>DB: SaveJob(child) with retry
        alt SQLITE_BUSY on first attempt
            DB-->>Workers: Error: database locked
            Workers->>Workers: Wait 100ms (backoff)
            Workers->>DB: SaveJob(child) retry
            DB-->>Workers: Success
        else Success on first attempt
            DB-->>Workers: Success
        end
        Workers->>Queue: Delete message with retry
    and Worker 2 Processing (500ms stagger)
        Workers->>Queue: Receive message
        Workers->>DB: SaveJob(child) with retry
        Workers->>Queue: Delete message with retry
    end
    
    Note over Test: Monitor and Validate
    Test->>Logs: Parse for SQLITE_BUSY errors
    Logs-->>Test: Error count: 0 ✅
    Test->>DB: Query goqite table
    DB-->>Test: Pending messages: 0 ✅
    Test->>DB: GetChildJobs(parentID)
    DB-->>Test: All 50 children present ✅
    Test->>Queue: GetQueueStats()
    Queue-->>Test: Metrics collected
    
    Note over Test: Document Results
    Test->>Test: Calculate metrics
    Test->>Test: Generate report
    Test->>Test: Save to load-test-results.md

## Proposed File Changes

### test\api\job_load_test.go(NEW)

References: 

- test\api\job_error_tolerance_integration_test.go
- test\api\job_cascade_test.go
- internal\queue\worker.go
- internal\storage\sqlite\job_storage.go
- internal\queue\manager.go

**Create comprehensive load test for concurrent job processing:**

1. **Test Structure:**
   - Use table-driven test with scenarios: light load (5 parents, 20 children each), medium load (10 parents, 50 children each), heavy load (15 parents, 100 children each)
   - Each scenario validates different aspects of the database lock fixes

2. **Test Setup (similar to `job_error_tolerance_integration_test.go`):**
   - Call `LoadTestConfig(t)` to initialize test configuration
   - Call `InitializeTestApp(t, config)` to get full app with real database
   - Defer cleanup with `app.Close()`
   - Create test HTTP server using `httptest.NewServer()` to simulate crawlable URLs

3. **Test Data Generation:**
   - Create multiple sources (one per parent job) with unique IDs
   - Each source should have crawl_config with: max_depth=2, concurrency=2, follow_links=true
   - Create job definitions for each source with crawl action
   - Configure job definitions with wait_for_completion=false for async execution

4. **Job Execution:**
   - Execute all job definitions concurrently using goroutines
   - Track execution start time for each job
   - Store execution IDs for later polling

5. **Monitoring and Validation:**
   - **SQLITE_BUSY Detection**: Implement log file parser to scan for "database is locked" or "SQLITE_BUSY" errors
   - **Queue Message Deletion**: Query goqite table directly to verify message count decreases to zero
   - **Job Hierarchy Validation**: Use `GetChildJobs()` to verify all expected children exist and have correct parent_id
   - **Worker Staggering**: Parse log timestamps for worker startup messages to verify 500ms stagger (2 workers)
   - **Completion Tracking**: Poll parent jobs until all reach terminal status (completed/failed/cancelled)

6. **Metrics Collection:**
   - Total jobs created (parents + children)
   - Total execution time
   - Average job completion time
   - SQLITE_BUSY error count (should be 0 after fixes)
   - Queue message deletion success rate (should be 100%)
   - Job hierarchy integrity (should be 100%)
   - Worker utilization (measure via queue stats)

7. **Results Documentation:**
   - Log detailed metrics to test output
   - Create summary table with pass/fail criteria
   - Document any edge cases or remaining issues
   - Save results to `docs/redesign-job-queue-post/load-test-results.md`

**Helper Functions to Implement:**
- `createLoadTestSource(id, baseURL string) map[string]interface{}` - Generate source configuration
- `createLoadTestJobDefinition(id, sourceID string, childCount int) *models.JobDefinition` - Generate job definition with specified child URL count
- `parseLogFileForErrors(logPath string) ([]string, error)` - Parse log file for SQLITE_BUSY errors
- `queryGoqiteTable(db *sql.DB) (int, error)` - Query goqite table for pending message count
- `validateJobHierarchy(ctx context.Context, storage interfaces.JobStorage, parentID string, expectedChildCount int) error` - Verify parent-child relationships
- `collectQueueMetrics(ctx context.Context, queueMgr *queue.Manager) map[string]interface{}` - Gather queue statistics
- `waitForJobCompletion(ctx context.Context, storage interfaces.JobStorage, jobID string, timeout time.Duration) (*models.CrawlJob, error)` - Poll job until terminal status

**Test Assertions:**
- Assert SQLITE_BUSY error count == 0 (critical)
- Assert all queue messages deleted successfully (critical)
- Assert job hierarchy integrity == 100% (critical)
- Assert all parent jobs reach terminal status within timeout (critical)
- Assert worker staggering is effective (verify 500ms delay between workers)
- Assert throughput meets minimum threshold (e.g., 10 jobs/second)

**Refer to:**
- `test/api/job_error_tolerance_integration_test.go` for app initialization pattern
- `test/api/job_cascade_test.go` for job hierarchy validation pattern
- `internal/queue/worker.go` for understanding worker staggering implementation
- `internal/storage/sqlite/job_storage.go` for GetChildJobs() and GetJobChildStats() usage

### test\api\test_fixtures.go(NEW)

References: 

- test\api\job_error_tolerance_test.go
- internal\common\config.go
- internal\app\app.go

**Create shared test fixtures and helper functions for load testing:**

1. **Configuration Helpers:**
   - `LoadTestConfig(t *testing.T) (*common.Config, func())` - Load test configuration with cleanup function
   - `InitializeTestApp(t *testing.T, config *common.Config) *app.App` - Initialize full application with all services
   - Both functions should create temporary database file for test isolation
   - Cleanup function should remove temporary database and close all connections

2. **Job Creation Helpers:**
   - `createTestHTTPServer(t *testing.T) *httptest.Server` - Create HTTP server that returns HTML content with links
   - `createJobDefinition(maxChildFailures int, failureAction string) *models.JobDefinition` - Create job definition with error tolerance
   - `createParentJob(name string) *models.CrawlJob` - Create parent job with pending status
   - `createChildJob(id, parentID, name string) *models.CrawlJob` - Create child job with pending status
   - `createJobMessage(parentID, url string, index int, jobDefID string) *queue.JobMessage` - Create queue message for crawler job
   - `createCrawlerJob(app *app.App, t *testing.T) *types.CrawlerJob` - Create crawler job executor with dependencies

3. **Validation Helpers:**
   - `pollParentJobStatus(t *testing.T, ctx context.Context, app *app.App, jobID string, timeout time.Duration) *models.CrawlJob` - Poll job until terminal status or timeout
   - `countCancelledChildren(t *testing.T, ctx context.Context, app *app.App, parentID string) int` - Count children with cancelled status
   - `validateEventPayload(t *testing.T, event interfaces.Event, expectedJobID string, expectedThreshold int)` - Validate EventJobFailed payload structure

4. **Database Access Helpers:**
   - `getDirectDBConnection(config *common.Config) (*sql.DB, error)` - Open direct SQLite connection for queries
   - `queryJobCount(db *sql.DB, status string) (int, error)` - Query crawl_jobs table for count by status
   - `queryQueueMessageCount(db *sql.DB, queueName string) (int, error)` - Query goqite table for pending messages

5. **Log Parsing Helpers:**
   - `findLatestLogFile(logDir string) (string, error)` - Find most recent log file in directory
   - `parseLogForPattern(logPath string, pattern string) ([]string, error)` - Parse log file for regex pattern matches
   - `countLogOccurrences(logPath string, pattern string) (int, error)` - Count occurrences of pattern in log file

**Implementation Notes:**
- Use `testing.T` for logging and assertions
- All helpers should handle errors gracefully and provide clear error messages
- Database connections should be properly closed in cleanup functions
- Log parsing should handle large files efficiently (streaming, not loading entire file)
- Refer to existing test patterns in `job_error_tolerance_test.go` for consistency

**Refer to:**
- `test/api/job_error_tolerance_test.go` for existing helper function patterns (lines 300-600)
- `internal/common/config.go` for configuration structure
- `internal/app/app.go` for app initialization sequence

### docs\redesign-job-queue-post\load-test-results.md(NEW)

References: 

- docs\redesign-job-queue-3\FINAL-SUMMARY.md

**Create comprehensive documentation of load test results:**

1. **Executive Summary:**
   - Test date and environment details
   - Overall pass/fail status
   - Key findings summary (2-3 sentences)
   - Recommendation for production readiness

2. **Test Configuration:**
   - Worker pool concurrency: 2 workers
   - SQLite busy timeout: 10000ms (10 seconds)
   - Retry logic: 5 attempts with exponential backoff (100ms initial delay)
   - Test scenarios: Light (5x20), Medium (10x50), Heavy (15x100)

3. **Test Results Table:**
   - Scenario | Total Jobs | Execution Time | SQLITE_BUSY Errors | Queue Deletion Success | Hierarchy Integrity | Status
   - Include pass/fail criteria for each metric
   - Use markdown table format for readability

4. **Detailed Metrics:**
   - **Database Lock Resilience:**
     - SQLITE_BUSY error count (before fixes vs after fixes)
     - Retry success rate
     - Average retry attempts per operation
   - **Queue Message Lifecycle:**
     - Total messages enqueued
     - Total messages deleted successfully
     - Deletion failure count
     - Average message processing time
   - **Job Hierarchy Integrity:**
     - Total parent jobs created
     - Total child jobs created
     - Orphaned jobs count (children without parent)
     - Missing children count (expected vs actual)
   - **Worker Pool Performance:**
     - Worker startup stagger verification (timestamps)
     - Average queue length during test
     - Peak queue length
     - Worker utilization percentage
   - **System Throughput:**
     - Jobs per second
     - Average job completion time
     - 95th percentile completion time

5. **Observations:**
   - Describe any unexpected behavior
   - Note any warnings or non-critical issues
   - Document edge cases discovered

6. **Remaining Issues (if any):**
   - List any unresolved problems
   - Severity assessment (critical, high, medium, low)
   - Recommended mitigation strategies

7. **Recommendations:**
   - Production readiness assessment
   - Suggested configuration tuning
   - Future testing needs
   - Monitoring recommendations for production

8. **Appendix:**
   - Sample log excerpts showing successful retry logic
   - Database query results
   - Queue statistics snapshots
   - Test execution timeline

**Format:**
- Use markdown with clear headings and sections
- Include code blocks for log excerpts and queries
- Use tables for metrics comparison
- Add emoji indicators for pass/fail (✅/❌)
- Keep technical but readable for non-developers

**Refer to:**
- `docs/redesign-job-queue-3/FINAL-SUMMARY.md` for documentation style
- Test output from `job_load_test.go` for actual metrics

### test\README.md(MODIFY)

References: 

- cmd\quaero-test-runner\README.md

**Add documentation for the new load testing infrastructure:**

1. **Locate the "Test Organization" section** (should be near the top of the file):
   - Add new subsection: "### Load Tests (`test/api/job_load_test.go`)"
   - Describe purpose: "Validates database lock fixes under high-concurrency scenarios"
   - List what it tests:
     - SQLITE_BUSY error prevention with retry logic
     - Queue message deletion success rate
     - Job hierarchy integrity under concurrent operations
     - Worker pool staggering effectiveness
     - System throughput and performance metrics

2. **Add "Running Load Tests" section** after existing test running instructions:
   - Provide command to run load tests: `go test -v ./test/api -run TestJobLoad`
   - Explain that load tests take longer (5-10 minutes) due to high job volume
   - Note that results are documented in `docs/redesign-job-queue-post/load-test-results.md`
   - Recommend running load tests before major releases or after concurrency changes

3. **Add "Test Fixtures" section** explaining the new `test_fixtures.go` file:
   - Describe shared helper functions available for all tests
   - List key helpers: `LoadTestConfig`, `InitializeTestApp`, `createTestHTTPServer`, etc.
   - Explain when to use fixtures vs inline test setup
   - Provide example of using fixtures in a new test

4. **Update "Test Infrastructure" section** with load testing details:
   - Mention that load tests use real database (not mocks)
   - Explain temporary database creation and cleanup
   - Note that load tests can be run in parallel with other tests (separate DB)
   - Describe log file parsing capabilities for error detection

5. **Add "Interpreting Load Test Results" section:**
   - Explain pass/fail criteria for each metric
   - Describe what to do if load tests fail
   - Provide guidance on adjusting concurrency settings if needed
   - Link to detailed results documentation

**Refer to:**
- Existing README.md structure and style
- `cmd/quaero-test-runner/README.md` for test runner documentation patterns