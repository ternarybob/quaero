# Plan: Merge Crawler Executor Files and Migrate to Worker Pattern

## Steps

1. **Create Merged Crawler Worker File**
   - Skill: @code-architect
   - Files: `internal/jobs/worker/crawler_worker.go` (NEW)
   - User decision: no
   - Description: Merge crawler_executor.go (1034 lines) and crawler_executor_auth.go (495 lines) into single crawler_worker.go file. Apply all transformations: package processor→worker, struct CrawlerExecutor→CrawlerWorker, constructor rename, receiver rename (e→w), organize into 5 logical sections (interface methods, config/rendering, auth, child job management, event publishing).

2. **Update App Registration and Imports**
   - Skill: @go-coder
   - Files: `internal/app/app.go`
   - User decision: no
   - Description: Add worker package import, update CrawlerWorker registration (processor.NewCrawlerExecutor→worker.NewCrawlerWorker), rename variable crawlerExecutor→crawlerWorker. Keep other executors unchanged.

3. **Add Deprecation Notices to Old Files**
   - Skill: @go-coder
   - Files: `internal/jobs/processor/crawler_executor.go`, `internal/jobs/processor/crawler_executor_auth.go`
   - User decision: no
   - Description: Add detailed deprecation comments at top of both files explaining merge, migration to worker package, and removal in ARCH-008.

4. **Update Architecture Documentation**
   - Skill: @none
   - Files: `AGENTS.md`, `docs/architecture/MANAGER_WORKER_ARCHITECTURE.md`
   - User decision: no
   - Description: Document ARCH-005 completion, update migration status, add crawler_worker.go to new directory listing, update remaining file counts, add detailed section on file merge process.

5. **Compile and Validate**
   - Skill: @go-coder
   - Files: All modified files
   - User decision: no
   - Description: Verify crawler_worker.go compiles independently, build full application, verify startup logs show "Crawler URL worker registered for job type: crawler_url", run go test compilation check.

## Success Criteria

- Single `crawler_worker.go` file exists in `internal/jobs/worker/` (~1529 lines)
- CrawlerWorker struct implements `worker.JobWorker` interface (Execute, GetWorkerType, Validate)
- `app.go` successfully imports and uses `worker.NewCrawlerWorker()`
- Application compiles cleanly with no errors
- Deprecation notices added to both old processor files
- Documentation updated to reflect ARCH-005 completion
- Conditional auth logic preserved (authStorage nil check)
- All 8 dependencies preserved in struct
- injectAuthCookies() method successfully merged as private method
