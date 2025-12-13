# Task 5: Call Loader in app.go initDatabase

- Group: 5 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 3,4
- Sandbox: /tmp/3agents/task-5/ | Source: . | Output: docs/fixes/

## Files
- `internal/app/app.go` - Add call to LoadConnectorsFromFiles

## Requirements
Add the connector loading call in `initDatabase()` method, after the job definitions loading (around line 264):

```go
// Load connectors from files
if err := a.StorageManager.LoadConnectorsFromFiles(context.Background(), a.Config.Connectors.Dir); err != nil {
    // Log warning but don't fail startup (consistent with other loaders)
    a.Logger.Warn().Err(err).Msg("Failed to load connectors from files")
}
```

This should be added AFTER job definitions loading (line 260-264) and BEFORE the config replacement phase (line 266).

## Acceptance
- [ ] LoadConnectorsFromFiles called in initDatabase
- [ ] Uses Config.Connectors.Dir for directory path
- [ ] Warning logged on error but doesn't fail startup
- [ ] Compiles
- [ ] Tests pass
