# Plan: Value Store {key-name} Replacement Feature

## Steps

1. **Create replacement engine with utility functions**
   - Skill: @code-architect
   - Files: `internal/common/replacement.go`
   - User decision: no

2. **Create comprehensive unit tests for replacement**
   - Skill: @test-writer
   - Files: `internal/common/replacement_test.go`
   - User decision: no

3. **Integrate replacement into job definition loading**
   - Skill: @go-coder
   - Files: `internal/storage/sqlite/load_job_definitions.go`
   - User decision: no

4. **Integrate replacement into config loading**
   - Skill: @go-coder
   - Files: `internal/common/config.go`
   - User decision: no

5. **Update main.go and app.go for two-phase initialization**
   - Skill: @go-coder
   - Files: `cmd/quaero/main.go`, `internal/app/app.go`
   - User decision: no

6. **Run integration tests and verify replacement**
   - Skill: @test-writer
   - Files: Test the complete flow end-to-end
   - User decision: no

## Success Criteria

- Replacement engine correctly handles `{key-name}` syntax
- Missing keys log warnings but don't fail
- Recursive replacement works for nested maps and structs
- Job definitions replace references before validation
- Config replacement happens after storage initialization
- All unit tests pass (15+ test cases)
- Application compiles cleanly with no breaking changes
- Two-phase initialization: config loads → storage init → replacement
- Graceful degradation when KV storage unavailable
