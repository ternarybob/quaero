I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current Architecture

**Key/Value Store Infrastructure (Phase 1 - Complete):**
- `key_value_store` table with key, value, description, timestamps
- `KeyValueStorage` interface with Get, Set, Delete, List, GetAll methods
- SQLite implementation in `internal/storage/sqlite/kv_storage.go`
- Service layer in `internal/services/kv/service.go`
- Wired into Manager and App initialization

**Integration Points for Replacement:**
1. **Job Definitions** (`load_job_definitions.go`):
   - Line 176: TOML unmarshal into `JobDefinitionFile`
   - Line 131: Convert to `JobDefinition` model
   - **Replacement target**: `Config map[string]interface{}` and `Steps[].Config map[string]interface{}`

2. **Config Loading** (`config.go`):
   - Line 317: TOML unmarshal into `Config` struct
   - Line 326: Return config
   - **Replacement target**: All string fields in nested Config structs (LLM.GoogleAPIKey, Agent.GoogleAPIKey, PlacesAPI.APIKey, etc.)

**Key Design Constraints:**
- Replacement happens at **runtime** when TOML is used (not at load time)
- Support **string values only** (as specified by user)
- Use `{key-name}` syntax (handlebar-style references)
- Manager has access to KV storage via `m.kv` field
- Config loading is stateless (no Manager access) - needs KV storage passed in

**Critical Insight:**
The user said "replacement occurs when the TOML is used" - this means:
- For job definitions: Replace during `ToJobDefinition()` conversion (line 43-79)
- For config: Replace after unmarshal but before return (line 322-326)
- Both need access to KV storage for `GetAll(ctx)` call

### Approach

## Implementation Strategy

**Core Replacement Engine:**
Create `internal/common/replacement.go` with pure utility functions that take KV map as input. This keeps the replacement logic stateless and testable.

**Integration Pattern:**
1. Fetch KV map once via `kvStorage.GetAll(ctx)` 
2. Pass map to replacement functions
3. Replace in-place (mutate existing structures)
4. Log warnings for unresolved references

**Two Replacement Modes:**
1. **Map replacement** - Recursive traversal of `map[string]interface{}` for job configs
2. **Struct replacement** - Reflection-based traversal of Config struct fields

**Error Handling:**
- Missing keys: Log warning, leave `{key-name}` unreplaced (fail-safe)
- Invalid syntax: Log warning, skip replacement
- KV storage errors: Log error, skip all replacement (graceful degradation)

**Testing Strategy:**
- Unit tests for replacement functions with mock KV data
- Test nested maps, struct fields, missing keys, invalid syntax
- Test integration points (job definitions, config loading)

### Reasoning

Read the relevant files to understand the codebase structure: `load_job_definitions.go` for job definition loading, `config.go` for config loading, `kv_storage.go` and `service.go` for KV storage interface, `manager.go` for storage access patterns, and `job_definition.go` for data models. Analyzed the TOML unmarshaling flow and identified integration points where replacement should occur.

## Mermaid Diagram

sequenceDiagram
    participant Main as main.go
    participant App as app.New()
    participant Config as config.go
    participant Storage as StorageManager
    participant KV as KeyValueStorage
    participant Replace as replacement.go
    participant JobDef as load_job_definitions.go

    Note over Main,JobDef: Phase 1: Initial Config Load (No Replacement)
    Main->>Config: LoadFromFile(nil, path)
    Config->>Config: Unmarshal TOML
    Config-->>Main: Config with {key-name} intact

    Note over Main,JobDef: Phase 2: Storage Initialization
    Main->>App: New(config, logger)
    App->>Storage: NewManager()
    Storage->>KV: NewKVStorage()
    Storage-->>App: StorageManager

    Note over Main,JobDef: Phase 3: Config Replacement
    App->>KV: GetAll(ctx)
    KV-->>App: map[string]string
    App->>Replace: ReplaceInStruct(config, kvMap)
    Replace->>Replace: Traverse struct fields
    Replace->>Replace: ReplaceKeyReferences()
    Replace-->>App: Config with values replaced

    Note over Main,JobDef: Phase 4: Job Definition Loading
    App->>JobDef: LoadJobDefinitionsFromFiles()
    JobDef->>JobDef: Unmarshal TOML
    JobDef->>JobDef: ToJobDefinition(kvStorage, logger)
    JobDef->>KV: GetAll(ctx)
    KV-->>JobDef: map[string]string
    JobDef->>Replace: ReplaceInMap(config, kvMap)
    Replace->>Replace: Recursive map traversal
    Replace->>Replace: ReplaceKeyReferences()
    Replace-->>JobDef: Maps with values replaced
    JobDef->>JobDef: Validate()
    JobDef-->>App: JobDefinitions ready

    Note over Main,JobDef: Runtime: {key-name} → actual values

## Proposed File Changes

### internal\common\replacement.go(NEW)

Create replacement engine with pure utility functions:

**Function: `ReplaceKeyReferences(input string, kvMap map[string]string) string`**
- Regex pattern: `\{([a-zA-Z0-9_-]+)\}` to match `{key-name}` syntax
- Use `regexp.ReplaceAllStringFunc()` for efficient replacement
- For each match: Look up key in kvMap, replace if found, log warning if not found
- Return modified string with all references replaced
- Handle empty input gracefully (return as-is)

**Function: `ReplaceInMap(m map[string]interface{}, kvMap map[string]string, logger arbor.ILogger) error`**
- Recursive traversal of map structure
- For each value: Check type using type switch
  - `string`: Call `ReplaceKeyReferences()` and update in-place
  - `map[string]interface{}`: Recursive call to `ReplaceInMap()`
  - `[]interface{}`: Iterate and handle string elements or nested maps
  - Other types: Skip (no replacement needed)
- Log debug message for each replacement: `logger.Debug().Str("key", key).Str("old", old).Str("new", new).Msg("Replaced key reference")`
- Log warning for unresolved references: `logger.Warn().Str("key", key).Msg("Key not found in KV store")`
- Mutate map in-place (no return value needed)
- Return error only for critical failures (nil for missing keys)

**Function: `ReplaceInStruct(v interface{}, kvMap map[string]string, logger arbor.ILogger) error`**
- Use reflection to traverse struct fields: `reflect.ValueOf(v).Elem()`
- Iterate through fields: `for i := 0; i < val.NumField(); i++`
- For each field: Check if it's a string, struct, or map
  - String field: Call `ReplaceKeyReferences()` and set new value with `field.SetString()`
  - Nested struct: Recursive call to `ReplaceInStruct()`
  - Map field: Call `ReplaceInMap()`
- Only process exported fields (check `field.CanSet()`)
- Log debug message for each replacement
- Handle pointer fields: Dereference before processing
- Return error for reflection failures

**Function: `logUnresolvedKeys(input string, kvMap map[string]string, logger arbor.ILogger)`**
- Helper function to find all `{key-name}` references in string
- Check each against kvMap
- Log warning for missing keys: `logger.Warn().Str("reference", "{key-name}").Msg("Unresolved key reference in config")`
- Called by `ReplaceKeyReferences()` before replacement

**Imports:** `context`, `fmt`, `reflect`, `regexp`, `github.com/ternarybob/arbor`

**Documentation:**
- Add package comment explaining `{key-name}` syntax
- Document that replacement is case-sensitive
- Document that missing keys are logged but not treated as errors
- Add examples in function comments:
  ```go
  // Example: ReplaceKeyReferences("api_key = {google-api-key}", map[string]string{"google-api-key": "sk-123"})
  // Returns: "api_key = sk-123"
  ```

### internal\storage\sqlite\load_job_definitions.go(MODIFY)

References: 

- internal\common\replacement.go(NEW)

Integrate replacement into job definition loading:

**Modify `ToJobDefinition()` method (lines 43-79):**
- Add parameter: `kvStorage interfaces.KeyValueStorage` to method signature
- After line 76 (initialize empty maps), add replacement logic:
  1. Create context: `ctx := context.Background()`
  2. Fetch KV map: `kvMap, err := kvStorage.GetAll(ctx)`
  3. Handle error: If err != nil, log warning and skip replacement (graceful degradation)
  4. Replace in job-level config: `common.ReplaceInMap(jobDef.Config, kvMap, logger)` (need logger parameter too)
  5. Replace in each step config: Loop through `jobDef.Steps` and call `common.ReplaceInMap(step.Config, kvMap, logger)`
  6. Replace in string fields: `jobDef.BaseURL`, `jobDef.AuthID` using `common.ReplaceKeyReferences()`
- Add logger parameter to method signature: `ToJobDefinition(kvStorage interfaces.KeyValueStorage, logger arbor.ILogger)`

**Update `LoadJobDefinitionsFromFiles()` method (lines 84-166):**
- Line 131: Update call to `ToJobDefinition()` to pass `m.kv` and `m.logger`:
  ```go
  jobDef := jobFile.ToJobDefinition(m.kv, m.logger)
  ```
- Add log message after replacement: `m.logger.Debug().Str("job_def_id", jobDef.ID).Msg("Applied key/value replacements to job definition")`

**Add import:** `"github.com/ternarybob/quaero/internal/common"`

**Note:** This ensures all `{key-name}` references in job definitions are replaced before validation and storage.

### internal\common\config.go(MODIFY)

References: 

- internal\common\replacement.go(NEW)

Integrate replacement into config loading:

**Modify `LoadFromFiles()` function (lines 301-327):**
- Add parameter: `kvStorage interfaces.KeyValueStorage` to function signature
- After line 321 (TOML unmarshal loop), before line 324 (applyEnvOverrides), add replacement logic:
  1. Check if kvStorage is nil: If nil, skip replacement (backward compatibility)
  2. Create context: `ctx := context.Background()`
  3. Fetch KV map: `kvMap, err := kvStorage.GetAll(ctx)`
  4. Handle error: If err != nil, log warning and skip replacement
  5. Create logger: Use `arbor.NewLogger()` for logging (config loading is stateless)
  6. Replace in config struct: `common.ReplaceInStruct(config, kvMap, logger)`
  7. Log info message: `logger.Info().Int("keys", len(kvMap)).Msg("Applied key/value replacements to config")`

**Update `LoadFromFile()` function (lines 291-296):**
- Add parameter: `kvStorage interfaces.KeyValueStorage`
- Update call to `LoadFromFiles()` to pass kvStorage:
  ```go
  return LoadFromFiles(kvStorage, path)
  ```
- Update signature: `func LoadFromFile(kvStorage interfaces.KeyValueStorage, path string) (*Config, error)`

**Update function signature (line 301):**
- Change from: `func LoadFromFiles(paths ...string) (*Config, error)`
- Change to: `func LoadFromFiles(kvStorage interfaces.KeyValueStorage, paths ...string) (*Config, error)`

**Add import:** `"github.com/ternarybob/quaero/internal/interfaces"`

**Note:** This is a breaking change - all callers of `LoadFromFile()` and `LoadFromFiles()` must be updated to pass kvStorage parameter. The kvStorage parameter can be nil for backward compatibility (replacement will be skipped).

### internal\app\app.go(MODIFY)

References: 

- internal\common\config.go(MODIFY)
- internal\common\replacement.go(NEW)

Update config loading to pass KV storage:

**Locate config loading call:**
- Search for `common.LoadFromFile()` or `common.LoadFromFiles()` call in `New()` function or initialization code
- This is typically in the app initialization sequence

**Problem:** Config is loaded BEFORE storage is initialized (chicken-and-egg problem)

**Solution - Two-phase initialization:**

**Phase 1: Initial config load (no replacement):**
- Keep existing `common.LoadFromFile(configPath)` call as-is
- Pass `nil` for kvStorage parameter: `common.LoadFromFile(nil, configPath)`
- This loads config with `{key-name}` references intact

**Phase 2: Config replacement (after storage init):**
- After storage initialization (around line 228), add replacement step:
  1. Get KV storage: `kvStorage := a.StorageManager.KeyValueStorage()`
  2. Create context: `ctx := context.Background()`
  3. Fetch KV map: `kvMap, err := kvStorage.GetAll(ctx)`
  4. Handle error: If err != nil, log warning and skip replacement
  5. Replace in config: `common.ReplaceInStruct(a.Config, kvMap, a.Logger)`
  6. Log info: `a.Logger.Info().Msg("Applied key/value replacements to runtime config")`

**Alternative approach (if config is passed to New()):**
- If config is loaded in `main.go` before `app.New()`, update `main.go` instead
- Pass `nil` to `LoadFromFile()` initially
- After app initialization, call replacement manually

**Note:** This ensures config replacement happens after KV storage is available but before services that depend on config values (LLM, Agent, Places) are initialized.

### cmd\quaero\main.go(MODIFY)

References: 

- internal\common\config.go(MODIFY)

Update main.go to pass nil kvStorage during initial config load:

**Locate config loading:**
- Find call to `common.LoadFromFile(configPath)` in `main()` function
- This is typically near the start of main(), before app initialization

**Update call:**
- Change from: `config, err := common.LoadFromFile(configPath)`
- Change to: `config, err := common.LoadFromFile(nil, configPath)`
- Add comment explaining two-phase initialization:
  ```go
  // Phase 1: Load config without KV replacement (storage not initialized yet)
  // Phase 2: Replacement happens in app.New() after storage initialization
  config, err := common.LoadFromFile(nil, configPath)
  ```

**No other changes needed:**
- The actual replacement will happen in `app.New()` after storage is initialized
- This maintains backward compatibility and clean separation of concerns

**Note:** If there are multiple config loading calls (tests, CLI commands), update all of them to pass `nil` for kvStorage parameter.

### internal\common\replacement_test.go(NEW)

References: 

- internal\common\replacement.go(NEW)

Create comprehensive unit tests for replacement functions:

**Test: `TestReplaceKeyReferences_Simple`**
- Input: `"api_key = {google-api-key}"`
- KV map: `{"google-api-key": "sk-12345"}`
- Expected: `"api_key = sk-12345"`
- Verify exact string match

**Test: `TestReplaceKeyReferences_Multiple`**
- Input: `"key1={key1}, key2={key2}, key3={key3}"`
- KV map: `{"key1": "val1", "key2": "val2", "key3": "val3"}`
- Expected: `"key1=val1, key2=val2, key3=val3"`
- Verify all references replaced

**Test: `TestReplaceKeyReferences_MissingKey`**
- Input: `"api_key = {missing-key}"`
- KV map: `{"other-key": "value"}`
- Expected: `"api_key = {missing-key}"` (unchanged)
- Verify warning logged (use mock logger)

**Test: `TestReplaceKeyReferences_InvalidSyntax`**
- Input: `"api_key = {invalid key}"` (space in key name)
- KV map: `{"invalid key": "value"}`
- Expected: `"api_key = {invalid key}"` (unchanged, doesn't match regex)
- Verify no replacement for invalid syntax

**Test: `TestReplaceKeyReferences_EmptyInput`**
- Input: `""`
- KV map: `{"key": "value"}`
- Expected: `""`
- Verify empty string handled gracefully

**Test: `TestReplaceInMap_SimpleString`**
- Input map: `{"api_key": "{google-api-key}"}`
- KV map: `{"google-api-key": "sk-12345"}`
- Expected: `{"api_key": "sk-12345"}`
- Verify in-place mutation

**Test: `TestReplaceInMap_NestedMap`**
- Input map: `{"llm": {"api_key": "{google-api-key}"}}`
- KV map: `{"google-api-key": "sk-12345"}`
- Expected: `{"llm": {"api_key": "sk-12345"}}`
- Verify recursive replacement

**Test: `TestReplaceInMap_MixedTypes`**
- Input map: `{"string": "{key1}", "int": 42, "bool": true, "nested": {"key": "{key2}"}}`
- KV map: `{"key1": "val1", "key2": "val2"}`
- Expected: `{"string": "val1", "int": 42, "bool": true, "nested": {"key": "val2"}}`
- Verify only strings replaced, other types unchanged

**Test: `TestReplaceInMap_ArrayOfStrings`**
- Input map: `{"urls": ["{url1}", "{url2}", "static-url"]}`
- KV map: `{"url1": "http://example1.com", "url2": "http://example2.com"}`
- Expected: `{"urls": ["http://example1.com", "http://example2.com", "static-url"]}`
- Verify array elements replaced

**Test: `TestReplaceInStruct_SimpleFields`**
- Input struct: `Config{LLM: LLMConfig{GoogleAPIKey: "{google-api-key}"}}`
- KV map: `{"google-api-key": "sk-12345"}`
- Expected: `Config{LLM: LLMConfig{GoogleAPIKey: "sk-12345"}}`
- Verify nested struct fields replaced

**Test: `TestReplaceInStruct_MultipleFields`**
- Input struct: `Config{LLM: LLMConfig{GoogleAPIKey: "{llm-key}"}, Agent: AgentConfig{GoogleAPIKey: "{agent-key}"}}`
- KV map: `{"llm-key": "sk-111", "agent-key": "sk-222"}`
- Expected: Both fields replaced with correct values
- Verify multiple nested structs handled

**Test: `TestReplaceInStruct_UnexportedFields`**
- Input struct with unexported field: `type Test struct { exported string; unexported string }`
- Verify unexported fields skipped (no panic)

**Test: `TestReplaceInStruct_PointerFields`**
- Input struct with pointer field: `Config{ErrorTolerance: &ErrorTolerance{FailureAction: "{action}"}}`
- KV map: `{"action": "stop_all"}`
- Expected: Pointer dereferenced and field replaced
- Verify nil pointers handled gracefully

**Test: `TestIntegration_JobDefinitionReplacement`**
- Create `JobDefinitionFile` with `{key-name}` references in Config and Steps[].Config
- Call `ToJobDefinition()` with mock KV storage
- Verify all references replaced in resulting `JobDefinition`
- Verify validation passes after replacement

**Test: `TestIntegration_ConfigReplacement`**
- Create TOML string with `{key-name}` references in LLM, Agent, PlacesAPI sections
- Unmarshal to Config struct
- Call `ReplaceInStruct()` with mock KV map
- Verify all API key fields replaced
- Verify other fields unchanged

**Helper functions:**
- `createMockLogger() arbor.ILogger` - Returns mock logger for testing
- `createTestKVMap() map[string]string` - Returns standard test KV map
- `assertStringReplaced(t *testing.T, input, expected, actual string)` - Assertion helper

**Imports:** `testing`, `github.com/stretchr/testify/assert`, `github.com/stretchr/testify/require`, `github.com/ternarybob/arbor`

**Note:** Use table-driven tests where appropriate to reduce code duplication.