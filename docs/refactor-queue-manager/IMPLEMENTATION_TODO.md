# Queue Management Redesign - Implementation Tracker

**Started:** [DATE]
**Status:** Not Started
**Current Phase:** Phase 0 - Preparation
**Last Updated:** [DATE]

---

## Implementation Status Legend

- [ ] Not Started
- [x] Completed
- [⏳] In Progress
- [⚠️] Blocked
- [❌] Failed/Rolled Back

---

## Phase 0: Preparation and Backup

### 0.1 Backup Current System
- [ ] Create database backup
  ```bash
  cp quaero.db quaero.db.backup.$(date +%Y%m%d_%H%M%S)
  ```
- [ ] Verify backup created successfully
- [ ] Create git commit with current state
  ```bash
  git commit -am "Backup before queue redesign"
  ```
- [ ] Create git tag
  ```bash
  git tag backup-before-queue-redesign-v4
  ```
- [ ] Verify tag created: `git tag -l`

### 0.2 Review and Understand Architecture
- [ ] Read complete refactor document (`01-refactor.md`)
- [ ] Review architecture diagram (lines 100-158)
- [ ] Understand data flow example (lines 160-189)
- [ ] Review database schema (lines 193-262)
- [ ] Note: goqite is the ONLY queue manager (Principle 1)

**Phase 0 Completion Criteria:**
- ✅ Backup verified
- ✅ Git history clean
- ✅ Architecture understood

---

## Phase 1: File Structure Setup

### 1.1 Create Directories
- [ ] Create `internal/queue/` directory
- [ ] Create `internal/jobs/` directory (if not exists)
- [ ] Create `docs/migrations/` directory (if not exists)
- [ ] Verify directories exist

### 1.2 Create Core Implementation Files
- [ ] Create `internal/queue/types.go`
- [ ] Create `internal/queue/manager.go`
- [ ] Create `internal/queue/worker.go`
- [ ] Create `internal/jobs/manager.go`

### 1.3 Create Executor Files
- [ ] Create `internal/services/crawler/executor.go`
- [ ] Note location for future executors (summarizer, cleanup)

### 1.4 Create Handler and UI Files
- [ ] Create `internal/handlers/job_handler.go`
- [ ] Create `pages/jobs.html`

### 1.5 Create Migration File
- [ ] Create `docs/migrations/008_redesign_job_queue.sql`

**Phase 1 Completion Criteria:**
- ✅ All directories created
- ✅ All placeholder files created
- ✅ File structure verified

---

## Phase 2: Database Migration

### 2.1 Review Migration SQL
- [ ] Review migration SQL (lines 196-240)
- [ ] Understand table structure:
  - `jobs` table (metadata only, NOT queue)
  - `job_logs` table
  - goqite will create its own table
- [ ] Understand indexes and relationships

### 2.2 Execute Migration
- [ ] Stop running application
- [ ] Backup database again (pre-migration)
- [ ] Run migration SQL:
  ```bash
  sqlite3 quaero.db < docs/migrations/008_redesign_job_queue.sql
  ```
- [ ] Verify migration success

### 2.3 Verify Database Schema
- [ ] Check `jobs` table exists
  ```sql
  .schema jobs
  ```
- [ ] Check `job_logs` table exists
  ```sql
  .schema job_logs
  ```
- [ ] Verify indexes created
  ```sql
  .indexes jobs
  ```
- [ ] Verify old tables dropped (queue_messages, queue_state, job_queue)

**Phase 2 Completion Criteria:**
- ✅ Migration executed successfully
- ✅ New tables verified
- ✅ Old tables removed
- ✅ Indexes in place

---

## Phase 3: Core Queue Implementation

### 3.1 Implement Queue Types
- [ ] Open `internal/queue/types.go`
- [ ] Implement `Message` struct (lines 270-283)
  - JobID field
  - Type field
  - Payload field (json.RawMessage)
- [ ] Add package documentation
- [ ] Build to verify compilation: `go build ./internal/queue`

### 3.2 Implement Queue Manager
- [ ] Open `internal/queue/manager.go`
- [ ] Implement `Manager` struct (lines 290-370)
- [ ] Implement `NewManager()` function
- [ ] Implement `Enqueue()` method
- [ ] Implement `Receive()` method
- [ ] Implement `Extend()` method
- [ ] Implement `Close()` method
- [ ] Add godoc comments
- [ ] Build to verify: `go build ./internal/queue`

### 3.3 Write Queue Manager Tests
- [ ] Create `internal/queue/manager_test.go`
- [ ] Implement basic enqueue test (lines 1579-1635)
- [ ] Implement receive test
- [ ] Implement delete test
- [ ] Run tests: `go test ./internal/queue`
- [ ] Verify all tests pass

**Phase 3 Completion Criteria:**
- ✅ Queue types implemented
- ✅ Queue manager implemented
- ✅ All queue tests passing
- ✅ No compilation errors

---

## Phase 4: Job Manager Implementation

### 4.1 Implement Job Manager Types
- [ ] Open `internal/jobs/manager.go`
- [ ] Implement `Job` struct (lines 405-419)
- [ ] Implement `JobLog` struct (lines 652-658)
- [ ] Implement `Manager` struct (lines 391-401)

### 4.2 Implement Job Creation Methods
- [ ] Implement `NewManager()` function
- [ ] Implement `CreateParentJob()` method (lines 422-450)
- [ ] Implement `CreateChildJob()` method (lines 452-481)
- [ ] Test compilation: `go build ./internal/jobs`

### 4.3 Implement Job Query Methods
- [ ] Implement `GetJob()` method (lines 484-523)
- [ ] Implement `ListParentJobs()` method (lines 525-542)
- [ ] Implement `ListChildJobs()` method (lines 544-560)
- [ ] Implement `scanJobs()` helper (lines 660-702)

### 4.4 Implement Job Update Methods
- [ ] Implement `UpdateJobStatus()` method (lines 562-582)
- [ ] Implement `UpdateJobProgress()` method (lines 584-591)
- [ ] Implement `SetJobError()` method (lines 593-600)
- [ ] Implement `SetJobResult()` method (lines 602-613)

### 4.5 Implement Job Logging Methods
- [ ] Implement `AddJobLog()` method (lines 615-622)
- [ ] Implement `GetJobLogs()` method (lines 624-649)

### 4.6 Write Job Manager Tests
- [ ] Create `internal/jobs/manager_test.go`
- [ ] Test parent job creation
- [ ] Test child job creation
- [ ] Test job queries
- [ ] Test job updates
- [ ] Run tests: `go test ./internal/jobs`
- [ ] Verify all tests pass

**Phase 4 Completion Criteria:**
- ✅ Job manager fully implemented
- ✅ All job manager tests passing
- ✅ CRUD operations working
- ✅ Parent/child relationships working

---

## Phase 5: Worker Pool Implementation

### 5.1 Implement Worker Pool Structure
- [ ] Open `internal/queue/worker.go`
- [ ] Implement `Executor` interface (lines 721-724)
- [ ] Implement `WorkerPool` struct (lines 726-735)
- [ ] Implement `NewWorkerPool()` function (lines 737-748)

### 5.2 Implement Worker Pool Control
- [ ] Implement `RegisterExecutor()` method (lines 750-753)
- [ ] Implement `Start()` method (lines 755-763)
- [ ] Implement `Stop()` method (lines 765-771)

### 5.3 Implement Worker Logic
- [ ] Implement `worker()` method (lines 773-788)
- [ ] Implement `processNextJob()` method (lines 790-840)
- [ ] Add error handling and logging
- [ ] Add panic recovery
- [ ] Test compilation: `go build ./internal/queue`

### 5.4 Write Worker Pool Tests
- [ ] Create `internal/queue/worker_test.go`
- [ ] Create mock executor
- [ ] Test worker pool start/stop
- [ ] Test job processing
- [ ] Test error handling
- [ ] Run tests: `go test ./internal/queue`

**Phase 5 Completion Criteria:**
- ✅ Worker pool implemented
- ✅ Worker pool tests passing
- ✅ Job processing logic working
- ✅ Error handling in place

---

## Phase 6: Executor Implementation

### 6.1 Implement Crawler Executor
- [ ] Open `internal/services/crawler/executor.go`
- [ ] Implement `Executor` struct (lines 859-862)
- [ ] Implement `CrawlerPayload` struct (lines 871-875)
- [ ] Implement `NewExecutor()` function (lines 864-869)
- [ ] Implement `Execute()` method (lines 877-922)
- [ ] Test compilation: `go build ./internal/services/crawler`

### 6.2 Test Crawler Executor
- [ ] Create `internal/services/crawler/executor_test.go`
- [ ] Test execute with mock data
- [ ] Test child job creation
- [ ] Test error handling
- [ ] Run tests: `go test ./internal/services/crawler`

### 6.3 Plan Future Executors (DO NOT IMPLEMENT YET)
- [ ] Document summarizer executor requirements
- [ ] Document cleanup executor requirements
- [ ] Document any other executor types needed

**Phase 6 Completion Criteria:**
- ✅ Crawler executor implemented
- ✅ Crawler executor tests passing
- ✅ Child job spawning working
- ✅ Future executors documented

---

## Phase 7: API Handler Implementation

### 7.1 Implement Job Handler Structure
- [ ] Open `internal/handlers/job_handler.go`
- [ ] Implement `JobHandler` struct (lines 940-942)
- [ ] Implement `NewJobHandler()` function (lines 944-946)

### 7.2 Implement API Endpoints
- [ ] Implement `ListJobs()` handler (lines 948-974)
- [ ] Implement `GetJob()` handler (lines 1013-1026)
- [ ] Implement `GetJobChildren()` handler (lines 976-989)
- [ ] Implement `GetJobLogs()` handler (lines 991-1011)
- [ ] Implement `CreateJob()` handler (lines 1028-1048)

### 7.3 Write Handler Tests
- [ ] Create `internal/handlers/job_handler_test.go`
- [ ] Test ListJobs endpoint
- [ ] Test GetJob endpoint
- [ ] Test GetJobChildren endpoint
- [ ] Test GetJobLogs endpoint
- [ ] Test CreateJob endpoint
- [ ] Run tests: `go test ./internal/handlers`

**Phase 7 Completion Criteria:**
- ✅ All API endpoints implemented
- ✅ Handler tests passing
- ✅ Proper error handling
- ✅ JSON responses working

---

## Phase 8: Router Integration

### 8.1 Update Router Configuration
- [ ] Open `internal/server/router.go`
- [ ] Add job routes (lines 1056-1070):
  - `GET /api/jobs`
  - `POST /api/jobs`
  - `GET /api/jobs/{id}`
  - `GET /api/jobs/{id}/children`
  - `GET /api/jobs/{id}/logs`
- [ ] Add jobs UI route:
  - `GET /jobs`
- [ ] Verify route registration

### 8.2 Update UI Handler
- [ ] Open `internal/handlers/ui.go`
- [ ] Add `JobsPage()` method (lines 1410-1414)
- [ ] Verify template rendering

**Phase 8 Completion Criteria:**
- ✅ All routes registered
- ✅ API endpoints accessible
- ✅ UI route accessible
- ✅ No route conflicts

---

## Phase 9: UI Implementation

### 9.1 Implement Jobs Page Template
- [ ] Open `pages/jobs.html`
- [ ] Copy template from lines 1080-1403
- [ ] Verify Bulma CSS link
- [ ] Verify Alpine.js CDN link

### 9.2 Implement UI Components
- [ ] Implement header section (lines 1103-1118)
- [ ] Implement parent jobs table (lines 1126-1200)
- [ ] Implement empty state (lines 1196-1199)
- [ ] Implement job details modal (lines 1203-1300)

### 9.3 Implement JavaScript Logic
- [ ] Implement `jobsApp()` function (lines 1305-1400)
- [ ] Implement `init()` method
- [ ] Implement `loadJobs()` method
- [ ] Implement `viewDetails()` method
- [ ] Implement `loadDetails()` method
- [ ] Implement helper functions (formatDate, formatTime, getDuration)

### 9.4 Test UI Manually
- [ ] Start application
- [ ] Navigate to http://localhost:8085/jobs
- [ ] Verify page loads without errors
- [ ] Verify table renders (even if empty)
- [ ] Verify modal opens/closes
- [ ] Check browser console for errors

**Phase 9 Completion Criteria:**
- ✅ Jobs page implemented
- ✅ Table displays correctly
- ✅ Modal works
- ✅ Auto-refresh working
- ✅ No JavaScript errors

---

## Phase 10: Application Integration

### 10.1 Update App Initialization
- [ ] Open `internal/app/app.go`
- [ ] Add queue manager initialization (lines 1478-1483)
- [ ] Add job manager initialization (lines 1485-1487)
- [ ] Add worker pool creation (lines 1489-1490)
- [ ] Register crawler executor (lines 1492-1494)
- [ ] Start worker pool (lines 1502-1503)
- [ ] Verify initialization order

### 10.2 Update App Structure
- [ ] Add `queueMgr` field to App struct
- [ ] Add `jobMgr` field to App struct
- [ ] Add `workerPool` field to App struct
- [ ] Update shutdown logic to stop worker pool

### 10.3 Update Dependencies
- [ ] Run `go mod tidy`
- [ ] Verify goqite dependency: `github.com/maragudk/goqite`
- [ ] Verify google/uuid dependency
- [ ] Resolve any dependency conflicts

**Phase 10 Completion Criteria:**
- ✅ App initializes successfully
- ✅ Queue manager created
- ✅ Job manager created
- ✅ Worker pool started
- ✅ No initialization errors

---

## Phase 11: Configuration

### 11.1 Update Configuration File
- [ ] Open `quaero.toml`
- [ ] Add `[queue]` section
- [ ] Add `num_workers = 5` setting
- [ ] Document configuration options

### 11.2 Update Config Struct
- [ ] Open `internal/common/config.go`
- [ ] Add `Queue` struct
- [ ] Add `NumWorkers` field
- [ ] Update config loading

**Phase 11 Completion Criteria:**
- ✅ Configuration updated
- ✅ Queue settings loaded
- ✅ Worker count configurable

---

## Phase 12: Old Code Cleanup

### 12.1 Identify Old Code
- [ ] Search for `queue_messages` references
  ```bash
  grep -r "queue_messages" internal/
  ```
- [ ] Search for `queue_state` references
  ```bash
  grep -r "queue_state" internal/
  ```
- [ ] Search for chevron UI code
  ```bash
  grep -r "chevron" pages/
  ```
- [ ] List all files to delete

### 12.2 Remove Old Files
- [ ] Delete old queue implementation files
- [ ] Delete old job_queue files
- [ ] Delete chevron UI components
- [ ] Verify no compilation errors

### 12.3 Remove Test URLs
- [ ] Search for hardcoded test URLs
  ```bash
  grep -r "http://localhost:3333" internal/
  grep -r "test.example.com" internal/
  ```
- [ ] Remove all hardcoded test URLs
- [ ] Replace with configuration-driven URLs

**Phase 12 Completion Criteria:**
- ✅ Old code removed
- ✅ No compilation errors
- ✅ No test URLs in production code
- ✅ Code compiles cleanly

---

## Phase 13: Testing

### 13.1 Unit Test Suite
- [ ] Run all queue tests: `go test ./internal/queue/...`
- [ ] Run all job tests: `go test ./internal/jobs/...`
- [ ] Run all handler tests: `go test ./internal/handlers/...`
- [ ] Run all executor tests: `go test ./internal/services/crawler/...`
- [ ] Verify 100% pass rate

### 13.2 Integration Tests
- [ ] Create `test/api/jobs_test.go`
- [ ] Implement job creation test (lines 1651-1690)
- [ ] Implement job retrieval test
- [ ] Implement job children test
- [ ] Implement job logs test
- [ ] Run integration tests: `go test ./test/api/...`

### 13.3 Build Verification
- [ ] Clean build: `.\scripts\build.ps1 -Clean`
- [ ] Verify build success
- [ ] Check binary created: `.\bin\quaero.exe`
- [ ] Check test runner built: `.\bin\quaero-test-runner.exe`

**Phase 13 Completion Criteria:**
- ✅ All unit tests passing
- ✅ All integration tests passing
- ✅ Build successful
- ✅ No test failures

---

## Phase 14: Smoke Testing

### 14.1 Database Verification
- [ ] Start application: `.\scripts\build.ps1 -Run`
- [ ] Check goqite table created:
  ```sql
  sqlite3 quaero.db "SELECT name FROM sqlite_master WHERE type='table' AND name='goqite_jobs';"
  ```
- [ ] Check jobs table:
  ```sql
  sqlite3 quaero.db "SELECT COUNT(*) FROM jobs;"
  ```
- [ ] Check job_logs table:
  ```sql
  sqlite3 quaero.db "SELECT COUNT(*) FROM job_logs;"
  ```

### 14.2 API Verification
- [ ] Test ListJobs endpoint:
  ```bash
  curl http://localhost:8085/api/jobs
  ```
- [ ] Verify response is valid JSON
- [ ] Verify empty array or job list returned

### 14.3 UI Verification
- [ ] Open browser to http://localhost:8085/jobs
- [ ] Verify page loads
- [ ] Verify table displays
- [ ] Click "Refresh" button
- [ ] Verify no errors in browser console

### 14.4 End-to-End Job Test
- [ ] Create a test job via API:
  ```bash
  curl -X POST http://localhost:8085/api/jobs \
    -H "Content-Type: application/json" \
    -d '{"type":"crawler","payload":{"url":"https://example.com","depth":1}}'
  ```
- [ ] Verify job appears in UI
- [ ] Verify job status changes (pending → running → completed)
- [ ] Click "View Details"
- [ ] Verify child jobs appear (if any)
- [ ] Verify logs appear
- [ ] Verify job completes successfully

**Phase 14 Completion Criteria:**
- ✅ Database tables verified
- ✅ API endpoints working
- ✅ UI functioning
- ✅ End-to-end job completes
- ✅ No critical errors

---

## Phase 15: Documentation and Finalization

### 15.1 Update Documentation
- [ ] Update README.md with new queue architecture
- [ ] Document new API endpoints
- [ ] Document job phases (pre/core/post)
- [ ] Document configuration options

### 15.2 Create Operator Guide
- [ ] Document how to monitor queue
- [ ] Document how to troubleshoot stuck jobs
- [ ] Document how to clear queue
- [ ] Document worker pool tuning

### 15.3 Final Verification
- [ ] Review checklist from lines 1693-1717
- [ ] Verify all success criteria met (lines 1760-1790)
- [ ] Test rollback procedure (don't execute)
- [ ] Document known issues

### 15.4 Create Release
- [ ] Create git commit:
  ```bash
  git add .
  git commit -m "feat(queue)!: refactor job queue architecture with goqite"
  ```
- [ ] Create git tag:
  ```bash
  git tag v0.2.0-queue-redesign
  ```
- [ ] Update `.version` file
- [ ] Push changes (if applicable)

**Phase 15 Completion Criteria:**
- ✅ Documentation complete
- ✅ All success criteria met
- ✅ Release tagged
- ✅ Ready for production

---

## Rollback Procedures

### If Phase 2 (Migration) Fails
```bash
# Restore database
cp quaero.db.backup.YYYYMMDD_HHMMSS quaero.db

# Revert code
git reset --hard backup-before-queue-redesign-v4

# Resume: Fix migration SQL, then retry Phase 2
```

### If Phase 10 (Integration) Fails
```bash
# Restore database
cp quaero.db.backup.YYYYMMDD_HHMMSS quaero.db

# Revert code
git reset --hard backup-before-queue-redesign-v4

# Rebuild old version
.\scripts\build.ps1 -Clean
.\scripts\build.ps1 -Run

# Resume: Review initialization order, fix dependencies, retry Phase 10
```

### If Phase 14 (Smoke Test) Fails
```bash
# DO NOT rollback yet - analyze first
# Check logs for specific errors
# Verify database state (see Phase 14.1)
# Fix specific issue
# Resume from failed smoke test step
```

---

## Common Issues and Resumption Points

### Issue: goqite table not found
**Phase:** 10 (App Integration)
**Fix:** Ensure `q.Setup(ctx)` called in NewManager
**Resume:** Phase 10.1

### Issue: No executor registered
**Phase:** 10 (App Integration)
**Fix:** Register executor before starting worker pool
**Resume:** Phase 10.1 (line 1492-1494)

### Issue: Jobs stuck in "running"
**Phase:** 5 (Worker Pool)
**Fix:** Add panic recovery and proper error handling
**Resume:** Phase 5.3

### Issue: UI not showing child jobs
**Phase:** 9 (UI)
**Fix:** Verify API endpoint returns data, check parent_id in database
**Resume:** Phase 9.4

### Issue: Logs not appearing
**Phase:** 6 (Executor)
**Fix:** Ensure AddJobLog() called in executor
**Resume:** Phase 6.1

---

## Progress Tracking

**Total Phases:** 15
**Completed Phases:** 0
**Current Phase:** 0 (Preparation)
**Estimated Time Remaining:** 8-12 hours

**Phase Estimates:**
- Phase 0-2: 30 minutes (Prep & Migration)
- Phase 3-5: 2-3 hours (Core Implementation)
- Phase 6-8: 2-3 hours (Executors & Handlers)
- Phase 9: 1-2 hours (UI)
- Phase 10-11: 1 hour (Integration)
- Phase 12: 30 minutes (Cleanup)
- Phase 13-14: 2-3 hours (Testing)
- Phase 15: 1 hour (Documentation)

---

## Notes and Observations

[Add notes here as you progress through implementation]

---

## Success Metrics Checklist

Final verification before marking complete:

- [ ] ✅ Queue is managed solely by goqite (no custom queue tables/logic)
- [ ] ✅ Job hierarchy works (parent/child relationships functional)
- [ ] ✅ UI is simple (no chevrons, clean modal design)
- [ ] ✅ Jobs execute successfully (proper state transitions)
- [ ] ✅ No environment pollution (no test URLs in production code)
- [ ] ✅ All tests passing
- [ ] ✅ Documentation updated
- [ ] ✅ Ready for production

---

**Implementation Guide Reference:** `docs/refactor-queue-manager/01-refactor.md`
**Database Migration:** `docs/migrations/008_redesign_job_queue.sql`
