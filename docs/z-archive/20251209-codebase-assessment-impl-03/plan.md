# Plan: Fix Codebase Assessment Pipeline Implementation
Type: fix | Workdir: ./docs/fix/20251209-codebase-assessment-impl-03/

## User Intent (from manifest)
Make the Codebase Assessment Pipeline functional:
1. Fix the placeholder `{project_name}` issue - tags should use concrete values or be properly substituted
2. Implement or wire up the missing agent types that the pipeline requires
3. Ensure the pipeline can process documents and complete successfully

## Analysis Summary

### Root Causes Identified:
1. **Agent types not implemented**: Only `keyword_extractor` is implemented. The pipeline needs:
   - `metadata_enricher` - for extract_build_info step
   - `category_classifier` - for classify_files step
   - `entity_recognizer` - for identify_components step

2. **Placeholder resolution**: `{project_name}` should be resolved from KV storage but requires the key to exist. The UAT job definition uses placeholders that need to be set via UI/API before running.

3. **Test vs UAT configs differ**:
   - Test config (`test/config/job-definitions/codebase_assess.toml`) uses concrete "test-project"
   - UAT config (`bin/job-definitions/codebase_assess.toml`) uses `{project_name}` placeholder

## Tasks
| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Implement metadata_enricher agent | - | no | sonnet |
| 2 | Implement category_classifier agent | - | no | sonnet |
| 3 | Implement entity_recognizer agent | - | no | sonnet |
| 4 | Register all three agents in agent service | 1,2,3 | no | sonnet |
| 5 | Update UAT job definition to use concrete project name or document handling | - | no | sonnet |
| 6 | Run test and verify pipeline completes | 4,5 | no | sonnet |

## Order
[1,2,3] → [4] → [5] → [6]

## Implementation Notes
- All three agents follow the same pattern as `keyword_extractor.go`
- Each agent must implement `AgentExecutor` interface: `Execute()` and `GetType()`
- Registration happens in `internal/services/agents/service.go` NewService()
- Agent types are already in the valid list in `agent_worker.go` - no changes needed there
