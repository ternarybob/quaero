# Task 2: Create news-crawler job definition test

Workdir: ./docs/feature/20251212-job-definition-tests/ | Depends: 1 | Critical: no
Model: sonnet | Skill: go

## Context

This task is part of: Creating job definition test infrastructure for Quaero
Prior tasks completed: Task 1 - Framework helper methods added

## User Intent Addressed

Create test for news-crawler job definition that runs end-to-end with monitoring and screenshots.

## Input State

Files that exist before this task:
- `test/ui/job_framework_test.go` - UITestContext with RunJobDefinitionTest method
- `test/config/job-definitions/news-crawler.toml` - News crawler job definition

## Output State

Files after this task completes:
- `test/ui/job_definition_news_crawler_test.go` - Complete test file for news crawler

## Skill Patterns to Apply

### From go/SKILL.md:
- **DO:** Use table-driven tests where appropriate
- **DO:** Wrap errors with context using %w
- **DO:** Put integration tests in test/ui/
- **DON'T:** Use global state

## Implementation Steps

1. Create `test/ui/job_definition_news_crawler_test.go`
2. Define TestJobDefinitionNewsCrawler function
3. Configure JobDefinitionTestConfig for news-crawler:
   - JobName: "News Crawler"
   - JobDefinitionPath: "../config/job-definitions/news-crawler.toml"
   - Timeout: 10 minutes (max_pages=10, depth=2)
   - RequiredEnvVars: none (no API keys needed)
   - AllowFailure: false
4. Call RunJobDefinitionTest with config
5. Log success

## Code Specifications

```go
package ui

import (
    "testing"
    "time"
)

// TestJobDefinitionNewsCrawler tests the News Crawler job definition end-to-end
func TestJobDefinitionNewsCrawler(t *testing.T) {
    utc := NewUITestContext(t, 15*time.Minute)
    defer utc.Cleanup()

    utc.Log("--- Testing Job Definition: News Crawler ---")

    config := JobDefinitionTestConfig{
        JobName:           "News Crawler",
        JobDefinitionPath: "../config/job-definitions/news-crawler.toml",
        Timeout:           10 * time.Minute,
        RequiredEnvVars:   nil, // No API keys needed
        AllowFailure:      false,
    }

    if err := utc.RunJobDefinitionTest(config); err != nil {
        t.Fatalf("Job definition test failed: %v", err)
    }

    utc.Log("✓ News Crawler job definition test completed successfully")
}
```

## Accept Criteria

- [ ] File `test/ui/job_definition_news_crawler_test.go` exists
- [ ] Test function TestJobDefinitionNewsCrawler defined
- [ ] Uses JobDefinitionTestConfig with correct values for news-crawler
- [ ] Timeout set to 10 minutes
- [ ] No required env vars (crawler doesn't need API keys)
- [ ] Code compiles: `go build ./test/ui/...`

## Handoff

After completion, next task(s): 6 (verification)
