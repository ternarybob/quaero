# Plan: Refactor job_error_display_simple_test.go

## Objective
Refactor `test/ui/job_error_display_simple_test.go` to specifically test two real job scenarios:
1. Run "places-nearby-restaurants" job and verify documents are created
2. Run "keyword-extractor-agent" job and verify success

Note: The test will fail as expected since the Gemini API key is not currently configured, but the test structure should properly validate job execution and error handling.

## Steps

1. **Analyze existing test structure and API test patterns**
   - Skill: @none
   - Files: test/ui/job_error_display_simple_test.go, test/api/job_integration_test.go, test/common/*.go
   - User decision: no
   - Understand current test helpers, job execution patterns, and document verification approaches

2. **Design new test structure for two-job scenario**
   - Skill: @code-architect
   - Files: test/ui/job_error_display_simple_test.go
   - User decision: no
   - Plan test phases, helper functions needed, and UI verification strategy

3. **Implement Phase 1: Places job execution and document verification**
   - Skill: @go-coder
   - Files: test/ui/job_error_display_simple_test.go
   - User decision: no
   - Create job definition, execute job, poll for completion, verify documents created in UI

4. **Implement Phase 2: Keyword extraction job execution and success verification**
   - Skill: @go-coder
   - Files: test/ui/job_error_display_simple_test.go
   - User decision: no
   - Execute keyword-extractor-agent job, handle expected failure due to missing API key, verify error display in UI

5. **Add comprehensive logging and screenshots**
   - Skill: @test-writer
   - Files: test/ui/job_error_display_simple_test.go
   - User decision: no
   - Enhance test observability with detailed logging and strategic screenshots for both job scenarios

6. **Compile and validate test structure**
   - Skill: @go-coder
   - Files: test/ui/job_error_refactor/*.go
   - User decision: no
   - Ensure code compiles, follows Go test patterns, and matches existing test/ui conventions

## Success Criteria
- Test executes both "places-nearby-restaurants" and "keyword-extractor-agent" jobs
- Test verifies documents are created by places job in the UI
- Test properly handles expected failure of keyword job due to missing API key
- Test uses ChromeDP to verify error display in UI
- Test follows existing patterns from test/api/job_integration_test.go for job execution
- Test follows existing patterns from test/ui/*.go for UI verification
- Code compiles without errors
- Test is runnable (even if it fails due to API key as expected)
