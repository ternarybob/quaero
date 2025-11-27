# Task 6: Add Warning Logging for Load Failures

- Group: 6 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 5
- Sandbox: /tmp/3agents/task-6/ | Source: . | Output: docs/fixes/

## Files
- `internal/storage/badger/load_connectors.go` - Enhance warning logging

## Requirements
Ensure the `LoadConnectorsFromFiles` function logs appropriate warnings:

1. When a connector file exists but fails to parse:
   - Log warning with file name and error

2. When a connector has invalid type:
   - Log warning with connector name and invalid type

3. When a connector has empty token:
   - Log warning with connector name

4. Add summary log at end:
   - Log info with loaded count, skipped count, and error count

Example log output:
```
WARN: Failed to parse connector file: connectors.toml: invalid TOML syntax at line 5
WARN: Skipping connector 'invalid': unknown type 'unknown_type', valid types are: github, gitlab
WARN: Skipping connector 'github': token is required
INFO: Finished loading connectors from files loaded=2 skipped=1 errors=1
```

## Acceptance
- [ ] Warning logged for parse failures
- [ ] Warning logged for invalid connector types
- [ ] Warning logged for missing required fields
- [ ] Summary log shows counts
- [ ] Compiles
- [ ] Tests pass
