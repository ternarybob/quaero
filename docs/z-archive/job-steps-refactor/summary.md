# Complete: Job Steps TOML Refactor

## Classification
- Type: feature
- Location: ./docs/feature/job-steps-refactor/

This refactor simplifies job step configuration by introducing a new `[step.{name}]` table format with flat configuration fields, replacing the nested `[[steps]]` array format with `[steps.config]` and `[steps.config.document_filter]` sub-tables.

## Stats
Tasks: 7 | Files: ~30 | Duration: ~15min
Models: Planning=opus, Workers=sonnet

## Key Changes

### New TOML Format
```toml
# OLD FORMAT
[[steps]]
name = "extract_keywords"
action = "agent"
[steps.config]
agent_type = "keyword_extractor"
[steps.config.document_filter]
limit = 100

# NEW FORMAT
[step.extract_keywords]
action = "agent"
agent_type = "keyword_extractor"
filter_limit = 100
```

### Code Changes

1. **JobStep Model** (`internal/models/job_definition.go`)
   - Added `Depends` field for step dependencies (comma-separated step names)

2. **TOML Parsing** (`internal/jobs/service.go`)
   - Added support for `[step.{name}]` tables via `map[string]JobStepFile`
   - Uses `toml:",remain"` to capture extra fields into flat config map

3. **Validation** (`internal/models/job_definition.go`)
   - Updated `ValidateStep` to validate flat `filter_*` fields
   - Added `Depends` validation (referenced steps exist, no self-dependency)

4. **Managers** (`internal/queue/managers/agent_manager.go`)
   - Updated to read flat `filter_*` fields instead of nested `document_filter`

### TOML Files Updated
- All files in `test/config/job-definitions/`
- All files in `bin/job-definitions/`
- All files in `deployments/local/job-definitions/`

## Benefits
- Simpler, flatter TOML structure (KISS principle)
- Step name in table key eliminates redundant `name` field
- Filter fields use `filter_*` prefix for clarity
- New `depends` field enables step dependencies within a job
- Breaking change - old `[[steps]]` format removed (aggressive development)

## Verify
- go build: ✅
- go test ./internal/queue/...: ✅ All tests pass
