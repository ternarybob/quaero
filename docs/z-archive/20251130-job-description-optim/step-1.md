# Step 1: Fix test/config crawler jobs missing steps
- Task: task-1.md | Group: 1 | Model: opus

## Actions
1. Converted my-custom-crawler.toml to step-based structure
2. Converted news-crawler.toml to step-based structure
3. Removed deprecated type/job_type fields from news-crawler.toml

## Files
- `test/config/job-definitions/my-custom-crawler.toml` - Added [step.crawl] section with crawler config
- `test/config/job-definitions/news-crawler.toml` - Added [step.crawl_news] section, removed deprecated fields

## Decisions
- Step name "crawl" for generic crawler: Simple and clear
- Step name "crawl_news" for news crawler: Descriptive of purpose

## Verify
Compile: N/A (TOML config) | Tests: ✅

## Status: ✅ COMPLETE
