# Plan: Job Steps TOML Refactor

## Classification
- Type: feature
- Workdir: ./docs/feature/job-steps-refactor/

## Analysis

### Current Format
```toml
[[steps]]
name = "step_name"
action = "web_search"
on_error = "fail"

[steps.config]
query = "search query"
api_key = "{google_gemini_api_key}"

[steps.config.document_filter]
limit = 100
```

### Target Format
```toml
[step.step_name]
action = "web_search"
on_error = "fail"
depends = ""  # comma-separated step names for dependencies
query = "search query"
api_key = "{google_gemini_api_key}"
filter_limit = 100
filter_tags = ["tag1", "tag2"]
```

### Key Changes
1. `[[steps]]` array → `[step.{name}]` tables (name in key, not field)
2. `[steps.config]` nested → flat config directly in step table
3. `[steps.config.document_filter]` → `filter_*` prefixed fields
4. New `depends` field for step dependencies
5. Remove redundant `name` field (now in table key)
6. Keep `action`, `on_error`, `description` (optional)

### Dependencies
- `internal/models/job_definition.go` - JobStep struct
- `internal/jobs/service.go` - TOML parsing (JobStepFile, ToJobDefinition)
- `internal/models/job_definition.go` - ValidateStep function
- All managers that read step.Config

### Risks
- Breaking change to all existing TOML files
- Step execution order logic needs careful handling for dependencies

## Groups

| Task | Desc | Depends | Critical | Complexity | Model |
|------|------|---------|----------|------------|-------|
| 1 | Update JobStep model to add Depends field | none | no | low | sonnet |
| 2 | Refactor TOML parsing for new [step.name] format | 1 | no | high | sonnet |
| 3 | Update step validation for flat config and depends | 2 | no | medium | sonnet |
| 4 | Update managers to read flat config fields | 3 | no | medium | sonnet |
| 5 | Update test TOML files to new format | 4 | no | low | sonnet |
| 6 | Update bin/ and deployments/ TOML files | 5 | no | low | sonnet |
| 7 | Remove deprecated code and clean up | 6 | no | low | sonnet |

## Order
Sequential: [1] → [2] → [3] → [4] → [5] → [6] → [7]

All tasks are sequential due to dependencies between model, parsing, validation, and config files.
