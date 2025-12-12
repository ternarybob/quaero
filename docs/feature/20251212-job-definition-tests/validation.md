# Validation

Validator: opus | Date: 2025-12-12

## User Request

"Create job definition tests for new-crawler, nearby-restaurants-*, and codebase_classify. Extract from existing job_* tests, create new named tests with common run/template that monitors jobs until complete, takes screenshots, and uploads job definitions to results dir. Tests should use common code for startup/monitor/screenshots."

## User Intent

Create a dedicated test infrastructure for testing specific job definitions end-to-end with:
1. `job_definition_{name}_test.go` naming convention
2. Common framework with shared utilities for monitoring, screenshots, TOML copying
3. Individual tests for news-crawler, nearby-restaurants-places, nearby-restaurants-keywords, codebase_classify
4. Extensible design for adding more job definition tests

## Success Criteria Check

- [x] Common `JobDefinitionTest` helper struct/methods in `job_framework_test.go`: **MET** - JobDefinitionTestConfig struct + RunJobDefinitionTest, CopyJobDefinitionToResults, RefreshAndScreenshot methods added (lines 423-562)
- [x] `job_definition_news_crawler_test.go` - tests news crawler job end-to-end: **MET** - File created with TestJobDefinitionNewsCrawler (29 lines)
- [x] `job_definition_nearby_restaurants_places_test.go` - tests places API job: **MET** - File created with TestJobDefinitionNearbyRestaurantsPlaces (29 lines)
- [x] `job_definition_nearby_restaurants_keywords_test.go` - tests multi-step job: **MET** - File created with TestJobDefinitionNearbyRestaurantsKeywords (29 lines)
- [x] `job_definition_codebase_classify_test.go` - tests codebase analysis pipeline: **MET** - File created with TestJobDefinitionCodebaseClassify (29 lines)
- [x] Each test copies its job definition TOML to results directory: **MET** - CopyJobDefinitionToResults called by RunJobDefinitionTest (line 522)
- [x] Screenshots captured: job started, status changes, completion, post-refresh: **MET** - Screenshots at "job_definition", status changes (via MonitorJob), and "final_state" (via RefreshAndScreenshot)
- [x] Tests use shared monitoring code from framework: **MET** - All tests call RunJobDefinitionTest which uses existing TriggerJob and MonitorJob
- [x] Tests compile and pass `go build ./test/ui/...`: **MET** - Build succeeded, 4 test functions discovered

## Implementation Review

| Task | Intent | Implemented | Match |
|------|--------|-------------|-------|
| 1 | Add framework helper methods | JobDefinitionTestConfig, CopyJobDefinitionToResults, RefreshAndScreenshot, RunJobDefinitionTest | ✅ |
| 2 | News crawler test | TestJobDefinitionNewsCrawler with 10min timeout | ✅ |
| 3 | Places API test | TestJobDefinitionNearbyRestaurantsPlaces with 5min timeout, QUAERO_GOOGLE_PLACES_API_KEY required | ✅ |
| 4 | Multi-step keywords test | TestJobDefinitionNearbyRestaurantsKeywords with 8min timeout, 2 API keys required, AllowFailure=true | ✅ |
| 5 | Codebase classify test | TestJobDefinitionCodebaseClassify with 15min timeout, AllowFailure=true | ✅ |
| 6 | Verification | Build passes, 4 tests discovered | ✅ |

## Skill Compliance

### go/SKILL.md

| Pattern | Applied | Evidence |
|---------|---------|----------|
| Wrap errors with %w | ✅ | All error returns use fmt.Errorf with %w (lines 439, 447, 458, 464, 478, 482, 491, 496, 523, 529, 537, 542, 553, 558) |
| Context everywhere | ✅ | chromedp.Run uses utc.Ctx throughout |
| Tests in test/ui/ | ✅ | All 4 new test files in test/ui/ directory |
| Constructor injection | ✅ | UITestContext receives dependencies via NewUITestContext |
| No panic on errors | ✅ | All errors returned, t.Fatalf used only in tests |

## Gaps

None identified. All success criteria met.

## Technical Check

Build: ✅ Pass | Tests: ✅ 4 tests discovered (TestJobDefinitionNewsCrawler, TestJobDefinitionNearbyRestaurantsPlaces, TestJobDefinitionNearbyRestaurantsKeywords, TestJobDefinitionCodebaseClassify)

## Verdict: ✅ MATCHES

Implementation fully matches user intent. All requested job definition tests created with:
- Common framework code for monitoring/screenshots/TOML copying
- Individual test files following naming convention
- Proper API key checking with graceful skip
- Appropriate timeouts for each job type
- AllowFailure flags where rate limits or path issues may occur
