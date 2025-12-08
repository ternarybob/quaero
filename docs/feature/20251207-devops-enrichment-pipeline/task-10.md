# Task 10: Write unit tests for all actions

Depends: 2,3,4,5,6 | Critical: no | Model: sonnet

## Addresses User Intent

Ensure each action works correctly in isolation before integration testing.

## Do

- Create `internal/jobs/actions/extract_structure_test.go`:
  - Test include extraction (local vs system)
  - Test define extraction
  - Test platform detection patterns
  - Test with malformed/edge case files
  - Test skip logic for non-C/C++ files

- Create `internal/jobs/actions/analyze_build_system_test.go`:
  - Test Makefile target extraction
  - Test CMakeLists.txt parsing
  - Test vcxproj parsing
  - Test flag and library extraction
  - Test with complex multi-target Makefiles

- Create `internal/jobs/actions/classify_devops_test.go`:
  - Mock LLM service
  - Test JSON response parsing
  - Test error handling for malformed LLM responses
  - Test metadata merge with existing Pass 1 data

- Create `internal/jobs/actions/build_dependency_graph_test.go`:
  - Test edge creation from includes
  - Test path normalization
  - Test component aggregation
  - Test with circular includes

- Create `internal/jobs/actions/aggregate_devops_summary_test.go`:
  - Mock LLM service
  - Test markdown generation
  - Test document creation

## Accept

- [ ] All 5 test files created
- [ ] Tests cover positive cases
- [ ] Tests cover edge cases
- [ ] Tests cover error scenarios
- [ ] All unit tests pass
