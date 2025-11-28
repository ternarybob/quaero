# Task 4: Create Job Definition TOML

- Group: 4 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 3
- Sandbox: /tmp/3agents/task-4/ | Source: C:\development\quaero | Output: C:\development\quaero\docs\plans\web-search

## Files
- `test/config/job-definitions/web-search-asx.toml` - Create new file

## Requirements

Create a job definition for testing web search with the ASX:GNP query:

```toml
# Web Search Job Definition
# Uses Gemini SDK with GoogleSearch grounding to search the web

id = "web-search-asx-gnp"
name = "Web Search: ASX:GNP Company Info"
type = "web_search"
job_type = "user"
description = "Search for latest information on ASX listed company ASX:GNP"
tags = ["web-search", "asx", "stocks", "gnp"]

# Cron schedule (empty = manual execution only)
schedule = ""

# Maximum execution time
timeout = "5m"

# Whether this job is enabled
enabled = true

# Whether to auto-start when scheduler initializes
auto_start = false

# Job steps definition
[[steps]]
name = "search_asx_gnp"
action = "web_search"
on_error = "fail"

[steps.config]
# Natural language search query
query = "find me latest information on ASX listed company ASX:GNP"

# API Key (loaded from variables)
api_key = "{google_api_key}"

# Search parameters
depth = 3    # Number of follow-up exploration queries (max 10)
breadth = 3  # Number of results per query (max 5)
```

## Acceptance
- [ ] Valid TOML syntax
- [ ] Uses correct job type "web_search"
- [ ] Correct step action "web_search"
- [ ] Depth < 10, breadth < 5
- [ ] References API key variable
