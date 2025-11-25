I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current State Analysis:**

The codebase has a **dual storage architecture**:
- **Main storage**: Badger (default) or SQLite - used for documents, auth, KV, job definitions, job logs
- **Queue storage**: Always SQLite via goqite library - used for message queue and job metadata

**Key Dependencies:**
1. `goqite` library (maragu.dev/goqite) - hardcoded to use SQLite for persistent queue
2. `JobManager` (`internal/jobs/manager.go`) - uses `*sql.DB` directly instead of `JobStorage` interface
3. `QueueManager` (`internal/queue/manager.go`) - wraps goqite, requires `*sql.DB`
4. App initialization creates separate SQLite DB when main storage is Badger (lines 391-401 in app.go)

**Badger Implementation Status:**
- ✅ All storage interfaces implemented in `internal/storage/badger/`
- ✅ Tests already use Badger via `SetupTestEnvironment()`
- ✅ Default config uses Badger (line 197 in config.go)

**Blockers for Complete SQLite Removal:**
1. goqite library dependency (external, cannot modify)
2. JobManager's direct SQL usage (architectural debt)
3. Queue interface exposes `goqite.ID` type (line 17 in queue_service.go)

**Breaking Changes Accepted:** User explicitly stated breaking changes are acceptable, enabling architectural refactoring.


### Approach

**Strategy: Replace goqite with Badger-based queue implementation**

This plan removes ALL SQLite dependencies by implementing a Badger-backed queue to replace goqite. The approach follows these principles:

1. **Create Badger Queue Implementation** - New queue manager using Badger for persistence, matching goqite's interface
2. **Refactor JobManager** - Migrate from direct `*sql.DB` usage to `JobStorage` interface
3. **Remove SQLite Package** - Delete entire `internal/storage/sqlite/` directory
4. **Update Configuration** - Remove SQLite config section, simplify storage factory
5. **Update Tests** - Ensure all tests use Badger (already mostly done)

**Trade-offs:**
- ✅ Complete SQLite removal achieved
- ✅ Simpler architecture (single storage backend)
- ⚠️ Breaking change: Existing queue data in SQLite will be lost (migration not provided)
- ⚠️ Custom queue implementation requires thorough testing
- ⚠️ Visibility timeout and message redelivery logic must be reimplemented

**Why This Approach:**
- User explicitly requested "complete SQLite removal" with "breaking changes accepted"
- Badger storage is already fully implemented and tested
- Eliminates dual-storage complexity
- Removes external dependency on goqite
- Aligns with project's direction (default is already Badger)


### Reasoning

Started by examining the files mentioned in the task: `factory.go`, `config.go`, `app.go`, and `setup.go`. Discovered that SQLite directory exists with many implementation files. Searched for goqite usage and found it's the queue system requiring SQLite. Explored storage interfaces and found Badger implementations already exist for all storage types. Analyzed JobManager and found it uses `*sql.DB` directly instead of interfaces. Reviewed queue implementation and interface to understand what needs to be replaced. Confirmed tests already use Badger. Concluded that complete SQLite removal requires replacing goqite with Badger-based queue.


## Mermaid Diagram

sequenceDiagram
    participant Dev as Developer
    participant Queue as Queue Manager
    participant Job as Job Manager
    participant Storage as Storage Manager
    participant Badger as Badger DB

    Note over Dev,Badger: Phase 1: Create Badger Queue
    Dev->>Queue: Implement BadgerManager
    Queue->>Badger: Use Badger for persistence
    Note over Queue: Replace goqite with<br/>Badger-backed queue

    Note over Dev,Badger: Phase 2: Refactor JobManager
    Dev->>Job: Remove *sql.DB dependency
    Job->>Storage: Use JobStorage interface
    Storage->>Badger: All operations via Badger

    Note over Dev,Badger: Phase 3: Remove SQLite
    Dev->>Dev: Delete internal/storage/sqlite/
    Dev->>Dev: Remove SQLite from config
    Dev->>Dev: Update factory to Badger-only

    Note over Dev,Badger: Phase 4: Update App Init
    Dev->>Storage: Initialize Badger storage
    Storage->>Queue: Pass Badger DB to queue
    Queue->>Job: Pass storage interfaces
    Job->>Badger: All persistence via Badger

    Note over Dev,Badger: Result: Single Storage Backend
    Badger-->>Job: Job data
    Badger-->>Queue: Queue messages
    Badger-->>Storage: Documents, Auth, KV

## Proposed File Changes

### internal\queue\badger_manager.go(NEW)

References: 

- internal\queue\manager.go(DELETE)
- internal\queue\types.go
- internal\storage\badger\job_storage.go

Create new Badger-based queue manager to replace goqite.

**Implementation Requirements:**
- Implement same interface as current `Manager` in `internal/queue/manager.go`
- Use Badger for persistent message storage with key structure: `queue:{queue_name}:{message_id}`
- Message ID generation: Use UUID or timestamp-based IDs (replace goqite.ID)
- Visibility timeout: Store message with `visible_at` timestamp, only return messages where `visible_at <= now()`
- Redelivery logic: Track `receive_count`, implement max_receive limit
- FIFO ordering: Use Badger's ordered key iteration with timestamp prefixes
- Atomic operations: Use Badger transactions for enqueue/receive/delete

**Key Methods:**
- `NewBadgerManager(db *badger.DB, queueName string, visibilityTimeout time.Duration, maxReceive int) (*BadgerManager, error)` - Constructor
- `Enqueue(ctx context.Context, msg Message) error` - Add message with unique ID and initial visibility
- `Receive(ctx context.Context) (*Message, func() error, error)` - Get next visible message, update visibility timeout, return delete function
- `Extend(ctx context.Context, messageID string, duration time.Duration) error` - Extend visibility timeout for long-running jobs
- `Close() error` - Cleanup resources

**Data Structures:**
- `QueueMessage` struct with fields: ID, Body (Message), EnqueuedAt, VisibleAt, ReceiveCount
- Serialize to JSON for Badger storage

**Concurrency Safety:**
- Use Badger transactions to prevent race conditions
- Implement optimistic locking for message visibility updates

**Reference Implementation Pattern:**
Follow the structure of `internal/storage/badger/job_storage.go` for Badger usage patterns, transaction handling, and error management.

### internal\interfaces\queue_service.go(MODIFY)

Update QueueManager interface to remove goqite dependency.

**Changes:**
1. Remove import of `maragu.dev/goqite`
2. Change `Extend` method signature from `Extend(ctx context.Context, messageID goqite.ID, duration time.Duration) error` to `Extend(ctx context.Context, messageID string, duration time.Duration) error`
3. Update interface documentation to reflect Badger-based implementation
4. Add comment explaining breaking change from goqite.ID to string

**Rationale:**
Removing goqite.ID type dependency allows complete removal of goqite library. String-based message IDs are more flexible and work with any queue backend.

### internal\queue\manager.go(DELETE)

Delete goqite-based queue manager implementation.

This file is replaced by `badger_manager.go` which provides the same interface without SQLite dependency.

### internal\jobs\manager.go(MODIFY)

References: 

- internal\interfaces\storage.go
- internal\storage\badger\job_storage.go

Refactor JobManager to use storage interfaces instead of direct SQL database access.

**Major Architectural Change:**
Replace `db *sql.DB` field with storage interface dependencies:
- `jobStorage interfaces.JobStorage` - For job CRUD operations
- `jobLogStorage interfaces.JobLogStorage` - For job log operations
- `jobDefStorage interfaces.JobDefinitionStorage` - For job definition operations (if needed)

**Constructor Update:**
Change `NewManager(db *sql.DB, queue *queue.Manager, eventService interfaces.EventService)` to:
`NewManager(jobStorage interfaces.JobStorage, jobLogStorage interfaces.JobLogStorage, queue interfaces.QueueManager, eventService interfaces.EventService)`

**Method Refactoring:**
Replace all direct SQL queries with interface method calls:
- `db.QueryRow()` → `jobStorage.GetJob()`
- `db.Query()` → `jobStorage.ListJobs()`
- `db.Exec()` → `jobStorage.SaveJob()`, `jobStorage.UpdateJob()`, `jobStorage.DeleteJob()`
- Log operations → `jobLogStorage.AppendLog()`, `jobLogStorage.GetLogs()`

**Remove SQL-Specific Code:**
- Remove `retryOnBusy()` function (SQLite-specific busy handling)
- Remove direct SQL transaction management
- Remove SQL schema assumptions

**Preserve Business Logic:**
- Keep all job lifecycle logic (create, update, status transitions)
- Keep event publishing for job status changes
- Keep job tree aggregation logic
- Keep progress tracking and statistics

**Error Handling:**
Update error messages to be storage-agnostic (remove "SQLite" references).

**Testing Impact:**
This change enables testing with any storage backend (Badger, mock, etc.).

### internal\app\app.go(MODIFY)

References: 

- internal\queue\badger_manager.go(NEW)
- internal\jobs\manager.go(MODIFY)

Update app initialization to use Badger-only storage and new queue implementation.

**Remove SQLite DB Fields (lines 51-132):**
- Delete `SQLiteDB *sql.DB` field
- Delete `SQLiteDBCloser interface{ Close() error }` field
- Remove `database/sql` import

**Update initServices Method (lines 307-664):**

**Section 5.6 - Queue Manager Initialization (lines 389-408):**
Replace entire section with:
```go
// 5.6. Initialize queue manager (Badger-backed)
badgerDB := a.StorageManager.DB().(*badger.DB)
queueMgr, err := queue.NewBadgerManager(
    badgerDB,
    a.Config.Queue.QueueName,
    parseDuration(a.Config.Queue.VisibilityTimeout),
    a.Config.Queue.MaxReceive,
)
if err != nil {
    return fmt.Errorf("failed to initialize queue manager: %w", err)
}
a.QueueManager = queueMgr
a.Logger.Info().Str("queue_name", a.Config.Queue.QueueName).Msg("Queue manager initialized")
```

**Section 5.8 - Job Manager Initialization (lines 410-413):**
Replace with:
```go
// 5.8. Initialize job manager with storage interfaces
jobMgr := jobs.NewManager(
    a.StorageManager.JobStorage(),
    a.StorageManager.JobLogStorage(),
    queueMgr,
    a.EventService,
)
a.JobManager = jobMgr
a.Logger.Info().Msg("Job manager initialized")
```

**Remove initDatabase SQLite Logic (lines 237-305):**
Remove lines 391-401 that create separate SQLite DB for queue/jobs when main storage is not SQLite.

**Update Close Method (lines 864-971):**
Remove SQLite DB closing logic:
- Remove `a.SQLiteDB.Close()` calls
- Remove `a.SQLiteDBCloser.Close()` calls

**Add Helper Function:**
Add `parseDuration(s string) time.Duration` helper to parse visibility timeout string.

**Import Updates:**
- Remove `database/sql` import
- Add `github.com/dgraph-io/badger/v3` import for type assertion

### internal\storage\factory.go(MODIFY)

Simplify storage factory to support Badger only.

**Remove SQLite Support:**
1. Remove `"github.com/ternarybob/quaero/internal/storage/sqlite"` import
2. Update `NewStorageManager` function:
   - Remove `case "sqlite", "":` branch
   - Change default case to return Badger manager
   - Update error message to indicate only Badger is supported

**Updated Implementation:**
```go
func NewStorageManager(logger arbor.ILogger, config *common.Config) (interfaces.StorageManager, error) {
    switch config.Storage.Type {
    case "badger", "":
        return badger.NewManager(logger, &config.Storage.Badger)
    default:
        return nil, fmt.Errorf("unsupported storage type: %s (only 'badger' is supported)", config.Storage.Type)
    }
}
```

**Remove NewAuthStorage Function:**
Delete `NewAuthStorage` function (lines 25-32) as it's a thin wrapper that's no longer needed. Callers should use `StorageManager.AuthStorage()` instead.

**Documentation:**
Add comment explaining that SQLite support was removed and Badger is the only supported backend.

### internal\common\config.go(MODIFY)

Remove SQLite configuration section and simplify storage config.

**Remove SQLite Config Type (lines 62-70):**
Delete entire `SQLiteConfig` struct definition.

**Update StorageConfig (lines 50-56):**
Remove `SQLite SQLiteConfig \`toml:"sqlite"\`` field.
Result:
```go
type StorageConfig struct {
    Type       string           `toml:"type"` // "badger" only
    Badger     BadgerConfig     `toml:"badger"`
    RavenDB    RavenDBConfig    `toml:"ravendb"` // Future support
    Filesystem FilesystemConfig `toml:"filesystem"`
}
```

**Update NewDefaultConfig (lines 196-213):**
Remove SQLite config initialization:
```go
Storage: StorageConfig{
    Type: "badger",
    Badger: BadgerConfig{
        Path: "./data/quaero.badger",
    },
    Filesystem: FilesystemConfig{
        Images:      "./data/images",
        Attachments: "./data/attachments",
    },
},
```

**Update Documentation:**
- Update `StorageConfig.Type` comment to indicate only "badger" is supported
- Add migration note in file header explaining SQLite removal

**Environment Variable Cleanup:**
In `applyEnvOverrides` function (lines 361-621), remove all SQLite-related environment variable handling:
- Remove `QUAERO_STORAGE_SQLITE_*` variable parsing
- Keep only Badger-related variables

**Breaking Change Note:**
Add comment in file header:
```go
// BREAKING CHANGE: SQLite storage removed in favor of Badger-only architecture.
// Existing SQLite databases cannot be automatically migrated. Users must export
// data and re-import after upgrading.
```

### internal\storage\sqlite(DELETE)

Delete entire SQLite storage implementation directory.

This removes all SQLite-specific code including:
- `manager.go` - SQLite storage manager
- `connection.go` - SQLite database connection and schema setup
- `schema.go` - SQL schema definitions
- `document_storage.go` - Document storage implementation
- `auth_storage.go` - Auth storage implementation
- `kv_storage.go` - Key-value storage implementation
- `job_storage.go` - Job storage implementation
- `job_log_storage.go` - Job log storage implementation
- `job_definition_storage.go` - Job definition storage implementation
- `load_*.go` - File loading utilities
- All test files (`*_test.go`)

**Rationale:**
Complete removal of SQLite dependency as requested. All functionality is now provided by Badger storage implementations in `internal/storage/badger/`.

### go.mod(MODIFY)

Remove SQLite and goqite dependencies from go.mod.

**Remove Dependencies:**
1. `maragu.dev/goqite` - goqite queue library (SQLite-based)
2. `modernc.org/sqlite` - Pure Go SQLite driver

**Keep Dependencies:**
- `github.com/dgraph-io/badger/v3` - Badger database (primary storage)
- All other existing dependencies

**Run After Changes:**
```bash
go mod tidy
```

This will clean up any transitive dependencies that are no longer needed.

### test\common\setup.go(MODIFY)

Update test setup to ensure Badger-only configuration.

**Verify Badger Configuration (lines 197-259):**
In `LoadTestConfig` function, ensure test config always uses Badger:
- Verify `config.Storage.Type = "badger"` is set
- Remove any SQLite-related test configuration
- Ensure test database path uses unique directory per test run

**Update SetupTestEnvironment (lines 261-458):**
- Remove any SQLite-specific setup code
- Ensure Badger database is properly initialized
- Add cleanup for Badger database directory in test teardown

**No Major Changes Expected:**
Tests already use Badger via `SetupTestEnvironment()`, so minimal changes should be needed. This is primarily verification and cleanup.

**Add Test Helper:**
Add helper function to generate unique Badger paths for parallel test execution:
```go
func GetTestBadgerPath(testName string) string {
    return filepath.Join(os.TempDir(), "quaero-test-", testName, time.Now().Format("20060102-150405"))
}
```

### test\api\health_check_test.go(MODIFY)

References: 

- test\common\setup.go(MODIFY)

Verify and update health check test to ensure Badger-only usage.

**Verification Steps:**
1. Confirm test uses `common.SetupTestEnvironment()` which provides Badger storage
2. Remove any SQLite-specific assertions or checks
3. Ensure test cleanup properly closes Badger database

**Expected Changes:**
Minimal to none - this test already uses the common test framework which uses Badger.

**Add Test Case:**
Add test case to verify storage type is Badger:
```go
func TestStorageType(t *testing.T) {
    env := common.SetupTestEnvironment(t, "health_check")
    defer env.Cleanup()
    
    // Verify storage type via API
    helper := common.NewHTTPTestHelper(t, env.BaseURL())
    resp := helper.Get("/api/config")
    helper.AssertStatus(resp, http.StatusOK)
    
    var config map[string]interface{}
    helper.ParseJSON(resp, &config)
    
    storage := config["storage"].(map[string]interface{})
    assert.Equal(t, "badger", storage["type"])
}
```

### test\api\settings_system_test.go(MODIFY)

References: 

- test\common\setup.go(MODIFY)

Update settings/system tests to remove SQLite-specific test cases.

**Remove SQLite-Specific Tests:**
- Remove any tests that verify SQLite-specific behavior
- Remove tests that check SQLite configuration
- Remove tests that verify SQLite database file paths

**Update Config Tests:**
In `TestConfig_Get` function, update assertions to expect Badger-only config:
- Verify `storage.type` is "badger"
- Remove assertions for `storage.sqlite` fields
- Verify `storage.badger.path` exists

**No Major Changes Expected:**
These tests use `common.SetupTestEnvironment()` which already provides Badger storage. Changes should be minimal verification updates.

### test\api\jobs_test.go(MODIFY)

References: 

- internal\queue\badger_manager.go(NEW)

Update jobs tests to work with Badger-based queue implementation.

**Queue Behavior Changes:**
Badger queue may have slightly different timing/ordering behavior than goqite:
- Update test timeouts if needed for queue polling
- Adjust assertions for message ordering (FIFO should be maintained)
- Update visibility timeout tests to work with new implementation

**Remove SQLite-Specific Tests:**
- Remove any tests that verify SQLite-specific queue behavior
- Remove tests that check goqite-specific features

**Update Job Lifecycle Tests:**
Ensure job lifecycle tests (create → run → complete → logs) work with new queue:
- Verify job enqueueing works
- Verify job processing works
- Verify job status transitions work
- Verify job logs are persisted correctly

**Add Queue-Specific Tests:**
Add tests to verify Badger queue behavior:
- Test message visibility timeout
- Test message redelivery after max_receive
- Test concurrent message processing
- Test queue persistence across restarts

**No Major Changes Expected:**
Tests use high-level job APIs which abstract queue implementation. Most tests should work unchanged.

### README.md(MODIFY)

Update README to reflect Badger-only storage architecture.

**Update Architecture Section:**
- Remove references to SQLite storage option
- Update storage description to indicate Badger is the only supported backend
- Remove SQLite configuration examples
- Update database path examples to use Badger paths

**Update Configuration Section:**
- Remove `[storage.sqlite]` configuration examples
- Update `[storage]` section to show only Badger config:
  ```toml
  [storage]
  type = "badger"
  
  [storage.badger]
  path = "./data/quaero.badger"
  ```

**Update Installation/Setup Instructions:**
- Remove SQLite-related setup steps
- Update data directory structure to show Badger directory
- Remove references to SQLite database files

**Add Migration Guide:**
Add section explaining breaking change:
```markdown
## Breaking Changes in v2.0

### SQLite Removal

Version 2.0 removes SQLite support in favor of Badger-only storage. This provides:
- Simpler architecture (single storage backend)
- Better performance for concurrent operations
- No external database dependencies

**Migration:** Existing SQLite databases cannot be automatically migrated. To upgrade:
1. Export your data from v1.x (documents, jobs, etc.)
2. Upgrade to v2.0
3. Re-import your data via API

Alternatively, continue using v1.x if SQLite support is required.
```

**Update Troubleshooting Section:**
- Remove SQLite-specific troubleshooting tips
- Add Badger-specific troubleshooting (e.g., disk space, file permissions)

**Update API Documentation:**
- Remove references to SQLite-specific features (e.g., FTS5)
- Update search documentation to reflect Badger-based search

### AGENTS.md(MODIFY)

Update AGENTS.md to reflect Badger-only storage architecture.

**Update Storage Section:**
Find and update all references to storage architecture:
- Change "SQLite with FTS5" to "Badger with custom search"
- Remove SQLite-specific implementation notes
- Update storage layer description to indicate Badger-only

**Update Job System Architecture:**
Update queue description (around lines 310-320 based on grep results):
- Change "goqite-backed queue" to "Badger-backed queue"
- Remove references to goqite library
- Update queue implementation notes

**Update Data Flow Section:**
Update storage references in data flow diagrams:
- Change "SQLite" to "Badger" in all flow descriptions
- Update database schema references

**Update Technology Stack:**
Update storage technology:
- Remove: "Storage: SQLite with FTS5 (full-text search)"
- Add: "Storage: Badger (embedded key-value store)"
- Remove: "Queue: goqite (SQLite-backed message queue)"
- Add: "Queue: Custom Badger-backed message queue"

**Update Storage Schema Section:**
Remove SQLite-specific schema details:
- Remove FTS5 index descriptions
- Remove SQL schema examples
- Add Badger key structure documentation

**Update Testing Guidelines:**
Update test infrastructure notes:
- Remove SQLite test setup instructions
- Update to reflect Badger-only testing
- Update test database cleanup instructions

**Update Common Development Tasks:**
Update any task instructions that reference SQLite:
- Database initialization
- Schema migrations
- Data export/import

### docs\features\endpoint-review.md(NEW)

Update endpoint review documentation to reflect storage changes.

**Update Endpoint Descriptions:**
Find and update any endpoint descriptions that mention SQLite:
- Update storage-related endpoint descriptions
- Remove SQLite-specific endpoint notes
- Update database operation descriptions

**Update Implementation Notes:**
If document contains implementation notes about storage:
- Update to reflect Badger-only architecture
- Remove SQLite-specific implementation details
- Add notes about Badger queue implementation

**Minimal Changes Expected:**
This is primarily a documentation file, so changes should be limited to descriptive text updates.