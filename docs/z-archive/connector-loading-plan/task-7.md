# Task 7: Create API Tests for Connector Loading

- Group: 7 | Mode: concurrent | Model: sonnet
- Skill: @test-automator | Critical: no | Depends: 5
- Sandbox: /tmp/3agents/task-7/ | Source: . | Output: docs/fixes/

## Files
- `test/api/connector_loading_test.go` - NEW file

## Requirements
Create API tests for connector loading functionality:

1. **Test_ConnectorLoading_FromTOMLFile**
   - Create temp directory with connectors.toml
   - Define github and gitlab connectors
   - Verify connectors are loaded via API GET /api/connectors
   - Verify connector details match TOML definition

2. **Test_ConnectorLoading_EmptyDirectory**
   - Create empty connectors directory
   - Verify startup succeeds
   - Verify GET /api/connectors returns empty list

3. **Test_ConnectorLoading_InvalidTOML**
   - Create connectors.toml with invalid TOML syntax
   - Verify startup succeeds (graceful degradation)
   - Verify appropriate warning in logs

4. **Test_ConnectorLoading_MissingToken**
   - Create connector with empty token
   - Verify connector is skipped
   - Verify warning logged

Follow the existing test patterns in `test/api/` directory.

## Acceptance
- [ ] Test file created in test/api/
- [ ] Tests cover happy path (valid connectors loaded)
- [ ] Tests cover empty directory
- [ ] Tests cover invalid TOML
- [ ] Tests cover missing required fields
- [ ] All tests pass
