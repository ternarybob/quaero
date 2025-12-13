# Complete: Job Definition Tests

Type: feature | Tasks: 6 | Files: 5

## User Request

"Create job definition tests for new-crawler, nearby-restaurants-*, and codebase_classify. Extract from existing job_* tests, create new named tests with common run/template that monitors jobs until complete, takes screenshots, and uploads job definitions to results dir."

## Result

Created a comprehensive job definition test infrastructure with:
- **Common framework** in `job_framework_test.go` with `JobDefinitionTestConfig`, `RunJobDefinitionTest()`, `CopyJobDefinitionToResults()`, and `RefreshAndScreenshot()` methods
- **4 individual test files** following `job_definition_{name}_test.go` naming convention
- **Full test lifecycle**: trigger job, monitor until completion, capture screenshots at key stages, copy TOML to results, refresh and final screenshot

## Files Created/Modified

| File | Action | Lines |
|------|--------|-------|
| `test/ui/job_framework_test.go` | modified | +140 |
| `test/ui/job_definition_news_crawler_test.go` | created | 29 |
| `test/ui/job_definition_nearby_restaurants_places_test.go` | created | 29 |
| `test/ui/job_definition_nearby_restaurants_keywords_test.go` | created | 29 |
| `test/ui/job_definition_codebase_classify_test.go` | created | 29 |

## Skills Used

- go (error handling with %w, chromedp UI testing, test infrastructure)

## Validation: ✅ MATCHES

All 9 success criteria met:
- Common helper methods in framework
- 4 test files created
- TOML copying to results
- Screenshot capture throughout
- Shared monitoring code
- Build verification passed

## Review: N/A

No critical triggers detected (no security, authentication, or architectural changes).

## Verify

Build: ✅ `go build ./test/ui/...` | Tests: ✅ 4 tests discovered

## Run Tests

```bash
# Run individual test
cd test
go test -v ./ui -run TestJobDefinitionNewsCrawler

# Run all job definition tests
go test -v ./ui -run 'TestJobDefinition.*'
```

Note: Tests requiring API keys will skip gracefully when keys are not set.
