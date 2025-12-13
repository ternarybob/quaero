# Plan: Remove All SQLite References

## Overview
Remove ALL SQLite/sql.DB references from the codebase. The storage backend is Badger only.
Breaking changes are acceptable.

## Dependency Analysis

The SQLite references fall into these categories:
1. **Code comments/dead code** - References in comments, unused parameters
2. **Config - Type field** - Storage type field that's redundant (only badger)
3. **Test configs** - TOML files with `type = "badger"` (redundant field)
4. **Documentation** - Comments mentioning SQLite
5. **Validation service** - Has unused UpdateValidationStatus with sql.DB

## Execution Groups

### Group 1 (Sequential) - Config Changes
1. **Remove storage.type from Config struct and TOML files**
   - Skill: @go-coder | Files: internal/common/config.go
   - Critical: yes:api-breaking | Depends: none
   - Changes: Remove `Type string` from StorageConfig, remove env override

### Group 2 (Parallel) - Code Cleanup
2a. **Clean SQLite references in banner.go**
    - Skill: @go-coder | Files: internal/common/banner.go
    - Critical: no | Depends: 1

2b. **Clean SQLite comments in handlers**
    - Skill: @go-coder | Files: internal/handlers/job_definition_handler.go
    - Critical: no | Depends: 1

2c. **Clean validation service - remove UpdateValidationStatus**
    - Skill: @go-coder | Files: internal/services/validation/toml_validation_service.go
    - Critical: no | Depends: 1

2d. **Clean model comments**
    - Skill: @go-coder | Files: internal/models/document.go
    - Critical: no | Depends: 1

2e. **Clean integration test**
    - Skill: @go-coder | Files: internal/common/replacement_integration_test.go
    - Critical: no | Depends: 1

2f. **Clean database maintenance worker**
    - Skill: @go-coder | Files: internal/queue/workers/database_maintenance_worker.go
    - Critical: no | Depends: 1

2g. **Clean search service comments**
    - Skill: @go-coder | Files: internal/services/search/*.go
    - Critical: no | Depends: 1

2h. **Clean app.go comments**
    - Skill: @go-coder | Files: internal/app/app.go
    - Critical: no | Depends: 1

### Group 3 (Parallel) - TOML Config Cleanup
3a. **Clean bin/quaero.toml**
    - Skill: @none | Files: bin/quaero.toml
    - Critical: no | Depends: 1

3b. **Clean test/config/*.toml**
    - Skill: @none | Files: test/config/*.toml
    - Critical: no | Depends: 1

3c. **Clean deployments/docker config**
    - Skill: @none | Files: deployments/docker/config.offline.example.toml
    - Critical: no | Depends: 1

3d. **Clean deployments/local config**
    - Skill: @none | Files: deployments/local/quaero.toml
    - Critical: no | Depends: 1

### Group 4 (Sequential) - Build & Test
4. **Build and verify compilation**
   - Skill: @go-coder | Files: all
   - Critical: yes | Depends: 2a-2h, 3a-3d

5. **Run API tests**
   - Skill: @test-writer | Files: test/api/
   - Critical: yes | Depends: 4

6. **Run UI tests**
   - Skill: @test-writer | Files: test/ui/
   - Critical: yes | Depends: 5

## Execution Map
```
[1] ──┬──> [2a] ──┬
     ├──> [2b] ──┤
     ├──> [2c] ──┤
     ├──> [2d] ──┤
     ├──> [2e] ──┤
     ├──> [2f] ──┤
     ├──> [2g] ──┤
     ├──> [2h] ──┤
     ├──> [3a] ──┤
     ├──> [3b] ──┤
     ├──> [3c] ──┤
     └──> [3d] ──┴──> [4] ──> [5] ──> [6] ──> [Final Review]
```

## Success Criteria
- No SQLite/sql.DB references in code (except go.sum)
- No storage.type field in config
- Build passes: `go build ./...`
- API tests pass: `go test ./test/api/...`
- UI tests pass: `go test ./test/ui/...`
