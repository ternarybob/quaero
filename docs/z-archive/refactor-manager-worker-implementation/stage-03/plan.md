# Plan: Migrate Manager Files from executor/ to manager/ (ARCH-004)

## Steps

1. **Create crawler_manager.go in internal/jobs/manager/**
   - Skill: @code-architect
   - Files: internal/jobs/manager/crawler_manager.go (NEW), internal/jobs/executor/crawler_step_executor.go (READ)
   - User decision: no
   - Copy from crawler_step_executor.go, rename struct CrawlerStepExecutor → CrawlerManager, rename constructor, update package to manager, update receiver variable e → m

2. **Create database_maintenance_manager.go in internal/jobs/manager/**
   - Skill: @code-architect
   - Files: internal/jobs/manager/database_maintenance_manager.go (NEW), internal/jobs/executor/database_maintenance_step_executor.go (READ)
   - User decision: no
   - Copy from database_maintenance_step_executor.go, rename struct DatabaseMaintenanceStepExecutor → DatabaseMaintenanceManager, rename constructor, update package to manager, update receiver variable e → m

3. **Create agent_manager.go in internal/jobs/manager/**
   - Skill: @code-architect
   - Files: internal/jobs/manager/agent_manager.go (NEW), internal/jobs/executor/agent_step_executor.go (READ)
   - User decision: no
   - Copy from agent_step_executor.go, rename struct AgentStepExecutor → AgentManager, rename constructor, update package to manager, update receiver variable e → m

4. **Update internal/app/app.go to use new manager package**
   - Skill: @go-coder
   - Files: internal/app/app.go
   - User decision: no
   - Add manager import, update 3 manager registrations to use new constructors (NewCrawlerManager, NewDatabaseMaintenanceManager, NewAgentManager), keep executor import for other managers

5. **Add deprecation notices to old executor files**
   - Skill: @go-coder
   - Files: internal/jobs/executor/crawler_step_executor.go, internal/jobs/executor/database_maintenance_step_executor.go, internal/jobs/executor/agent_step_executor.go
   - User decision: no
   - Add deprecation comments indicating migration to manager package and removal timeline (ARCH-008)

6. **Update documentation files**
   - Skill: @none
   - Files: AGENTS.md, docs/architecture/MANAGER_WORKER_ARCHITECTURE.md
   - User decision: no
   - Update migration status to show ARCH-004 complete, add checkmarks for migrated files, document file mappings

7. **Compile and verify implementation**
   - Skill: @go-coder
   - Files: All modified files
   - User decision: no
   - Run go build to verify compilation, check that all 3 managers compile correctly in new location

8. **Run tests to validate migration**
   - Skill: @test-writer
   - Files: Test suites in test/api/
   - User decision: no
   - Run relevant test suites to ensure managers work correctly from new location, verify no regressions

## Success Criteria

- 3 new manager files created in internal/jobs/manager/ (crawler_manager.go, database_maintenance_manager.go, agent_manager.go)
- All struct names use "Manager" suffix instead of "StepExecutor"
- All constructor names use "Manager" suffix
- internal/app/app.go successfully imports and uses new managers
- Deprecation notices added to old files
- Documentation updated to reflect ARCH-004 completion
- Application compiles cleanly
- All tests pass
- Old files remain in executor/ for backward compatibility (removed in ARCH-008)
