# Plan: Add Test Configuration and Comprehensive Settings Page Tests

## Steps

1. **Create test/config/auth directory with test auth files**
   - Skill: @test-writer
   - Files: test/config/auth/test-cookies.json, test/config/auth/test-apikey.json, test/common/setup.go
   - User decision: no

2. **Add API test to verify auth config loading**
   - Skill: @test-writer
   - Files: test/api/auth_config_test.go
   - User decision: no

3. **Create comprehensive UI test for settings page**
   - Skill: @test-writer
   - Files: test/ui/settings_test.go
   - User decision: no

4. **Verify all tests compile and pass**
   - Skill: @test-writer
   - Files: test/api/auth_config_test.go, test/ui/settings_test.go
   - User decision: no

## Success Criteria
- test/config/auth directory exists with test auth files
- test/common/setup.go loads auth config like it loads job-definitions
- API test verifies auth config loading from test/config/auth
- UI test covers: page loads, accordion expands, accordion state persists on refresh
- All console errors checked in UI tests
- All tests compile and pass
- Tests follow existing patterns (homepage_test.go structure)
