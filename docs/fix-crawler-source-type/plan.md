# Plan: Fix Crawler Source Type Issue

## Problem Analysis

**Log Errors:**
1. `ERR invalid source type:  (must be one of: jira, confluence, github, web)`
2. `ERR sql: Scan error on column index 1, name "completed_children": converting NULL to int is unsupported`

**Root Cause:**
- CrawlerStepExecutor passes `jobDef.SourceType` to crawler service (line 92)
- JobDefinition model has `SourceType` field but it's empty for generic crawler jobs
- news-crawler.toml doesn't specify `source_type` field
- Crawler service validates source_type and rejects empty strings (service.go:297-301)

**Architecture Issue:**
The recent places job implementation added a job-type agnostic approach, but the crawler still expects source-specific types. This breaks the "non-context specific" principle for jobs.

## Steps

1. **Add default source_type for crawler jobs**
   - Skill: @code-architect
   - Files: `internal/jobs/executor/crawler_step_executor.go`
   - User decision: no
   - When job definition doesn't have source_type, default to "web" for generic crawling
   - This maintains backward compatibility while supporting source-specific jobs

2. **Add source_type to news-crawler job definition**
   - Skill: @none
   - Files: `deployments/local/job-definitions/news-crawler.toml`
   - User decision: no
   - Add `source_type = "web"` to the job definition config
   - Document that generic crawler jobs should use "web" as source_type

3. **Fix NULL handling for completed_children SQL query**
   - Skill: @go-coder
   - Files: `internal/jobs/manager.go` or `internal/storage/sqlite/job_storage.go`
   - User decision: no
   - Use COALESCE in SQL query to handle NULL values gracefully
   - Change: `COUNT(...)` → `COALESCE(COUNT(...), 0)`

4. **Add validation and helpful error messages**
   - Skill: @go-coder
   - Files: `internal/jobs/executor/crawler_step_executor.go`
   - User decision: no
   - Log warning if source_type is missing
   - Log info about defaulting to "web"
   - Improve error messages for debugging

5. **Test crawler with news-crawler job**
   - Skill: @test-writer
   - Files: Manual testing (no automated test needed)
   - User decision: no
   - Build and run application
   - Execute news-crawler job
   - Verify no errors in logs
   - Verify job completes successfully

## Success Criteria
- news-crawler job executes without "invalid source_type" error
- SQL NULL errors resolved for completed_children
- Generic crawler jobs work with default "web" source_type
- Source-specific jobs (jira, confluence, github) continue working
- Logs show clear messages about source_type defaulting
- Job-type agnostic architecture maintained

## Design Note
The solution maintains the job-type agnostic principle by:
- Defaulting to "web" source_type for generic crawlers
- Allowing explicit source_type override in job definitions
- Supporting both generic and source-specific crawling
- No breaking changes to existing job definitions
