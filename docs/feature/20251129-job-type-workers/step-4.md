# Step 4: Update TOML Parsing

- Task: task-4.md | Group: 4 | Model: sonnet

## Actions
1. Updated ToJobDefinition() to return error for validation
2. Added type and description field parsing from TOML
3. Implemented action-to-type mapping for backward compatibility
4. Added comprehensive validation for type field
5. Updated callers to handle error return
6. Added 6 new test cases for parsing

## Files
- `internal/jobs/service.go` - Updated parsing with type support
- `internal/handlers/job_definition_handler.go` - Handle error return
- `internal/storage/badger/load_job_definitions.go` - Handle error return
- `internal/jobs/service_test.go` - Added comprehensive tests

## Decisions
- Type field takes precedence over action field
- Action field still populated for backward compatibility
- mapActionToStepType() handles 13 action values
- Clear error messages with valid options listed

## Action-to-Type Mapping
- agent, create_summaries, scan_documents, enrich → StepTypeAgent
- crawl → StepTypeCrawler
- places_search → StepTypePlacesSearch
- web_search → StepTypeWebSearch
- github_repo_fetch → StepTypeGitHubRepo
- github_actions_fetch → StepTypeGitHubActions
- transform → StepTypeTransform
- reindex → StepTypeReindex
- database_maintenance → StepTypeDatabaseMaintenance

## Verify
Compile: ✅ | Tests: ✅ (6/6 pass)

## Status: ✅ COMPLETE
