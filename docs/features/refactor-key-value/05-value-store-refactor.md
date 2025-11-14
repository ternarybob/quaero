I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The codebase currently loads API keys from the `./auth` directory via `LoadAuthCredentialsFromFiles()`, which stores them in the KV store after Phase 3 migration. However, this mixes authentication concerns (cookies for website extraction) with generic key/value storage. The user requires a dedicated loader for key/value pairs from a separate `./keys` directory with a simpler TOML format.

**Current state:**
- `load_auth_credentials.go` loads TOML files with `[section-name]` containing `api_key`, `service_type`, `description` fields
- Auth loader stores API keys in KV store (post-Phase 3 migration)
- Config has `AuthDirConfig` with `CredentialsDir` field (default: `./auth`)
- App initialization sequence: config load → auth credentials load → API key migration → `{key-name}` replacement → service init

**Key requirements from user:**
- New loader must be **separate from auth** (auth will become cookie-only in Phase 6)
- Simpler TOML format: `[key-name]` sections with `value = "secret"` (required) and `description = "..."` (optional)
- Load into KV store using `m.kv.Set(ctx, sectionName, value, description)`
- Add `KeysDirConfig` to main `Config` struct with default `./keys`
- Call new loader in `app.go` after auth credentials loading
- Comprehensive unit tests following existing patterns
- Breaking changes are acceptable, fewer steps are better


### Approach

Create a dedicated key/value loader infrastructure that mirrors the auth credentials loader pattern but with a simpler TOML schema. The implementation follows established codebase patterns: TOML parsing with sections, validation, idempotent storage operations, and comprehensive logging. The new loader will be called during app initialization after auth credentials loading, ensuring clean separation of concerns between authentication (cookies) and generic key/value storage.

**Key design decisions:**
1. **Simpler TOML schema**: Only `value` (required) and `description` (optional) fields, no `service_type`
2. **Idempotent loading**: Uses `Set()` which has `ON CONFLICT` handling for safe re-runs
3. **Separate config section**: `Keys KeysDirConfig` in main config, distinct from `Auth`
4. **Consistent error handling**: Non-fatal directory missing, warnings for invalid files, graceful degradation
5. **Reuse existing patterns**: File iteration, TOML parsing, validation, logging match auth loader style


### Reasoning

Explored the codebase by reading the three files mentioned (`load_auth_credentials.go`, `config.go`, `app.go`), then examined the KV storage interface and implementation to understand the `Set()` method signature. Reviewed existing test patterns in `load_auth_credentials_test.go` to understand the testing approach. Checked the SQLite manager structure to see where the new loader method will be added. This provided a complete picture of existing patterns and requirements for the new key/value loader.


## Mermaid Diagram

sequenceDiagram
    participant App as app.go<br/>initDatabase()
    participant Config as Config struct
    participant Manager as sqlite.Manager
    participant Loader as load_keys.go
    participant KVStore as KVStorage
    participant FS as File System

    Note over App,FS: Startup Sequence - Database Initialization
    
    App->>Config: Read Keys.Dir config
    Config-->>App: "./keys" (default)
    
    App->>Manager: LoadAuthCredentialsFromFiles("./auth")
    Note over Manager,KVStore: Phase 3: Auth loads API keys to KV
    Manager->>KVStore: Store API keys from auth files
    
    App->>Manager: LoadKeysFromFiles("./keys")
    activate Loader
    
    Loader->>FS: os.Stat("./keys")
    alt Directory exists
        FS-->>Loader: Directory found
        Loader->>FS: os.ReadDir("./keys")
        FS-->>Loader: [file1.toml, file2.toml, readme.txt]
        
        loop For each .toml file
            Loader->>FS: os.ReadFile(file)
            FS-->>Loader: TOML content
            Loader->>Loader: toml.Unmarshal() into map
            Loader->>Loader: validateKeyValueFile()
            alt Valid entry
                Loader->>KVStore: Set(key, value, description)
                KVStore-->>Loader: Success (ON CONFLICT UPDATE)
            else Invalid entry
                Loader->>Loader: Log warning, skip
            end
        end
        
        Loader-->>App: nil (success, N loaded, M skipped)
    else Directory not found
        FS-->>Loader: os.ErrNotExist
        Loader-->>App: nil (graceful, debug log)
    end
    deactivate Loader
    
    App->>Manager: MigrateAPIKeysToKVStore()
    Note over Manager,KVStore: Phase 3: Migrate old API keys
    
    App->>KVStore: GetAll() for replacement
    KVStore-->>App: Map of all keys
    App->>Config: ReplaceInStruct(config, kvMap)
    Note over App,Config: Phase 2: Replace {key-name} placeholders
    
    App->>App: Initialize services
    Note over App: LLM, Agent, Places use resolved keys

## Proposed File Changes

### internal\storage\sqlite\load_keys.go(NEW)

References: 

- internal\storage\sqlite\load_auth_credentials.go
- internal\interfaces\kv_storage.go

Create new file for loading key/value pairs from TOML files in the `./keys` directory.

**Structure:**
- Define `KeyValueFile` struct with `Value string` (required) and `Description string` (optional) fields for TOML unmarshaling
- Implement `LoadKeysFromFiles(ctx context.Context, dirPath string) error` method on `Manager` - main entry point that iterates directory and processes TOML files
- Implement `loadKeysFromTOML(filePath string) (map[string]*KeyValueFile, error)` helper - parses TOML file into map of sections
- Implement `validateKeyValueFile(kvFile *KeyValueFile, sectionName string) error` helper - validates required fields

**Implementation details:**
- Check if directory exists using `os.Stat()`, return nil (not error) if missing - directory is optional
- Read directory entries with `os.ReadDir()`, skip non-TOML files and subdirectories
- For each TOML file, parse sections using `toml.Unmarshal()` into `map[string]*KeyValueFile`
- Validate each section: non-empty section name and non-empty value
- Store in KV store using `m.kv.Set(ctx, sectionName, kvFile.Value, kvFile.Description)`
- Use default description if empty: "Loaded from file"
- Log loaded/skipped counts with structured logging using Arbor
- Handle errors gracefully: warn on file parse errors, continue processing other files

**TOML format example:**
```toml
[google-api-key]
value = "AIzaSyABC123..."
description = "Google API key for Gemini"

[github-token]
value = "ghp_xyz789..."
description = "GitHub personal access token"
```

**Error handling:**
- Directory not found: Debug log, return nil
- File read error: Warn log, skip file, continue
- TOML parse error: Warn log, skip file, continue
- Validation error: Warn log, skip section, continue
- KV store error: Error log, skip section, continue

**Logging levels:**
- Info: Directory path, loaded/skipped counts summary
- Debug: Skipped non-TOML files, directory not found
- Warn: File parse errors, validation errors
- Error: KV store errors

Follow the exact pattern from `load_auth_credentials.go` but with simpler struct (no `ServiceType` field).

### internal\common\keys_config.go(NEW)

References: 

- internal\common\config.go(MODIFY)

Create new file containing the `KeysDirConfig` struct for key/value directory configuration.

**Structure:**
- Define `KeysDirConfig` struct in package `common`
- Single field: `Dir string` with TOML tag `toml:"dir"`
- Add package-level documentation comment explaining purpose

**Purpose:**
- Separate config struct for keys directory (distinct from `AuthDirConfig`)
- Supports TOML unmarshaling with `toml:"dir"` tag
- Will be embedded in main `Config` struct as `Keys KeysDirConfig`
- Default value set in `NewDefaultConfig()`: `./keys`

**Environment variable support:**
- Will be added in `config.go` `applyEnvOverrides()` function: `QUAERO_KEYS_DIR`

**Documentation:**
- Add comment: "KeysDirConfig contains configuration for key/value file loading"
- Add field comment: "Dir is the directory containing key/value files (TOML format)"

Keep the file minimal and focused - single struct definition only, following the pattern of `AuthDirConfig`.

### internal\common\config.go(MODIFY)

References: 

- internal\common\keys_config.go(NEW)

Update config to add key/value directory configuration support.

**Changes to `Config` struct (around line 19-34):**
- Add new field after `Auth AuthDirConfig` field: `Keys KeysDirConfig` with TOML tag `toml:"keys"`
- Add comment above field: "Keys directory configuration for key/value file loading"
- This enables `[keys]` section in `quaero.toml`

**Changes to `NewDefaultConfig()` function (around line 174-287):**
- Add initialization after `Auth: AuthDirConfig{...}` block (around line 219-221)
- Set `Keys: KeysDirConfig{ Dir: "./keys" }` with comment "Default directory for key/value files"

**Changes to `applyEnvOverrides()` function (around line 352-611):**
- Add environment variable override after auth config section (around line 607-610)
- Check `QUAERO_KEYS_DIR` environment variable
- If set, override `config.Keys.Dir` with environment value
- Add comment: "Keys configuration"

**Documentation:**
- Ensure consistency with existing config patterns
- Follow same style as `Auth` field and `AuthDirConfig` initialization

**No breaking changes:**
- New field is optional (has default value)
- Existing configs without `[keys]` section will use default `./keys`
- Environment variable override is optional
- Backward compatible with existing deployments

### internal\app\app.go(MODIFY)

References: 

- internal\storage\sqlite\load_keys.go(NEW)
- internal\common\config.go(MODIFY)

Update app initialization to call the new key/value loader during database initialization.

**Changes to `initDatabase()` method (around line 198-258):**

**Location:** Insert new code block after `LoadAuthCredentialsFromFiles()` call (around line 220-228), before `MigrateAPIKeysToKVStore()` call (around line 230-238).

**Add new code block:**
- Check if storage manager is SQLite manager using type assertion
- Create context using `context.Background()`
- Call `sqliteMgr.LoadKeysFromFiles(ctx, a.Config.Keys.Dir)`
- Handle error with warning log: "Failed to load key/value pairs from files"
- Add comment: "Don't fail startup - key files are optional"
- On success, log info message with directory path: "Key/value pairs loaded from files"
- Add comment above block: "Load key/value pairs from files (after auth credentials)" and "This is separate from auth - auth is for cookies, keys are for generic secrets"

**Rationale for placement:**
- Load keys **after** auth credentials to maintain separation of concerns
- Load keys **before** API key migration to ensure all keys are available
- Load keys **before** `{key-name}` replacement in config (around line 240-255) so replacements can use newly loaded keys
- Non-fatal error handling: warn and continue if directory missing or files invalid
- Structured logging with directory path for debugging

**No changes to:**
- Service initialization order
- Handler initialization
- Other startup logic
- Existing auth credentials loading

**Testing note:**
- Ensure keys are loaded before config replacement (Phase 2 integration)
- Verify keys are available for service initialization (LLM, Agent, Places services)

### internal\storage\sqlite\load_keys_test.go(NEW)

References: 

- internal\storage\sqlite\load_auth_credentials_test.go
- internal\storage\sqlite\load_keys.go(NEW)

Create comprehensive unit tests for the new key/value loader.

**Test cases to implement:**

1. **TestLoadKeysFromTOML_WithSections**
   - Create TOML file with 2-3 sections using `os.WriteFile()` in `t.TempDir()`
   - Each section has `value` and `description` fields
   - Call `loadKeysFromTOML(filePath)` helper method
   - Assert section count matches expected (use `assert.Len()`)
   - Verify each section's key name, value, and description fields
   - Use `require.True()` to check section existence, `assert.Equal()` for field values

2. **TestLoadKeysFromTOML_EmptyFile**
   - Create empty TOML file in temp directory
   - Call `loadKeysFromTOML(filePath)`
   - Expect error using `require.Error()`
   - Assert error message contains "no sections found" using `assert.Contains()`

3. **TestLoadKeysFromFiles_StoresInKV**
   - Create temp directory with TOML file containing 2-3 key/value pairs
   - Create test manager with `setupTestDB(t)` helper
   - Call `LoadKeysFromFiles(ctx, tmpDir)` on manager
   - Verify keys stored in KV store using `m.kv.Get(ctx, key)` for each key
   - Verify descriptions using `m.kv.List(ctx)` and iterate results
   - Assert all keys and descriptions match expected values

4. **TestLoadKeysFromFiles_DirectoryNotFound**
   - Call `LoadKeysFromFiles()` with non-existent directory path
   - Expect no error (graceful degradation) using `require.NoError()`
   - Verify no keys stored in KV store by checking `m.kv.List()` returns empty slice

5. **TestLoadKeysFromFiles_SkipsNonTOML**
   - Create temp directory with mix of TOML and non-TOML files (.txt, .json, .md)
   - Call `LoadKeysFromFiles()`
   - Verify only TOML files processed by checking KV store contents
   - Assert only expected keys from TOML files exist

6. **TestValidateKeyValueFile**
   - Table-driven test with multiple scenarios
   - Test cases: valid key/value (no error), empty section name (error), empty value (error), missing description (no error - optional)
   - For each case, call `validateKeyValueFile()` and check error presence/message
   - Use `require.Error()` for error cases, `assert.Contains()` for error messages
   - Use `require.NoError()` for valid cases

**Test helpers and setup:**
- Use `t.TempDir()` for temporary directories (auto-cleanup)
- Use `os.WriteFile()` for creating test TOML files
- Use `setupTestDB(t)` for in-memory database (existing helper)
- Create `Manager` struct with `db`, `kv`, `logger` fields initialized
- Use `context.Background()` for all context parameters
- Use `arbor.NewLogger()` for test logger

**TOML test data example:**
```toml
[test-key-1]
value = "secret-value-1"
description = "Test key 1"

[test-key-2]
value = "secret-value-2"
description = "Test key 2"
```

**Assertion patterns:**
- Use `require.NoError()` for critical operations that must succeed
- Use `assert.Equal()` for value comparisons
- Use `assert.Len()` for slice/map length checks
- Use `assert.Contains()` for error message substring checks
- Use `assert.True()/False()` for boolean checks
- Use `require.True()` when subsequent code depends on the assertion

Follow the exact pattern from `load_auth_credentials_test.go` but adapted for simpler key/value structure (no `ServiceType` field).