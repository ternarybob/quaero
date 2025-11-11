# Plan: Agent Implementation v1 - Job Definitions and Integration Tests

## Steps

1. **Create keyword extractor agent job definition TOML**
   - Skill: @none
   - Files: `deployments/local/job-definitions/keyword-extractor-agent.toml` (NEW)
   - User decision: no

2. **Add test helpers to keyword extractor for unit testing**
   - Skill: @go-coder
   - Files: `internal/services/agents/keyword_extractor.go` (MODIFY)
   - User decision: no

3. **Create API integration tests for agent job execution**
   - Skill: @test-writer
   - Files: `test/api/agent_job_test.go` (NEW)
   - User decision: no

4. **Create unit tests for keyword extractor**
   - Skill: @test-writer
   - Files: `test/unit/keyword_extractor_test.go` (NEW)
   - User decision: no

5. **Run tests and verify implementation**
   - Skill: @test-writer
   - Files: All test files
   - User decision: no

## Success Criteria
- Job definition TOML created and follows existing patterns (news-crawler.toml, nearby-restaurants-places.toml)
- API tests verify end-to-end agent execution via HTTP endpoints
- Unit tests verify input validation and response parsing logic
- All tests compile and run (pass/fail documented in step files)
- Test infrastructure uses SetupTestEnvironment() and HTTPTestHelper patterns
- Documentation includes agent chaining examples and usage notes
