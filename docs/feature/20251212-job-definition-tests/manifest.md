# Feature: Job Definition Tests

- Slug: job-definition-tests | Type: feature | Date: 2025-12-12
- Request: "Create job definition tests for new-crawler, nearby-restaurants-*, and codebase_classify. Extract from existing job_* tests, create new named tests with common run/template that monitors jobs until complete, takes screenshots, and uploads job definitions to results dir. Tests should use common code for startup/monitor/screenshots."
- Prior: none

## User Intent

Create a dedicated test infrastructure for testing specific job definitions end-to-end. The tests should:

1. **Structure**: Create new test files with `job_definition_{name}_test.go` naming convention in `test/ui/`
2. **Common Framework**: Extend existing `job_framework_test.go` with shared utilities for:
   - Starting a job and monitoring until completion (without page refresh)
   - Taking screenshots at key stages (start, status changes, completion)
   - Copying the job definition TOML to the test results directory
   - Page refresh after completion and final screenshot
3. **Job-Specific Tests**: Individual tests for:
   - `news-crawler` - crawler job
   - `nearby-restaurants-places` - places API job
   - `nearby-restaurants-keywords` - multi-step places + keywords job
   - `codebase_classify` - local directory import + code map + rule classification
4. **Extensibility**: Design for easy addition of more job definition tests

## Success Criteria

- [ ] Common `JobDefinitionTest` helper struct/methods in `job_framework_test.go`
- [ ] `job_definition_news_crawler_test.go` - tests news crawler job end-to-end
- [ ] `job_definition_nearby_restaurants_places_test.go` - tests places API job
- [ ] `job_definition_nearby_restaurants_keywords_test.go` - tests multi-step job
- [ ] `job_definition_codebase_classify_test.go` - tests codebase analysis pipeline
- [ ] Each test copies its job definition TOML to results directory
- [ ] Screenshots captured: job started, status changes, completion, post-refresh
- [ ] Tests use shared monitoring code from framework
- [ ] Tests compile and pass `go build ./test/ui/...`

## Skills Assessment

| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | ✅ | ✅ | Go UI test code with chromedp, table-driven tests |
| frontend | .claude/skills/frontend/SKILL.md | ✅ | ❌ | Tests only, no frontend changes |

**Active Skills:** go
