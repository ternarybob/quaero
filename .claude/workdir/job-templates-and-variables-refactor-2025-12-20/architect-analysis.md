# ARCHITECT ANALYSIS: Job Templates & Variables Refactor

## Task Summary

Three sub-tasks:
1. Move variables from `{exe}/variables/variables.toml` to `{exe}/variables.toml`
2. Create job template system for stock investigation jobs with variable replacement
3. Create job template tests in `test/api/` and `test/ui/`

---

## TASK 1: Move Variables File

### Current State
- Variables stored in: `bin/variables/variables.toml`
- Loaded by: `internal/storage/badger/load_variables.go:LoadVariablesFromFiles()`
- Config reference: `internal/common/config.go:211` - `Dir: "./variables"`
- Similar to: `email.toml` and `connectors.toml` which are in `bin/` root

### DECISION: MODIFY (not CREATE)

The system already loads from a directory (`./variables/*.toml`). We can:
1. Move `bin/variables/variables.toml` → `bin/variables.toml`
2. Update config default `Dir: "./variables"` → `Dir: "./"`
3. Keep loader as-is (it loads ALL `.toml` from directory)

**ALTERNATIVE**: Just move the file without changing config - the loader already loads ALL `.toml` files. If we move to root, we need config change.

**BETTER APPROACH**: Keep directory structure, just flatten path expectation. Actually, looking closer:
- `email.toml` is at `bin/email.toml` with dedicated loader
- `connectors.toml` is at `bin/connectors.toml` with dedicated loader
- `variables/*.toml` is a directory with generic loader

The user wants `variables.toml` to be at `{exe}/variables.toml` like the others. This means:
1. Move `bin/variables/variables.toml` → `bin/variables.toml`
2. Delete `bin/variables/` directory
3. Update loader to load single file OR update config to point to `./` directory

**SIMPLEST**: Change config from `Dir: "./variables"` to `Dir: "./"` and move the file. The loader loads ALL .toml from that dir, so it will pick up `variables.toml`.

**RISK**: This would also try to load `email.toml` and `connectors.toml` as variable files. That's problematic because they have different structures.

**BEST APPROACH**: Create dedicated `load_variables_file.go` similar to `load_email.go` pattern, OR modify the loader to accept a file path not just directory.

Actually, looking at the user request again: "Move variables from {exe}/variables/variables.toml to {exe}/variables.toml. Like email.toml and connectors.toml"

This means they want a SINGLE FILE loader like email/connectors have.

**DECISION**: MODIFY existing loader to support both directory AND single file loading, then change config.

---

## TASK 2: Job Template System

### Current State - Stock Jobs Analysis

5 near-identical jobs in `bin/job-definitions/`:
- `web-search-asx-cba.toml` - CBA (Commonwealth Bank)
- `web-search-asx-exr.toml` - EXR (Elixir Energy)
- `web-search-asx-srl.toml` - SRL (Sunrise Energy)
- `web-search-asx-wes.toml` - WES (Wesfarmers)
- `web-search-asx.toml` - Generic

### What Varies Per Stock:
1. `id` - e.g., `"web-search-asx-cba"`
2. `name` - e.g., `"ASX:CBA Investment Analysis"`
3. `asx_code` - e.g., `"CBA"`
4. `output_tags` - e.g., `["cba", "asx-cba-data"]`
5. `filter_tags` - e.g., `["cba"]`
6. `query` strings - company-specific search terms
7. `tags` at job level - e.g., `["web-search", "asx", "stocks", "cba"]`
8. `body_from_tag` - e.g., `"asx-cba-summary"`
9. Subject lines in emails

### Existing Variable Substitution
- `internal/common/replacement.go` - `ReplaceKeyReferences()`
- Pattern: `{variable_name}` syntax
- Already used in job configs for API keys: `{google_gemini_api_key}`

### DECISION: EXTEND (not CREATE)

The variable replacement system already exists! We can:
1. Create template TOML files in `bin/job-templates/`
2. Use enhanced syntax: `{stock:ticker}`, `{stock:name}`, `{stock:lowercase_ticker}`
3. Create new worker type: `WorkerTypeTemplateOrchestrator = "template_orchestrator"`
4. This worker reads template, applies variables, creates child job definition, executes it

### Template Worker Design

```go
type WorkerTypeTemplateOrchestrator WorkerType = "template_orchestrator"
```

Config in job definition:
```toml
id = "asx-stocks-daily"
name = "ASX Stock Analysis (All Stocks)"
type = "template_orchestrator"
enabled = true

[step.run_templates]
type = "template_orchestrator"
template = "asx-stock-analysis"  # References bin/job-templates/asx-stock-analysis.toml
variables = [
    { ticker = "CBA", name = "Commonwealth Bank", industry = "banking" },
    { ticker = "WES", name = "Wesfarmers", industry = "retail" },
    { ticker = "EXR", name = "Elixir Energy", industry = "energy" },
]
```

Template file (`bin/job-templates/asx-stock-analysis.toml`):
```toml
id = "web-search-asx-{ticker:lowercase}"
name = "ASX:{ticker} Investment Analysis"
...
asx_code = "{ticker}"
output_tags = ["{ticker:lowercase}", "asx-{ticker:lowercase}-data"]
```

### Implementation Plan

1. Create new worker type in `worker_type.go`
2. Create template loader in `internal/jobs/` or `internal/storage/badger/`
3. Create `template_orchestrator` worker in `internal/queue/workers/`
4. Worker flow:
   - Load template TOML from `job-templates/` directory
   - For each variable set, substitute placeholders
   - Create ephemeral job definition (not persisted)
   - Execute as child job via job service
   - Wait for completion or track in parallel

---

## TASK 3: Tests

### API Test (`test/api/`)
- Follow pattern from `test/api/jobs_test.go`
- Test template loading, variable substitution, job creation

### UI Test (`test/ui/`)
- Follow pattern from `test/ui/job_definition_web_search_asx_test.go`
- Test UI for managing templates (if UI changes needed)

---

## Files to Modify/Create

### MODIFY
1. `internal/storage/badger/load_variables.go` - Support single file loading
2. `internal/common/config.go` - Update Variables config
3. `internal/models/worker_type.go` - Add TemplateOrchestrator
4. `internal/queue/workers/registry.go` - Register new worker

### CREATE (with justification)
1. `bin/job-templates/asx-stock-analysis.toml` - Template file
   - **Justification**: No existing template system, this is new functionality requested
2. `internal/queue/workers/template_orchestrator.go` - New worker
   - **Justification**: New worker type for template orchestration
3. `test/api/job_template_test.go` - API tests
   - **Justification**: Explicitly requested by user
4. `test/ui/job_template_test.go` - UI tests
   - **Justification**: Explicitly requested by user

### DELETE
1. `bin/variables/` directory (after moving variables.toml)

### MOVE
1. `bin/variables/variables.toml` → `bin/variables.toml`

---

## Anti-Creation Bias Compliance

All creations justified:
- Template file: New feature, no existing equivalent
- Worker: New type, extends existing worker pattern
- Tests: Explicitly requested

All modifications follow existing patterns:
- Worker type registration follows existing pattern
- Variable loading extends existing loader
- Tests follow existing test patterns

---

## Build Requirement

Will run: `./scripts/build.sh` after each change phase.

---

## Risk Assessment

1. **LOW**: Moving variables.toml - single file move + config update
2. **MEDIUM**: Template orchestrator - new worker type but follows existing patterns
3. **LOW**: Tests - follow existing patterns exactly
