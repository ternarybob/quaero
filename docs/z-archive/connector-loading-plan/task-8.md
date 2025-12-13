# Task 8: Create UI Tests for Connector Display

- Group: 8 | Mode: concurrent | Model: sonnet
- Skill: @test-automator | Critical: no | Depends: 5
- Sandbox: /tmp/3agents/task-8/ | Source: . | Output: docs/fixes/

## Files
- `test/ui/connector_ui_test.go` - NEW file

## Requirements
Create UI tests for connector display in Settings page:

1. **Test_ConnectorUI_DisplaysLoadedConnectors**
   - Load connectors via API or file
   - Navigate to Settings > Connectors
   - Verify connectors are listed
   - Verify connector type is displayed correctly

2. **Test_ConnectorUI_NoConnectorsMessage**
   - Start with no connectors
   - Navigate to Settings > Connectors
   - Verify "No Connectors" message displayed
   - Verify "Add a connector to integrate with external services" text

3. **Test_ConnectorUI_ConnectorDetails**
   - Create a connector
   - Click on connector in list
   - Verify connector details shown (name, type)
   - Verify token is masked/hidden for security

Follow the existing UI test patterns in `test/ui/` directory using the test framework.

## Acceptance
- [ ] Test file created in test/ui/
- [ ] Tests cover connector list display
- [ ] Tests cover empty state message
- [ ] Tests cover connector details view
- [ ] All tests pass
