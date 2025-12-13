# Plan: Create/Update Job Test for API Key Injection

## Steps

1. **Analyze existing job and config tests**
   - Skill: @none
   - Files: test/api/agent_job_test.go, test/api/config_dynamic_injection_test.go, test/api/job_definition_execution_test.go
   - User decision: no

2. **Create comprehensive job API key test**
   - Skill: @test-writer
   - Files: test/api/job_api_key_injection_test.go (new)
   - User decision: no

3. **Run tests to validate implementation**
   - Skill: @go-coder
   - Files: test/api/job_api_key_injection_test.go
   - User decision: no

## Success Criteria
- Test verifies API key is properly stored in KV storage
- Test verifies job definitions reference API keys by name
- Test verifies runtime validation detects missing API keys
- Test verifies API key injection into job config works correctly
- All tests compile and pass
- Test follows existing patterns in test/api/*_test.go files
