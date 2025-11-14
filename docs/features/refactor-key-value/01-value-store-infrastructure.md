I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State

**Auth Storage combines two concerns:**
- Cookie-based authentication (site_domain, cookies, tokens)
- API key storage (api_key field with auth_type='api_key')

**Existing patterns to follow:**
- Storage implementations: `document_storage.go`, `job_log_storage.go`, `auth_storage.go`
- Interface definitions in `internal/interfaces/storage.go`
- Manager wiring in `internal/storage/sqlite/manager.go`
- App initialization in `internal/app/app.go`
- Test patterns in `internal/storage/sqlite/*_test.go`

**Key architectural decisions:**
- Use SQLite for persistence (consistent with existing storage)
- Follow constructor-based dependency injection pattern
- Use `sync.Mutex` for write serialization (prevents SQLITE_BUSY errors)
- Store timestamps as Unix integers (consistent with schema)
- Return interfaces from constructors (enables testing with mocks)

### Approach

## Separation Strategy

**Create dedicated key/value infrastructure:**
1. New `key_value_store` table with simple schema (key, value, description, timestamps)
2. `KeyValueStorage` interface with CRUD operations (Get, Set, Delete, List, GetAll)
3. SQLite implementation following existing storage patterns
4. Business logic service layer for future extensibility
5. Wire into `StorageManager` and `App` initialization

**Design principles:**
- Keep it simple - no complex features in Phase 1
- Follow existing patterns exactly (consistency over innovation)
- Prepare for Phase 2 `{key-name}` replacement feature
- Maintain backward compatibility (Phase 3 will migrate API keys)

### Reasoning

Explored the codebase structure by reading schema definitions, storage implementations, interface patterns, manager wiring, app initialization, and test examples. Analyzed how auth storage currently handles both cookies and API keys, and identified the patterns to follow for creating the new key/value store infrastructure.

## Proposed File Changes

### internal\storage\sqlite\schema.go(MODIFY)

Add `key_value_store` table definition to `schemaSQL` constant after the `auth_credentials` table:

**Table structure:**
- `key TEXT PRIMARY KEY` - Unique key identifier
- `value TEXT NOT NULL` - Stored value (string only for Phase 1)
- `description TEXT` - Optional human-readable description
- `created_at INTEGER NOT NULL` - Unix timestamp
- `updated_at INTEGER NOT NULL` - Unix timestamp

**Index:**
- Create index on `updated_at` for efficient listing by recency: `CREATE INDEX IF NOT EXISTS idx_kv_updated ON key_value_store(updated_at DESC)`

**Placement:** Insert after line 36 (after auth_credentials indexes) and before line 38 (documents table comment)

**Note:** No foreign keys or complex constraints - keep it simple for generic key/value storage

### internal\interfaces\kv_storage.go(NEW)

References: 

- internal\interfaces\storage.go(MODIFY)

Create new interface file for key/value storage operations:

**Interface: `KeyValueStorage`**

Methods:
- `Get(ctx context.Context, key string) (string, error)` - Retrieve value by key, returns error if not found
- `Set(ctx context.Context, key string, value string, description string) error` - Insert or update key/value pair
- `Delete(ctx context.Context, key string) error` - Delete key/value pair, returns error if not found
- `List(ctx context.Context) ([]KeyValuePair, error)` - List all key/value pairs ordered by updated_at DESC
- `GetAll(ctx context.Context) (map[string]string, error)` - Get all key/value pairs as map (useful for bulk replacement operations in Phase 2)

**Model: `KeyValuePair`**

Struct fields:
- `Key string` - The key
- `Value string` - The value
- `Description string` - Optional description
- `CreatedAt time.Time` - Creation timestamp
- `UpdatedAt time.Time` - Last update timestamp

**Package:** `package interfaces`

**Imports:** `context`, `time`

**Pattern reference:** Follow `AuthStorage` interface pattern in `internal/interfaces/storage.go` (lines 15-29)

### internal\storage\sqlite\kv_storage.go(NEW)

References: 

- internal\storage\sqlite\document_storage.go
- internal\storage\sqlite\job_log_storage.go

Create SQLite implementation of `KeyValueStorage` interface:

**Struct: `KVStorage`**
- Fields: `db *SQLiteDB`, `logger arbor.ILogger`, `mu sync.Mutex` (for write serialization)
- Constructor: `NewKVStorage(db *SQLiteDB, logger arbor.ILogger) interfaces.KeyValueStorage`

**Method: `Get(ctx context.Context, key string) (string, error)`**
- Query: `SELECT value FROM key_value_store WHERE key = ?`
- Return `sql.ErrNoRows` as `fmt.Errorf("key '%s' not found", key)` for better error messages
- Use `QueryRowContext` for single row retrieval

**Method: `Set(ctx context.Context, key string, value string, description string) error`**
- Use mutex lock/unlock for write serialization (prevents SQLITE_BUSY)
- Query: `INSERT INTO key_value_store (key, value, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value, description = excluded.description, updated_at = excluded.updated_at`
- Set timestamps: `now := time.Now().Unix()`
- Use `ExecContext` for insert/update

**Method: `Delete(ctx context.Context, key string) error`**
- Query: `DELETE FROM key_value_store WHERE key = ?`
- Check `RowsAffected()` and return error if 0 rows deleted: `fmt.Errorf("key '%s' not found", key)`
- Use `ExecContext` for deletion

**Method: `List(ctx context.Context) ([]interfaces.KeyValuePair, error)`**
- Query: `SELECT key, value, description, created_at, updated_at FROM key_value_store ORDER BY updated_at DESC`
- Scan into slice of `interfaces.KeyValuePair` structs
- Convert Unix timestamps to `time.Time` using `time.Unix()`
- Use `QueryContext` for multiple rows

**Method: `GetAll(ctx context.Context) (map[string]string, error)`**
- Query: `SELECT key, value FROM key_value_store`
- Build `map[string]string` from results
- Use `QueryContext` for multiple rows

**Pattern reference:** Follow `DocumentStorage` implementation pattern in `internal/storage/sqlite/document_storage.go` (lines 18-34 for struct/constructor, lines 207-218 for Get pattern, lines 36-113 for Set pattern with mutex)

**Imports:** `context`, `database/sql`, `fmt`, `sync`, `time`, `github.com/ternarybob/arbor`, `github.com/ternarybob/quaero/internal/interfaces`

### internal\services\kv\service.go(NEW)

Create business logic service layer for key/value operations:

**Struct: `Service`**
- Fields: `storage interfaces.KeyValueStorage`, `logger arbor.ILogger`
- Constructor: `NewService(storage interfaces.KeyValueStorage, logger arbor.ILogger) *Service`

**Methods (delegate to storage with logging):**

**`Get(ctx context.Context, key string) (string, error)`**
- Call `s.storage.Get(ctx, key)`
- Log debug message on success: `logger.Debug().Str("key", key).Msg("Retrieved key/value pair")`
- Log error on failure: `logger.Error().Err(err).Str("key", key).Msg("Failed to get key/value pair")`

**`Set(ctx context.Context, key string, value string, description string) error`**
- Validate key is not empty: `if key == "" { return fmt.Errorf("key cannot be empty") }`
- Call `s.storage.Set(ctx, key, value, description)`
- Log info message on success: `logger.Info().Str("key", key).Msg("Stored key/value pair")`
- Log error on failure: `logger.Error().Err(err).Str("key", key).Msg("Failed to store key/value pair")`

**`Delete(ctx context.Context, key string) error`**
- Call `s.storage.Delete(ctx, key)`
- Log info message on success: `logger.Info().Str("key", key).Msg("Deleted key/value pair")`
- Log error on failure: `logger.Error().Err(err).Str("key", key).Msg("Failed to delete key/value pair")`

**`List(ctx context.Context) ([]interfaces.KeyValuePair, error)`**
- Call `s.storage.List(ctx)`
- Log debug message with count: `logger.Debug().Int("count", len(pairs)).Msg("Listed key/value pairs")`

**`GetAll(ctx context.Context) (map[string]string, error)`**
- Call `s.storage.GetAll(ctx)`
- Log debug message with count: `logger.Debug().Int("count", len(kvMap)).Msg("Retrieved all key/value pairs")`

**Purpose:** Provides business logic layer for future extensibility (validation, caching, events). Currently thin wrapper but follows service pattern used throughout codebase.

**Pattern reference:** Follow service pattern from `internal/services/documents/service.go` or `internal/services/auth/service.go`

**Imports:** `context`, `fmt`, `github.com/ternarybob/arbor`, `github.com/ternarybob/quaero/internal/interfaces`

### internal\interfaces\storage.go(MODIFY)

Add `KeyValueStorage()` method to `StorageManager` interface:

**Location:** After line 166 (after `JobDefinitionStorage()` method) and before line 167 (`DB()` method)

**Method signature:**
```go
KeyValueStorage() KeyValueStorage
```

**Note:** No additional imports needed - `KeyValueStorage` interface will be defined in new `internal/interfaces/kv_storage.go` file

**Pattern reference:** Follow existing method pattern (lines 162-166)

### internal\storage\sqlite\manager.go(MODIFY)

Wire up key/value storage in SQLite manager:

**1. Add field to `Manager` struct (after line 16):**
```go
kv interfaces.KeyValueStorage
```

**2. Initialize in `NewManager` constructor (after line 33, before line 34):**
```go
kv: NewKVStorage(db, logger),
```

**3. Add getter method (after line 65, before line 67):**
```go
// KeyValueStorage returns the KeyValue storage interface
func (m *Manager) KeyValueStorage() interfaces.KeyValueStorage {
    return m.kv
}
```

**4. Update initialization log message (line 37):**
Change from "Storage manager initialized" to "Storage manager initialized (auth, document, job, jobLog, jobDefinition, kv)"

**Pattern reference:** Follow existing field/initialization pattern for other storage types (lines 12-16, 29-33, 43-65)

### internal\app\app.go(MODIFY)

Wire up key/value service in application initialization:

**1. Add field to `App` struct (after line 95, before line 97):**
```go
// Key/Value service
KVService *kv.Service
```

**2. Initialize service in `initServices()` method (after line 308, before line 310):**
```go
// 5.11. Initialize key/value service
a.KVService = kv.NewService(
    a.StorageManager.KeyValueStorage(),
    a.Logger,
)
a.Logger.Info().Msg("Key/value service initialized")
```

**3. Add import:**
Add to imports section (around line 40): `"github.com/ternarybob/quaero/internal/services/kv"`

**Note:** Service initialization happens after storage layer (line 194-228) but before handlers (line 514). Place after JobService initialization (line 307) for logical grouping with other storage-backed services.

**Pattern reference:** Follow service initialization pattern used for other services (lines 251-308)

### internal\storage\sqlite\kv_storage_test.go(NEW)

References: 

- internal\storage\sqlite\auth_storage_test.go

Create comprehensive unit tests for key/value storage:

**Test: `TestKVStorage_SetAndGet`**
- Set a key/value pair with description
- Retrieve it and verify all fields match
- Verify timestamps are set correctly

**Test: `TestKVStorage_SetUpdate`**
- Set a key/value pair
- Update the same key with new value and description
- Verify value and description updated
- Verify `updated_at` changed but `created_at` unchanged

**Test: `TestKVStorage_GetNotFound`**
- Attempt to get non-existent key
- Verify error message contains key name

**Test: `TestKVStorage_Delete`**
- Set a key/value pair
- Delete it
- Verify Get returns not found error
- Verify Delete on non-existent key returns error

**Test: `TestKVStorage_List`**
- Set multiple key/value pairs with different timestamps
- List all pairs
- Verify count matches
- Verify ordering by updated_at DESC
- Verify all fields populated correctly

**Test: `TestKVStorage_GetAll`**
- Set multiple key/value pairs
- GetAll and verify map contains all keys
- Verify map values match stored values

**Test: `TestKVStorage_EmptyList`**
- List on empty database
- Verify returns empty slice (not nil)

**Test: `TestKVStorage_EmptyGetAll`**
- GetAll on empty database
- Verify returns empty map (not nil)

**Test: `TestKVStorage_ConcurrentWrites`**
- Use goroutines to write multiple keys concurrently
- Verify no SQLITE_BUSY errors (mutex prevents this)
- Verify all keys stored successfully

**Helper function: `setupTestDB(t *testing.T) (*SQLiteDB, func())`**
- Create in-memory SQLite database
- Initialize schema
- Return cleanup function
- Pattern reference: `internal/storage/sqlite/auth_storage_test.go` (lines 13-18)

**Imports:** `context`, `testing`, `time`, `github.com/stretchr/testify/assert`, `github.com/stretchr/testify/require`, `github.com/ternarybob/arbor`

**Pattern reference:** Follow test structure from `internal/storage/sqlite/auth_storage_test.go` (lines 13-236)