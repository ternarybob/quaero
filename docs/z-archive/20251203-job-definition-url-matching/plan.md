# Plan

## Task 1: Add `url_patterns` field to Job Definition TOML parsing

**File**: `internal/jobs/service.go`

Add `url_patterns` field to `JobDefinitionFile` struct for TOML parsing:
```go
UrlPatterns []string `toml:"url_patterns"` // URL patterns for matching (wildcards: *.domain.com/*)
```

Propagate to `models.JobDefinition` via `ToJobDefinition()`.

**File**: `internal/models/job_definition.go`

Add `UrlPatterns` field to `JobDefinition` struct:
```go
UrlPatterns []string `json:"url_patterns"` // URL patterns for automatic job matching
```

## Task 2: Create Confluence Job Definition TOML

**Files**:
- `bin/job-definitions/confluence-crawler.toml`
- `deployments/local/job-definitions/confluence-crawler.toml`
- `test/config/job-definitions/confluence-crawler.toml`

Content:
```toml
id = "confluence-crawler"
name = "Confluence Crawler"
type = "crawler"
description = "Default crawler configuration for Atlassian Confluence sites"
enabled = true

# URL patterns for automatic matching (wildcards supported)
url_patterns = ["*.atlassian.net/wiki/*", "*.atlassian.net/confluence/*"]

[step.crawl]
type = "crawler"
max_depth = 2
max_pages = 20
concurrency = 3
follow_links = true

# Confluence-specific patterns
include_patterns = ["/wiki/spaces/", "/wiki/display/", "/pages/"]
exclude_patterns = [
    "/login", "/logout", "/authenticate",
    "/plugins/", "/download/", "/rest/",
    "action=edit", "action=history"
]
```

## Task 3: Implement URL Pattern Matching in Quick Crawl Handler

**File**: `internal/handlers/job_definition_handler.go`

Add function to find matching job definition:
```go
// findMatchingJobDefinition searches crawler job definitions for URL pattern matches
func (h *JobDefinitionHandler) findMatchingJobDefinition(ctx context.Context, targetURL string) (*models.JobDefinition, error)
```

Pattern matching logic:
1. List all crawler-type job definitions with `url_patterns` set
2. For each pattern, convert wildcard to regex (`*` → `.*`)
3. Match against target URL
4. Return first match (or nil if no match)

Modify `CreateAndExecuteQuickCrawlHandler`:
1. After parsing URL, call `findMatchingJobDefinition()`
2. If match found:
   - Use matched job definition's config (max_depth, max_pages, patterns)
   - Override `start_urls` with the requested URL
   - Set/refresh authentication credentials
3. If no match: use current ad-hoc job creation logic

## Task 4: Ensure Authentication Integration

Authentication flow remains unchanged:
1. Cookies from extension are always stored (refresh existing credentials)
2. `auth_id` is set on the job definition
3. When job runs, crawler worker injects cookies

Key points:
- Store credentials with deterministic ID: `auth:generic:{siteDomain}`
- Set `AuthID` on the matched (or new) job definition
- For matched jobs, create a copy with updated `start_urls` and `auth_id`

## Task 5: Add Tests

**File**: `test/api/quick_crawl_url_matching_test.go`

Test cases:
1. URL matching finds correct job definition
2. Wildcards work correctly (*.domain.com/*)
3. No match returns nil (falls back to ad-hoc)
4. Config from matched job is used
5. Authentication is applied to matched job

## Implementation Order

1. Task 1: Add `url_patterns` field to model/parsing
2. Task 2: Create Confluence TOML files
3. Task 3: Implement URL matching in handler
4. Task 4: Verify authentication (mostly existing code)
5. Task 5: Add tests
