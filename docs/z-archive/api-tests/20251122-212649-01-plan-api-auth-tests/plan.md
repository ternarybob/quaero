# Plan: Add API Tests for Authentication Endpoints

## Steps

1. **Create helper functions for auth test setup and cleanup**
   - Skill: @test-writer
   - Files: `test/api/auth_test.go` (new)
   - User decision: no
   - Description: Implement `createTestAuthData()`, `captureTestAuth()`, `deleteTestAuth()`, and `cleanupAllAuth()` helper functions following patterns from `test/api/settings_system_test.go`

2. **Implement POST /api/auth capture tests**
   - Skill: @test-writer
   - Files: `test/api/auth_test.go`
   - User decision: no
   - Description: Create `TestAuthCapture` with subtests for success, invalid JSON, missing fields, and empty cookies scenarios

3. **Implement GET /api/auth/status tests**
   - Skill: @test-writer
   - Files: `test/api/auth_test.go`
   - User decision: no
   - Description: Create `TestAuthStatus` with subtests for authenticated and not authenticated states

4. **Implement GET /api/auth/list tests**
   - Skill: @test-writer
   - Files: `test/api/auth_test.go`
   - User decision: no
   - Description: Create `TestAuthList` with subtests for empty list, single credential, and multiple credentials, including sanitization verification

5. **Implement GET /api/auth/{id} tests**
   - Skill: @test-writer
   - Files: `test/api/auth_test.go`
   - User decision: no
   - Description: Create `TestAuthGet` with subtests for success, not found, and empty ID scenarios, including sanitization verification

6. **Implement DELETE /api/auth/{id} tests**
   - Skill: @test-writer
   - Files: `test/api/auth_test.go`
   - User decision: no
   - Description: Create `TestAuthDelete` with subtests for success, not found, and empty ID scenarios

7. **Implement comprehensive sanitization tests**
   - Skill: @test-writer
   - Files: `test/api/auth_test.go`
   - User decision: no
   - Description: Create `TestAuthSanitization` with subtests verifying cookies and tokens are never exposed in list or get responses

8. **Run full test suite and verify all tests pass**
   - Skill: @test-writer
   - Files: `test/api/auth_test.go`
   - User decision: no
   - Description: Execute `go test -v ./test/api/auth_test.go` and verify all test cases pass with proper error handling

## Success Criteria

- All 6 test functions implemented with comprehensive subtests
- Helper functions follow existing patterns from `test/api/settings_system_test.go`
- Critical sanitization verification ensures cookies/tokens are never exposed
- All positive and negative test cases covered for each endpoint
- Tests compile cleanly and pass on first run
- Code follows Go testing conventions and project patterns
- Test coverage includes error cases (invalid JSON, missing fields, not found)
- Cleanup functions ensure test isolation and clean state
