# Plan: Add API Tests for WebSocket Endpoint

## Steps

1. **Create test file with WebSocket helper functions**
   - Skill: @test-writer
   - Files: `test/api/websocket_test.go` (new)
   - User decision: no
   - Description: Create helper functions for WebSocket connection management (connectWebSocket, readWebSocketMessage, waitForMessageType, closeWebSocket) following patterns from auth_test.go and jobs_test.go

2. **Implement connection lifecycle tests**
   - Skill: @test-writer
   - Files: `test/api/websocket_test.go`
   - User decision: no
   - Description: Create TestWebSocketConnection with 4 subtests (Success, MultipleConnections, ReconnectAfterClose, InvalidUpgrade) to verify connection establishment, concurrent connections, and error handling

3. **Implement status message tests**
   - Skill: @test-writer
   - Files: `test/api/websocket_test.go`
   - User decision: no
   - Description: Create TestWebSocketStatusMessages with 2 subtests (InitialStatus, StatusBroadcast) to verify status message broadcasting

4. **Implement auth broadcast tests**
   - Skill: @test-writer
   - Files: `test/api/websocket_test.go`
   - User decision: no
   - Description: Create TestWebSocketAuthBroadcast with 2 subtests (AuthCaptured, MultipleClientsReceiveAuth) to verify auth capture broadcasting

5. **Implement log streaming tests**
   - Skill: @test-writer
   - Files: `test/api/websocket_test.go`
   - User decision: no
   - Description: Create TestWebSocketLogStreaming with 3 subtests (JobCreationLogs, MultipleLogMessages, LogMessageStructure) to verify real-time log message streaming

6. **Implement concurrent client tests**
   - Skill: @test-writer
   - Files: `test/api/websocket_test.go`
   - User decision: no
   - Description: Create TestWebSocketConcurrentClients with 2 subtests (FiveClients, ClientIsolation) to verify multiple concurrent client handling

7. **Implement connection cleanup and timeout tests**
   - Skill: @test-writer
   - Files: `test/api/websocket_test.go`
   - User decision: no
   - Description: Create TestWebSocketConnectionCleanup (3 subtests) and TestWebSocketMessageTimeout (2 subtests) to verify cleanup and timeout handling

8. **Run full test suite and verify**
   - Skill: @test-writer
   - Files: `test/api/websocket_test.go`
   - User decision: no
   - Description: Execute `go test -v -run TestWebSocket` to verify compilation and test execution, document results

## Success Criteria

- All 6 test functions implemented with comprehensive subtests (~18 total subtests)
- Helper functions follow patterns from auth_test.go and jobs_test.go
- Tests compile cleanly without errors
- WebSocket connections properly managed with cleanup
- Tests verify connection lifecycle, message broadcasting, concurrent clients, and cleanup
- Code follows Go testing conventions and project patterns
- Message types match those in internal/handlers/websocket.go
- Tests use gorilla/websocket client library
- Limitations documented in comments (throttling, event whitelist, load testing)
