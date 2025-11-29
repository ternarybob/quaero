# Task 4: Add Manager Method Implementation

- Group: 4 | Mode: concurrent | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 2,3
- Sandbox: /tmp/3agents/task-4/ | Source: . | Output: docs/fixes/

## Files
- `internal/storage/badger/manager.go` - Add LoadConnectorsFromFiles method

## Requirements
Add the `LoadConnectorsFromFiles` method to the `Manager` struct to satisfy the interface:

```go
// LoadConnectorsFromFiles loads connectors from TOML files
func (m *Manager) LoadConnectorsFromFiles(ctx context.Context, dirPath string) error {
    return LoadConnectorsFromFiles(ctx, m.connector, dirPath, m.logger)
}
```

Add this method after the existing `LoadJobDefinitionsFromFiles` method (around line 107).

## Acceptance
- [ ] LoadConnectorsFromFiles method added to Manager
- [ ] Method calls the standalone function with proper parameters
- [ ] Compiles
- [ ] Tests pass
