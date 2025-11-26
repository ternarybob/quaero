# Plan: Actions->Jobs Domain Rename & AI->Agent Type Refactor

## Summary

Two refactoring tasks:
1. **Rename "Actions Domain" to "Jobs Domain"**: `internal/actions` -> `internal/jobs`
2. **Refactor job type "ai" to "agent"**: `JobDefinitionTypeAI` -> `JobDefinitionTypeAgent`

## Current State Analysis

### Task 1: Actions -> Jobs Rename

**Current Structure:**
```
internal/actions/
└── definitions/
    └── orchestrator.go
```

**Files importing `internal/actions`:**
- `internal/handlers/job_definition_handler.go`
- `internal/app/app.go`

**Documentation to update:**
- `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md`
- `internal/actions/README.md` -> `internal/jobs/README.md`
- `internal/queue/README.md` (references actions)

### Task 2: AI -> Agent Type Refactor

**Files with `JobDefinitionTypeAI` or `"ai"` job type:**
- `internal/models/job_definition.go:33` - `JobDefinitionTypeAI = "ai"`
- `internal/models/job_definition.go:48` - validation switch
- `internal/models/job_definition.go:264` - AI-specific validation
- `internal/actions/definitions/orchestrator.go:199` - type check

**TOML files using `type = "ai"`:**
- `deployments/local/job-definitions/agent-*.toml`
- `deployments/local/job-definitions/keyword-extractor-agent.toml`
- `bin/job-definitions/*.toml`
- `test/config/job-definitions/*.toml`

## Execution Groups

### Group 1: Rename actions -> jobs (Folder & Imports)

1a. **Rename folder internal/actions to internal/jobs**
    - Files: `internal/actions/` -> `internal/jobs/`
    - Action: git mv or rename folder

1b. **Update package declaration in orchestrator.go**
    - File: `internal/jobs/definitions/orchestrator.go`
    - Action: Keep `package definitions` (no change needed)

1c. **Update imports in app.go**
    - File: `internal/app/app.go`
    - Action: Change `internal/actions/definitions` -> `internal/jobs/definitions`

1d. **Update imports in job_definition_handler.go**
    - File: `internal/handlers/job_definition_handler.go`
    - Action: Change `internal/actions/definitions` -> `internal/jobs/definitions`

### Group 2: Refactor AI -> Agent type

2a. **Update job_definition.go model**
    - File: `internal/models/job_definition.go`
    - Actions:
      - Rename `JobDefinitionTypeAI` -> `JobDefinitionTypeAgent`
      - Change value from `"ai"` -> `"agent"`
      - Update validation switch case
      - Update AI-specific validation to "Agent-specific"
      - Update error messages and comments

2b. **Update orchestrator.go type check**
    - File: `internal/jobs/definitions/orchestrator.go`
    - Action: Change `JobDefinitionTypeAI` -> `JobDefinitionTypeAgent`

2c. **Update TOML files type field**
    - Files: All agent TOML files
    - Action: Change `type = "ai"` -> `type = "agent"`

### Group 3: Update Documentation

3a. **Update MANAGER_WORKER_ARCHITECTURE.md**
    - Replace "Actions Domain" with "Jobs Domain"
    - Update folder paths from `internal/actions/` to `internal/jobs/`
    - Update all references to "actions"

3b. **Rename and update internal/actions/README.md**
    - Move to `internal/jobs/README.md`
    - Replace "Actions Domain" with "Jobs Domain"
    - Update all "actions" references to "jobs"

3c. **Update internal/queue/README.md**
    - Replace "Actions" references with "Jobs"
    - Update links to jobs README

### Group 4: Verification

4. **Build and test**
    - Run `go build ./...`
    - Run `go test ./internal/models/...`

## Execution Map

```
[1a] ──> [1b] ──> [1c,1d] (parallel) ──┐
                                        ├──> [4]
[2a] ──> [2b] ──> [2c] ────────────────┤
                                        │
[3a,3b,3c] (parallel) ─────────────────┘
```

## Success Criteria

- `internal/jobs/` folder exists with orchestrator
- All imports updated to `internal/jobs/definitions`
- `JobDefinitionTypeAgent = "agent"` in models
- All TOML files use `type = "agent"`
- Documentation uses "Jobs Domain" terminology
- Build succeeds
- Tests pass
