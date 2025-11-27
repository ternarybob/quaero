# Task 2: Create LoadConnectorsFromFiles Function

- Group: 2 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 1
- Sandbox: /tmp/3agents/task-2/ | Source: . | Output: docs/fixes/

## Files
- `internal/storage/badger/load_connectors.go` - NEW file

## Requirements
Create a new file `load_connectors.go` following the pattern of `load_variables.go`:

1. Define `ConnectorFile` struct for TOML parsing:
```go
// ConnectorFile represents a connector in TOML format
// Format:
// [connector_name]
// type = "github"
// token = "ghp_xxx"
type ConnectorFile struct {
    Type  string `toml:"type"`
    Token string `toml:"token"`
}
```

2. Create `LoadConnectorsFromFiles` function that:
   - Checks if directory exists (skip silently if not)
   - Reads all .toml files in the directory
   - For each file, parses sections as connectors
   - Maps TOML section name to connector Name and ID
   - Stores connector using ConnectorStorage.SaveConnector
   - Logs warnings for parse errors but continues processing
   - Returns nil (non-fatal errors logged as warnings)

3. The function should:
   - Use connector section name as both ID and Name
   - Map type field to ConnectorType
   - Store token in Config as JSON
   - Set CreatedAt/UpdatedAt to current time

## Acceptance
- [ ] New file `load_connectors.go` created
- [ ] ConnectorFile struct defined
- [ ] LoadConnectorsFromFiles function handles missing directory
- [ ] LoadConnectorsFromFiles parses TOML correctly
- [ ] LoadConnectorsFromFiles stores connectors
- [ ] Logs appropriate warnings on errors
- [ ] Compiles
- [ ] Tests pass
