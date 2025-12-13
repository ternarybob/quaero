# Task 1: Fix test/config crawler jobs missing steps
- Group: 1 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: none
- Sandbox: /tmp/3agents/task-1/ | Source: ./ | Output: ./docs/feature/20251130-job-description-optim/

## Files
- `test/config/job-definitions/my-custom-crawler.toml` - Add step section, move crawler config to step
- `test/config/job-definitions/news-crawler.toml` - Add step section, move crawler config to step, remove deprecated fields

## Requirements

### my-custom-crawler.toml
Convert flat crawler config to step-based structure:
- Keep: id, name, description, schedule, timeout, enabled, auto_start
- Move to `[step.crawl]`: start_urls, include_patterns, exclude_patterns, max_depth, max_pages, concurrency, follow_links
- Add type = "crawler" inside the step

### news-crawler.toml
Convert flat crawler config to step-based structure:
- Keep: id, name, description, schedule, timeout, enabled, auto_start
- Remove: type, job_type (deprecated)
- Move to `[step.crawl_news]`: start_urls, include_patterns, exclude_patterns, max_depth, max_pages, concurrency, follow_links
- Add type = "crawler" and description inside the step

## Acceptance
- [ ] my-custom-crawler.toml has [step.crawl] section with crawler config
- [ ] news-crawler.toml has [step.crawl_news] section with crawler config
- [ ] No deprecated type/job_type fields at root level
- [ ] Files are valid TOML
