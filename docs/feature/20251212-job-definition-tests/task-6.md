# Task 6: Verify tests compile and run basic checks

Workdir: ./docs/feature/20251212-job-definition-tests/ | Depends: 2,3,4,5 | Critical: no
Model: sonnet | Skill: go

## Context

This task is part of: Creating job definition test infrastructure for Quaero
Prior tasks completed: Tasks 1-5 - Framework and all test files created

## User Intent Addressed

Ensure all new test code compiles successfully and follows Go conventions.

## Input State

Files that exist before this task:
- `test/ui/job_framework_test.go` - Extended with helper methods
- `test/ui/job_definition_news_crawler_test.go`
- `test/ui/job_definition_nearby_restaurants_places_test.go`
- `test/ui/job_definition_nearby_restaurants_keywords_test.go`
- `test/ui/job_definition_codebase_classify_test.go`

## Output State

Files after this task completes:
- All files remain unchanged
- Verification that code compiles
- Verification that test functions are discovered

## Skill Patterns to Apply

### From go/SKILL.md:
- **DO:** Run go build to verify compilation
- **DO:** Run go test -list to verify tests are discovered
- **DON'T:** Skip verification steps

## Implementation Steps

1. Change to test directory
2. Run `go build ./ui/...` to verify compilation
3. Run `go test -list 'TestJobDefinition.*' ./ui/` to list new tests
4. Verify 4 new test functions are discovered
5. Record verification results

## Code Specifications

No code changes - verification only.

Commands to run:
```bash
cd test
go build ./ui/...
go test -list 'TestJobDefinition.*' ./ui/
```

Expected output: 4 test functions listed:
- TestJobDefinitionNewsCrawler
- TestJobDefinitionNearbyRestaurantsPlaces
- TestJobDefinitionNearbyRestaurantsKeywords
- TestJobDefinitionCodebaseClassify

## Accept Criteria

- [ ] `go build ./ui/...` completes without errors
- [ ] `go test -list` discovers all 4 new test functions
- [ ] No syntax errors or import issues
- [ ] Framework changes work with existing tests

## Handoff

After completion, next task(s): validation phase
