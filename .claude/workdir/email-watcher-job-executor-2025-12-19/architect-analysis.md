# ARCHITECT ANALYSIS: Email Watcher Job Executor

## Task Summary
Create a worker and job that:
- Runs every 2 minutes (cron schedule)
- Reads emails via IMAP
- Filters for subject containing 'quaero'
- Extracts job name from email body
- Executes the specified job definition

## Existing Code Analysis

### 1. Email Infrastructure
**FOUND**: `internal/queue/workers/email_worker.go`
- **Purpose**: SENDS emails (SMTP outbound only)
- **NOT USABLE**: No IMAP reading capability exists
- **Verdict**: Cannot extend - completely different purpose

**FOUND**: `internal/services/mailer/service.go`
- **Purpose**: SMTP email sending service
- **Configuration**: Uses KeyValue storage for SMTP credentials
- **NOT USABLE**: No IMAP implementation
- **Verdict**: Pattern can be followed for IMAP config storage

### 2. Worker Pattern Analysis
**EXISTING WORKERS** (20+ workers examined):
- `asx_stock_data_worker.go` - Excellent reference for scheduled data fetching
- `email_worker.go` - Reference for email integration patterns
- All implement `interfaces.DefinitionWorker` interface with:
  - `GetType() models.WorkerType`
  - `Init(ctx, step, jobDef) (*WorkerInitResult, error)`
  - `CreateJobs(ctx, step, jobDef, stepID, initResult) (string, error)`
  - `ReturnsChildJobs() bool`
  - `ValidateConfig(step) error`

### 3. Job Scheduling
**FOUND**: `internal/services/scheduler/scheduler_service.go`
- Uses `robfig/cron` for scheduling
- `RegisterJob(name, schedule, description, autoStart, handler)` pattern
- Schedule format: Cron expressions (e.g., `*/2 * * * *` for every 2 minutes)
- **Job Execution**: Orchestrator's `ExecuteJobDefinition(ctx, jobDef, jobMonitor, stepMonitor)` method

### 4. Job Definition Storage
**FOUND**: `internal/models/job_definition.go`
- Jobs are stored in `JobDefinitionStorage`
- Jobs can be looked up by ID or name
- Jobs have `Schedule` field for cron expression
- Jobs can be `Enabled` or disabled

### 5. Worker Registration
**PATTERN FOUND** in `internal/app/app.go`:
- Workers are registered in `StepManager` with their `WorkerType`
- New `WorkerType` constants must be added to `internal/models/worker_type.go`

## ARCHITECTURE DECISION

### What CANNOT Be Extended
❌ **Email Worker**: Completely different purpose (sends emails, not reads)
❌ **Mailer Service**: SMTP only, no IMAP

### What MUST Be Created (Justification)
1. **NEW**: IMAP Email Reader Service (`internal/services/imap/service.go`)
   - **Why**: No IMAP capability exists anywhere in codebase
   - **Pattern**: Follow `mailer.Service` pattern for config storage in KeyValue
   - **Config Keys**: `imap_host`, `imap_port`, `imap_username`, `imap_password`, `imap_use_tls`

2. **NEW**: Email Watcher Worker (`internal/queue/workers/email_watcher_worker.go`)
   - **Why**: No worker monitors emails for job execution triggers
   - **Pattern**: Follow `asx_stock_data_worker.go` for scheduled execution pattern
   - **Worker Type**: `WorkerTypeEmailWatcher` (new constant)

3. **MODIFY**: Worker Type Constants (`internal/models/worker_type.go`)
   - Add `WorkerTypeEmailWatcher WorkerType = "email_watcher"`
   - Add to `IsValid()` and `AllWorkerTypes()` functions

4. **MODIFY**: Worker Registration (`internal/app/app.go`)
   - Register new worker in StepManager

## Implementation Plan

### Phase 1: IMAP Service
**File**: `internal/services/imap/service.go`
- Create struct with `kvStorage`, `logger` (DI pattern)
- Config methods: `GetConfig()`, `SetConfig()`, `IsConfigured()`
- Email reading: `FetchUnreadEmails(ctx, subjectFilter)` returns `[]Email`
- Email struct: `{ID, From, Subject, Body, Date}`
- Use `emersion/go-imap` library (standard Go IMAP client)

### Phase 2: Email Watcher Worker
**File**: `internal/queue/workers/email_watcher_worker.go`
- Dependencies: `imapService`, `jobDefStorage`, `orchestrator`, `logger`, `jobMgr`
- `Init()`: Validate IMAP is configured, return single work item
- `CreateJobs()`:
  1. Fetch unread emails with subject containing 'quaero'
  2. Parse email body for job name (format: `execute: <job-name>`)
  3. Look up job definition by name
  4. Call `orchestrator.ExecuteJobDefinition(ctx, jobDef, nil, nil)`
  5. Mark email as read
- `ReturnsChildJobs()`: false (synchronous execution)

### Phase 3: Worker Type Registration
**File**: `internal/models/worker_type.go`
- Add constant: `WorkerTypeEmailWatcher WorkerType = "email_watcher"`
- Update `IsValid()` switch
- Update `AllWorkerTypes()` slice

### Phase 4: Worker Registration
**File**: `internal/app/app.go`
- Create IMAP service instance
- Create EmailWatcherWorker instance with dependencies
- Register in StepManager: `stepMgr.RegisterWorker(models.WorkerTypeEmailWatcher, emailWatcherWorker)`

### Phase 5: Job Definition
**Manual Step**: Create job definition via UI/API with:
- Name: "Email Job Watcher"
- Type: "job_definition"
- Schedule: `*/2 * * * *` (every 2 minutes)
- Enabled: true
- Steps: Single step with type `email_watcher`

## Dependencies Required
- `github.com/emersion/go-imap` - IMAP client library
- `github.com/emersion/go-message` - Email parsing (if needed)

## Risk Analysis
- **Low Risk**: Following established patterns exactly
- **No Breaking Changes**: All new code, no modifications to existing workers
- **Testable**: Can test IMAP service independently
- **Configurable**: Uses KV storage like existing email service

## Build Compliance
- Uses build scripts (not direct `go build`)
- All code follows Go patterns from skill document
- Constructor injection for all dependencies
- Arbor logging throughout
- Proper error wrapping with context

## ANTI-CREATION VIOLATIONS: NONE
✅ Searched exhaustively for IMAP functionality - DOES NOT EXIST
✅ Email worker serves different purpose (sending vs reading)
✅ Following exact patterns from similar workers (ASX stock data)
✅ Minimum viable implementation (no over-engineering)

## ARCHITECT VERDICT
**CREATE NEW CODE** - Justified and necessary. No existing code can be extended for this purpose.
