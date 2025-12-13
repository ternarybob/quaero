# Task 3: Add Interface Method to StorageManager

- Group: 3 | Mode: concurrent | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 2
- Sandbox: /tmp/3agents/task-3/ | Source: . | Output: docs/fixes/

## Files
- `internal/interfaces/storage.go` - Add method to StorageManager interface

## Requirements
Add `LoadConnectorsFromFiles` method to the `StorageManager` interface:

```go
// LoadConnectorsFromFiles loads connectors from TOML files in the specified directory
// This is used to load connector configurations at startup
LoadConnectorsFromFiles(ctx context.Context, dirPath string) error
```

Add this method after the existing `LoadJobDefinitionsFromFiles` method in the interface.

## Acceptance
- [ ] LoadConnectorsFromFiles method added to StorageManager interface
- [ ] Proper documentation comment added
- [ ] Compiles
- [ ] Tests pass
