# Step 1: Update JobStep Model

- Task: task-1.md | Group: 1 | Model: opus

## Actions
1. Created `internal/models/step_type.go` with StepType enum
2. Added 9 step type constants (agent, crawler, places_search, web_search, github_repo, github_actions, transform, reindex, database_maintenance)
3. Added `Type` and `Description` fields to JobStep struct
4. Updated ValidateStep() to require Type field
5. Added IsValid() and String() methods to StepType
6. Added AllStepTypes() helper function
7. Updated existing tests to include Type field

## Files
- `internal/models/step_type.go` - NEW: StepType enum with validation
- `internal/models/job_definition.go` - Updated JobStep struct
- `internal/models/job_definition_test.go` - Updated tests

## Decisions
- Keep Action field temporarily for backward compatibility
- StepType as string type for TOML serialization compatibility

## Verify
Compile: ✅ | Tests: ✅

## Status: ✅ COMPLETE
