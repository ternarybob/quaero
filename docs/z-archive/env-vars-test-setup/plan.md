# Plan: Test Environment Variable Loading

## Overview
Implement functionality to load environment variables from `.env.test` file into an in-memory key/value store during test setup, and update the settings API keys test to verify that the GOOGLE_API_KEY is properly inserted and displayed.

## Steps

1. **Update setup.go to load .env.test file into memory**
   - Skill: @go-coder
   - Files: `test/common/setup.go`, `test/config/.env.test`
   - User decision: no
   - Create a function to parse .env.test file and store key/value pairs in memory
   - Make the environment variables accessible to tests via the TestEnvironment struct
   - Load the .env file during SetupTestEnvironment initialization

2. **Update settings_apikeys_test.go to insert GOOGLE_API_KEY**
   - Skill: @test-writer
   - Files: `test/ui/settings_apikeys_test.go`, `test/common/setup.go`
   - User decision: no
   - Access GOOGLE_API_KEY from the loaded environment variables
   - Insert the key into the settings page via the API or UI automation
   - Verify the value is displayed correctly in the UI (should match the value from .env.test)

3. **Run and verify the tests**
   - Skill: @test-writer
   - Files: `test/ui/settings_apikeys_test.go`
   - User decision: no
   - Run the updated test to ensure it passes
   - Verify compilation is clean
   - Document any issues or warnings

## Success Criteria
- `setup.go` successfully loads `.env.test` file and stores key-value pairs in memory
- `settings_apikeys_test.go` can access GOOGLE_API_KEY from loaded env vars
- Test verifies the value is inserted and displayed correctly in the UI
- All code compiles cleanly
- Tests pass or issues are clearly documented
- The displayed value matches the value from `.env.test` file

## Technical Notes
- The .env.test file format: `KEY="value"` (standard .env format)
- Need to parse KEY=VALUE pairs, handling quotes
- Store in a map[string]string accessible via TestEnvironment
- The test should use the actual value from .env.test to verify display
