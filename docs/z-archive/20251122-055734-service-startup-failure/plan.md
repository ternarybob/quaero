# Service Startup Failure Diagnostic Plan

**Date:** 2025-11-22
**Status:** Investigation
**Priority:** Critical

## Problem Statement

Service builds successfully with `./scripts/build.ps1 -run` but fails immediately after starting with no error output in logs. The log file `bin\logs\quaero.2025-11-21T18-56-20.log` exists but shows service stopping after "System logs service initialized" without error messages.

## Root Cause Analysis

Based on log file examination:
- Last successful log entry: `time=18:56:21 level=INF message="System logs service initialized"`
- This occurs at line 345 in `internal/app/app.go::initServices()`
- Next code execution point: Queue manager initialization (line 356)
- **Critical Finding:** Application crashes before reaching BadgerDB queue initialization at line 356-366

Possible causes identified:
1. **BadgerDB assertion failure** - Line 351 casts `StorageManager.DB()` to `*badger.DB` without proper error handling
2. **Panic during type assertion** - If the assertion fails, application panics before reaching error logging
3. **Missing panic recovery** - main.go goroutine has panic recovery (line 152-156) but main initialization does not
4. **Configuration mismatch** - `bin/quaero.toml` contains legacy SQLite config `[storage.sqlite]` (line 43-45) despite Badger-only migration

## Investigation Steps

### Step 1: Verify BadgerDB Type Assertion Safety
**@skill:** go-coder
**User decision:** no

**Objective:** Determine if the type assertion at line 351 is causing a panic

**Actions:**
1. Review `internal/app/app.go` lines 348-366 (BadgerDB initialization)
2. Check if `StorageManager.DB()` returns correct type for Badger backend
3. Examine `internal/storage/badger/manager.go::DB()` implementation
4. Verify `internal/storage/factory.go` always creates Badger manager

**Expected Findings:**
- Type assertion should succeed if StorageManager is BadgerManager
- Need to verify StorageManager.DB() returns `*badger.DB` not `*badgerhold.Store`

**Success Criteria:**
- Identify if type assertion is safe
- Document actual vs expected type returned by DB()

---

### Step 2: Add Panic Recovery to Application Initialization
**@skill:** go-coder
**User decision:** no

**Objective:** Catch and log panics during app.New() initialization

**Actions:**
1. Add defer/recover block in `cmd/quaero/main.go` before `app.New()` call (line 137)
2. Wrap app.New() in a function with panic recovery
3. Log panic details including stack trace to both stdout and log file
4. Ensure logger is initialized before recovery attempt

**Implementation:**
```go
// Add after line 136 in main.go
func initializeApp(config *common.Config, logger arbor.ILogger) (application *app.App, err error) {
    defer func() {
        if r := recover(); r != nil {
            logger.Fatal().
                Str("panic", fmt.Sprintf("%v", r)).
                Str("stack", string(debug.Stack())).
                Msg("Application initialization panicked")
            err = fmt.Errorf("initialization panic: %v", r)
        }
    }()

    application, err = app.New(config, logger)
    return application, err
}
```

**Success Criteria:**
- Panic messages appear in log file with full stack trace
- User sees descriptive error before application exits

---

### Step 3: Fix BadgerDB Type Assertion in Queue Manager Initialization
**@skill:** go-coder
**User decision:** no

**Objective:** Make BadgerDB retrieval type-safe with proper error handling

**Actions:**
1. Review `internal/storage/badger/manager.go::DB()` (lines 77-82)
2. Verify it returns `*badgerhold.Store` wrapped in interface{}, not `*badger.DB`
3. Update `internal/app/app.go` line 351-354 to:
   - First assert to `*badgerhold.Store`
   - Extract underlying `*badger.DB` via BadgerDB() method
   - Add proper error handling instead of blind assertion

**Implementation:**
```go
// Replace lines 351-354 in app.go
badgerStore, ok := a.StorageManager.DB().(*badgerhold.Store)
if !ok {
    return fmt.Errorf("storage manager is not backed by BadgerDB (got %T)", a.StorageManager.DB())
}

// Extract underlying badger.DB from badgerhold.Store
badgerDB := badgerStore.Badger()

queueMgr, err := queue.NewBadgerManager(
    badgerDB,
    a.Config.Queue.QueueName,
    parseDuration(a.Config.Queue.VisibilityTimeout),
    a.Config.Queue.MaxReceive,
)
```

**Success Criteria:**
- Type assertion uses proper error handling
- Application logs descriptive error if wrong type
- Queue manager receives correct `*badger.DB` instance

---

### Step 4: Update Configuration File for Badger-Only Storage
**@skill:** none
**User decision:** no

**Objective:** Remove legacy SQLite configuration from deployed config file

**Actions:**
1. Update `deployments/local/quaero.toml` to remove `[storage.sqlite]` section
2. Add `[storage.badger]` section with proper defaults
3. Update build script to warn if deployed config contains SQLite references
4. Add migration notes to README

**Implementation:**
```toml
# Remove lines 43-45:
# [storage.sqlite]
# reset_on_startup = true
# path = "./data/quaero.db"

# Add instead:
[storage.badger]
# path = "./data/quaero.badger"  # Default: ./data/quaero.badger
```

**Success Criteria:**
- Deployed config file contains no SQLite references
- BadgerDB path is explicitly documented
- Build warnings appear if legacy config detected

---

### Step 5: Verify Queue Manager Initialization After Fix
**@skill:** test-writer
**User decision:** no

**Objective:** Ensure queue manager initializes successfully with BadgerDB

**Actions:**
1. Run `./scripts/build.ps1 -run` after applying fixes
2. Verify log file contains "Queue manager initialized" message
3. Check that application reaches server startup ("Server ready")
4. Test basic queue operations via health check endpoint

**Test Cases:**
1. Application starts successfully
2. Log file shows queue initialization: `"Queue manager initialized" queue_name="quaero_jobs"`
3. Server responds to health check at `http://localhost:8085/api/health`
4. No panic messages in logs

**Success Criteria:**
- Application starts and reaches "Server ready" state
- All initialization steps complete without panics
- Log file shows complete initialization sequence

---

### Step 6: Add Startup Self-Diagnostic Logging
**@skill:** go-coder
**User decision:** no

**Objective:** Add detailed logging to catch future startup failures

**Actions:**
1. Add checkpoint logging at each major initialization step in `app.go`
2. Log type information for critical assertions
3. Add timing metrics for slow initialization steps
4. Include system state in crash reports

**Implementation:**
```go
// Add before line 351 in app.go::initServices()
a.Logger.Debug().
    Str("storage_type", fmt.Sprintf("%T", a.StorageManager)).
    Str("db_type", fmt.Sprintf("%T", a.StorageManager.DB())).
    Msg("Initializing queue manager - verifying storage compatibility")

// Add after successful queue initialization
a.Logger.Debug().
    Dur("init_duration", time.Since(startTime)).
    Msg("Queue manager initialization completed")
```

**Success Criteria:**
- Each initialization phase has clear start/complete log entries
- Type mismatches are logged before they cause panics
- Performance bottlenecks during startup are visible

---

## Success Criteria (Overall)

1. **Immediate:** Application starts successfully without panics
2. **Short-term:** Clear error messages appear in logs for any initialization failures
3. **Long-term:** Self-diagnostic logging catches future startup issues before they cause silent failures

## Risk Assessment

- **Low Risk:** Adding panic recovery and logging (Steps 2, 6)
- **Medium Risk:** Fixing type assertions (Step 3) - requires careful testing
- **Low Risk:** Config file updates (Step 4) - backward compatible
- **Low Risk:** Verification testing (Step 5)

## Rollback Plan

If fixes cause regressions:
1. Revert to commit `6b38bf2` (last known working state with SQLite)
2. Document BadgerDB migration blockers
3. Create interim fix to support both SQLite and Badger during transition

## Dependencies

- All steps depend on Step 1 findings
- Step 3 must complete before Step 5
- Step 2 should be implemented first for better diagnostics

## Timeline

- Step 1-2: 30 minutes (diagnosis + panic recovery)
- Step 3: 1 hour (fix type assertion + testing)
- Step 4: 15 minutes (config updates)
- Step 5: 30 minutes (verification testing)
- Step 6: 45 minutes (enhanced logging)

**Total Estimated Time:** 3 hours
