# Plan: Job Definition Tests

Type: feature | Workdir: ./docs/feature/20251212-job-definition-tests/ | Date: 2025-12-12

## Context

Project: Quaero
Related files:
- `test/ui/job_framework_test.go` - Existing UI test framework with UITestContext
- `test/ui/job_types_test.go` - Existing job type tests (partial)
- `test/config/job-definitions/*.toml` - Job definition TOML files
- `test/common/setup.go` - Test environment setup

## User Intent (from manifest)

Create a dedicated test infrastructure for testing specific job definitions end-to-end. The tests should:
1. Use `job_definition_{name}_test.go` naming convention in `test/ui/`
2. Extend framework with shared utilities for monitoring, screenshots, TOML copying
3. Create individual tests for: news-crawler, nearby-restaurants-places, nearby-restaurants-keywords, codebase_classify
4. Design for easy extension to more job definitions

## Success Criteria (from manifest)

- [ ] Common `JobDefinitionTest` helper struct/methods in `job_framework_test.go`
- [ ] `job_definition_news_crawler_test.go` - tests news crawler job end-to-end
- [ ] `job_definition_nearby_restaurants_places_test.go` - tests places API job
- [ ] `job_definition_nearby_restaurants_keywords_test.go` - tests multi-step job
- [ ] `job_definition_codebase_classify_test.go` - tests codebase analysis pipeline
- [ ] Each test copies its job definition TOML to results directory
- [ ] Screenshots captured: job started, status changes, completion, post-refresh
- [ ] Tests use shared monitoring code from framework
- [ ] Tests compile and pass `go build ./test/ui/...`

## Active Skills

| Skill | Key Patterns to Apply |
|-------|----------------------|
| go | Table-driven tests, error wrapping with %w, context.Context on all I/O |

## Technical Approach

1. **Extend UITestContext** in `job_framework_test.go` with:
   - `RunJobDefinitionTest()` - orchestrates full test: trigger, monitor, screenshots, copy TOML
   - `CopyJobDefinitionToResults()` - copies TOML file to test results directory
   - `RefreshAndScreenshot()` - page refresh then screenshot

2. **Create individual test files** following naming convention `job_definition_{name}_test.go`:
   - Each test calls the shared `RunJobDefinitionTest()` with job-specific config
   - Tests check API key availability and skip if not present
   - Tests specify timeouts appropriate for job type

3. **Screenshot strategy**:
   - On job trigger
   - On each status change (via existing MonitorJob)
   - After completion
   - After page refresh

## Files to Change

| File | Action | Purpose |
|------|--------|---------|
| `test/ui/job_framework_test.go` | modify | Add JobDefinitionTestConfig, RunJobDefinitionTest, CopyJobDefinitionToResults |
| `test/ui/job_definition_news_crawler_test.go` | create | Test for news-crawler job |
| `test/ui/job_definition_nearby_restaurants_places_test.go` | create | Test for nearby-restaurants-places job |
| `test/ui/job_definition_nearby_restaurants_keywords_test.go` | create | Test for multi-step places+keywords job |
| `test/ui/job_definition_codebase_classify_test.go` | create | Test for codebase analysis pipeline |

## Tasks

| # | Desc | Depends | Critical | Model | Skill | Est. Files |
|---|------|---------|----------|-------|-------|------------|
| 1 | Add JobDefinitionTestConfig and helper methods to framework | - | no | sonnet | go | 1 |
| 2 | Create news-crawler job definition test | 1 | no | sonnet | go | 1 |
| 3 | Create nearby-restaurants-places job definition test | 1 | no | sonnet | go | 1 |
| 4 | Create nearby-restaurants-keywords job definition test | 1 | no | sonnet | go | 1 |
| 5 | Create codebase_classify job definition test | 1 | no | sonnet | go | 1 |
| 6 | Verify tests compile and run basic checks | 2,3,4,5 | no | sonnet | go | 0 |

## Execution Order

[1] → [2,3,4,5] → [6]

## Risks/Decisions

- **API Keys**: Tests requiring Google Places/Gemini API keys should skip gracefully when not available
- **Timeouts**: codebase_classify has 4h timeout in definition but tests should use reasonable test timeout
- **File Paths**: codebase_classify uses absolute Windows paths - may need adjustment for test environment
