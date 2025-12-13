# Plan: Fix Connector Loading from TOML Files

## Analysis

### Problem Statement
Connectors defined in `bin/connectors/connectors.toml` are not being loaded at startup. The Settings UI shows "No Connectors" despite the TOML file containing valid connector definitions.

### Root Cause
After analyzing the codebase:
1. **Missing Loader Function**: There is NO `LoadConnectorsFromFiles` function in the codebase
2. **Pattern Exists for Other Resources**:
   - `LoadVariablesFromFiles` exists in `internal/storage/badger/load_variables.go`
   - `LoadJobDefinitionsFromFiles` exists in `internal/storage/badger/load_job_definitions.go`
   - `LoadEnvFile` exists in `internal/storage/badger/load_env.go`
3. **Configuration Exists**: `Config.Connectors.Dir` (default: `./connectors`) is defined but never used
4. **No Warning**: When connector files exist but fail to load, no warning is shown

### Current TOML Format (connectors.toml)
```toml
[github]
type = "github"
token = "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

[gitlab]
type = "gitlab"
token = "something-goes-here"
```

### Expected Model Structure
```go
type Connector struct {
    ID        string          `json:"id"`
    Name      string          `json:"name"`
    Type      ConnectorType   `json:"type"`
    Config    json.RawMessage `json:"config"`
    CreatedAt time.Time       `json:"created_at"`
    UpdatedAt time.Time       `json:"updated_at"`
}
```

### Dependencies
- `internal/storage/badger/manager.go` - needs LoadConnectorsFromFiles method
- `internal/interfaces/storage.go` - needs interface method
- `internal/app/app.go` - needs to call loader during init
- `internal/models/connector.go` - has model, may need new connector types

### Risks
- Low: Adding new loader follows existing pattern
- Low: Changes are additive, no breaking changes
- Medium: Need to handle TOML format mapping to model structure

## Groups

| Task | Desc | Depends | Critical | Complexity | Model |
|------|------|---------|----------|------------|-------|
| 1 | Add ConnectorType for GitLab | none | no | low | sonnet |
| 2 | Create LoadConnectorsFromFiles function | 1 | no | medium | sonnet |
| 3 | Add interface method to StorageManager | 2 | no | low | sonnet |
| 4 | Add Manager method implementation | 2,3 | no | low | sonnet |
| 5 | Call loader in app.go initDatabase | 3,4 | no | low | sonnet |
| 6 | Add warning logging for load failures | 5 | no | low | sonnet |
| 7 | Create API tests for connector loading | 5 | no | medium | sonnet |
| 8 | Create UI tests for connector display | 5 | no | medium | sonnet |

## Order
Sequential: [1] -> [2] -> [3,4] (concurrent) -> [5] -> [6] -> [7,8] (concurrent) -> Review

## Files to Modify/Create

1. `internal/models/connector.go` - Add GitLab connector type
2. `internal/storage/badger/load_connectors.go` - NEW: Create connector loader
3. `internal/interfaces/storage.go` - Add LoadConnectorsFromFiles interface method
4. `internal/storage/badger/manager.go` - Add LoadConnectorsFromFiles method
5. `internal/app/app.go` - Call LoadConnectorsFromFiles during init
6. `test/api/connector_loading_test.go` - NEW: API tests
7. `test/ui/connector_ui_test.go` - NEW: UI tests
