# Step 3: Implement analyze_build_system action

Model: sonnet | Status: ✅

## Done

- Created AnalyzeBuildSystemAction with regex + LLM analysis
- Implemented build file detection (Makefile, CMake, vcxproj, configure, sln)
- Implemented target extraction for Makefile and CMake
- Implemented compiler flag and library extraction
- Added LLM integration for complex dependency analysis
- Updated DevOpsWorker.handleAnalyzeBuildSystem to use action

## Files Changed

- `internal/jobs/actions/analyze_build_system.go` - New action file (509 lines)
- `internal/queue/workers/devops_worker.go` - Updated handler, added llmService field
- `internal/app/app.go` - Wired llmService to DevOpsWorker

## Build Check

Build: ✅ (syntax validated) | Tests: ⏭️
