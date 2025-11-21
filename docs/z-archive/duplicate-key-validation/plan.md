# Plan: Add Duplicate Key Validation with UI Tests

## Steps

1. **Add service-side duplicate key validation (case-insensitive)**
   - Skill: @go-coder
   - Files: `C:\development\quaero\internal\handlers\kv_handler.go`
   - User decision: no
   - Description: Modify CreateKVHandler to check for duplicate keys (case-insensitive) before insertion and return HTTP 409 Conflict with descriptive error message

2. **Update UI to display error notifications from API**
   - Skill: @go-coder
   - Files: `C:\development\quaero\pages\static\settings-components.js`
   - User decision: no
   - Description: Update authApiKeys component's submitApiKey function to parse and display error messages from API responses (already partially implemented, ensure HTTP 409 errors show user-friendly messages)

3. **Add UI tests for duplicate key validation**
   - Skill: @test-writer
   - Files: `C:\development\quaero\test\ui\settings_apikeys_test.go`
   - User decision: no
   - Description: Add two new tests - TestSettingsAPIKeysDuplicateSameCase and TestSettingsAPIKeysDuplicateDifferentCase - that verify UI displays warnings when attempting to insert duplicate keys

4. **Run tests and verify compilation**
   - Skill: @test-writer
   - Files: All modified files
   - User decision: no
   - Description: Compile all code, run the new UI tests to verify duplicate validation works, document any issues found

## Success Criteria
- Service-side validation prevents duplicate keys (case-insensitive comparison)
- API returns HTTP 409 Conflict with clear error message when duplicate detected
- UI displays error notification from API response
- Test 1: Same-case duplicate (e.g., "TEST_KEY" twice) shows warning in UI
- Test 2: Different-case duplicate (e.g., "TEST_KEY" then "test_key") shows warning in UI
- All code compiles without errors
- Tests pass demonstrating the validation works correctly
