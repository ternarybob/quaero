# Plan: Create API Tests for Settings and System Endpoints

## Overview
Create comprehensive API tests for Settings and System endpoints in `test/api/settings_system_test.go` following the `health_check_test.go` template pattern. Tests will cover KV Store, Connectors, Config, Status, and Logs endpoints with proper validation of CRUD operations, case-insensitivity, error handling, and response structures.

## Steps

1. **Create settings_system_test.go with KV Store tests**
   - Skill: @test-writer
   - Files: `test/api/settings_system_test.go` (NEW)
   - User decision: no
   - Implement: TestKVStore_CRUD, TestKVStore_CaseInsensitive, TestKVStore_Upsert, TestKVStore_DuplicateValidation, TestKVStore_ValueMasking, TestKVStore_ValidationErrors
   - Follow pattern from `test/api/health_check_test.go` using `SetupTestEnvironment()` and `HTTPTestHelper`
   - Test all 6 KV endpoints with proper assertions for case-insensitive keys, value masking, upsert behavior, and error cases

2. **Add Connector API tests**
   - Skill: @test-writer
   - Files: `test/api/settings_system_test.go` (EDIT)
   - User decision: no
   - Implement: TestConnectors_CRUD, TestConnectors_Validation, TestConnectors_GitHubConnectionTest
   - Test connector creation, listing, updating, deletion with GitHub connector validation
   - Handle connection test scenarios (may need mock or skip if no valid token)

3. **Add System endpoint tests**
   - Skill: @test-writer
   - Files: `test/api/settings_system_test.go` (EDIT)
   - User decision: no
   - Implement: TestConfig_Get, TestStatus_Get, TestVersion_Get, TestHealth_Get
   - Verify response structures for config (with injected keys), status, version, and health endpoints
   - Validate config contains expected sections and version/build info is present

4. **Add Logs endpoint tests**
   - Skill: @test-writer
   - Files: `test/api/settings_system_test.go` (EDIT)
   - User decision: no
   - Implement: TestLogsRecent_Get, TestSystemLogs_ListFiles, TestSystemLogs_GetContent
   - Test recent logs retrieval, log file listing, and log content filtering (by level and limit)
   - Handle empty log scenarios gracefully

5. **Add helper functions and run full test suite**
   - Skill: @test-writer
   - Files: `test/api/settings_system_test.go` (EDIT)
   - User decision: no
   - Implement helper functions: createKVPair, deleteKVPair, createConnector, deleteConnector
   - Run complete test suite: `cd test/api && go test -v -run Settings`
   - Verify all tests compile and execute without errors

## Success Criteria
- File `test/api/settings_system_test.go` created with all 16 test functions
- All tests follow the `health_check_test.go` pattern using `SetupTestEnvironment()` and `HTTPTestHelper`
- Tests cover all KV Store endpoints (CRUD, case-insensitivity, masking, validation)
- Tests cover all Connector endpoints (CRUD, validation, GitHub connection test)
- Tests cover System endpoints (config, status, version, health)
- Tests cover Logs endpoints (recent, files, content with filtering)
- Helper functions created for common operations (createKVPair, deleteKVPair, createConnector, deleteConnector)
- All tests compile successfully with `go build`
- All tests execute with `go test -v -run Settings` (may have some skipped tests for GitHub if no token)
- Tests verify exact response structures and status codes per handler implementations
- Error cases properly tested (validation errors, not found, conflicts)
