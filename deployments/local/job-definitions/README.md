# Job Definitions Directory

This directory contains user-defined crawler job definitions that are automatically loaded at startup.

## Overview

Quaero supports defining custom crawler jobs using TOML or JSON files. Simply place your job definition files in this directory, and they will be loaded automatically when the service starts.

## File Format

### TOML Format (Recommended)

```toml
id = "my-crawler"
name = "My Custom Crawler"
description = "Crawls my favorite website"

# Initial URLs to crawl
start_urls = [
    "https://example.com/start"
]

# Schedule (cron format, empty = manual only)
schedule = ""

# Timeout duration
timeout = "30m"

# Job settings
enabled = true
auto_start = false

# URL filtering (regex patterns)
include_patterns = ["article", "post"]
exclude_patterns = ["login", "admin"]

# Crawler configuration
max_depth = 2
max_pages = 100
concurrency = 5
follow_links = true
```

### JSON Format

```json
{
  "id": "my-crawler",
  "name": "My Custom Crawler",
  "description": "Crawls my favorite website",
  "start_urls": ["https://example.com/start"],
  "schedule": "",
  "timeout": "30m",
  "enabled": true,
  "auto_start": false,
  "include_patterns": ["article", "post"],
  "exclude_patterns": ["login", "admin"],
  "max_depth": 2,
  "max_pages": 100,
  "concurrency": 5,
  "follow_links": true
}
```

## Field Reference

### Required Fields

- **`id`** (string): Unique identifier for the job (lowercase, hyphens allowed)
- **`name`** (string): Human-readable job name
- **`start_urls`** (array): Initial URLs to begin crawling

### Optional Fields

- **`description`** (string): Job description (default: "")
- **`schedule`** (string): Cron expression for scheduling (default: "" = manual only)
  - Examples: `"*/5 * * * *"` (every 5 minutes), `"0 0 * * *"` (daily at midnight)
- **`timeout`** (string): Maximum execution time (default: "30m")
  - Examples: `"30m"`, `"1h"`, `"2h30m"`
- **`enabled`** (boolean): Whether job is enabled (default: true)
- **`auto_start`** (boolean): Auto-start on scheduler init (default: false)

### Crawler Configuration

- **`include_patterns`** (array): Regex patterns for URLs to include (default: [])
  - If empty, all URLs are included (subject to exclude patterns)
- **`exclude_patterns`** (array): Regex patterns for URLs to exclude (default: [])
- **`max_depth`** (integer): Maximum crawl depth (default: 2)
- **`max_pages`** (integer): Maximum pages to crawl (default: 100)
- **`concurrency`** (integer): Number of concurrent workers (default: 5)
- **`follow_links`** (boolean): Whether to follow discovered links (default: true)

## Examples

### Simple News Crawler

```toml
id = "tech-news"
name = "Tech News Crawler"
description = "Crawls technology news articles"
start_urls = ["https://technews.example.com"]
include_patterns = ["article", "news"]
max_depth = 1
max_pages = 50
concurrency = 3
follow_links = true
```

### Scheduled Crawler

```toml
id = "daily-blog-crawler"
name = "Daily Blog Crawler"
description = "Crawls blog posts daily"
start_urls = ["https://blog.example.com"]
schedule = "0 0 * * *"  # Daily at midnight
timeout = "1h"
enabled = true
auto_start = true
max_depth = 2
max_pages = 200
concurrency = 10
follow_links = true
```

### Deep Crawl with Filtering

```toml
id = "documentation-crawler"
name = "Documentation Crawler"
description = "Crawls documentation pages"
start_urls = ["https://docs.example.com"]
include_patterns = ["^/docs/", "^/api/"]
exclude_patterns = ["login", "logout", "admin", "edit"]
max_depth = 5
max_pages = 1000
concurrency = 10
follow_links = true
```

## Usage

1. **Create a job definition file** in this directory (`.toml` or `.json`)
2. **Restart the Quaero service** to load the new job
3. **View the job** in the web UI at http://localhost:8085/jobs
4. **Run the job** manually or wait for scheduled execution

## Configuration

The job definitions directory can be configured in `quaero.toml`:

```toml
[jobs]
definitions_dir = "./job-definitions"  # Default location
```

## Notes

- **Idempotent Loading**: Jobs with the same `id` won't be duplicated
- **Validation**: Invalid job definitions are logged and skipped (won't fail startup)
- **File Types**: Both `.toml` and `.json` files are supported
- **Hot Reload**: Not supported - requires service restart to load new jobs
- **Hardcoded Jobs**: File-based jobs are loaded after hardcoded jobs (database-maintenance, stockhead-crawler)

## Troubleshooting

**Job not appearing in UI:**
- Check service logs for validation errors
- Ensure file has `.toml` or `.json` extension
- Verify `id` is unique and doesn't conflict with existing jobs
- Confirm `enabled = true` in job definition

**Job fails to execute:**
- Check `start_urls` are valid and accessible
- Verify regex patterns in `include_patterns` and `exclude_patterns`
- Ensure `max_depth`, `max_pages`, and `concurrency` are positive integers
- Check service logs for detailed error messages

