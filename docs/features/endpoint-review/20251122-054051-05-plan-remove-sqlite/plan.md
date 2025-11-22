# Plan: Remove SQLite Storage Entirely from Codebase

## Steps

1. **Create Badger-based queue manager to replace goqite**
   - Skill: @code-architect
   - Files: `internal/queue/badger_manager.go` (NEW), `internal/interfaces/queue_service.go`
   - User decision: no

2. **Refactor JobManager to use storage interfaces**
   - Skill: @go-coder
   - Files: `internal/jobs/manager.go`
   - User decision: no

3. **Update app initialization to use Badger-only storage**
   - Skill: @go-coder
   - Files: `internal/app/app.go`
   - User decision: no

4. **Remove SQLite from storage factory and config**
   - Skill: @go-coder
   - Files: `internal/storage/factory.go`, `internal/common/config.go`
   - User decision: no

5. **Delete SQLite storage implementation**
   - Skill: @go-coder
   - Files: `internal/storage/sqlite/` (DELETE)
   - User decision: no

6. **Clean up dependencies**
   - Skill: @go-coder
   - Files: `go.mod`, `go.sum`
   - User decision: no

7. **Update test infrastructure**
   - Skill: @test-writer
   - Files: `test/common/setup.go`, `test/api/health_check_test.go`, `test/api/settings_system_test.go`, `test/api/jobs_test.go`
   - User decision: no

8. **Update documentation**
   - Skill: @none
   - Files: `README.md`, `AGENTS.md`, `docs/features/endpoint-review.md`
   - User decision: no

## Success Criteria
- All SQLite dependencies removed from codebase
- Application compiles successfully with Badger-only storage
- Existing tests pass with Badger implementation
- Queue functionality works with new Badger-backed queue
- JobManager uses storage interfaces instead of direct SQL
- Documentation reflects Badger-only architecture
