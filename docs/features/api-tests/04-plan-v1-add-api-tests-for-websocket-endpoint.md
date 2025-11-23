I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current State:**
- WebSocket handler (`internal/handlers/websocket.go`) manages client connections, broadcasts multiple message types (status, auth, logs, job updates), and subscribes to EventService events
- Route registered at `/ws` in `internal/server/routes.go` (line 40)
- Unit tests exist (`internal/handlers/websocket_test.go`) using `httptest.NewServer` for isolated testing
- No API integration tests exist for WebSocket endpoint (0% coverage)
- `gorilla/websocket v1.5.3` available in `go.mod` (line 13)
- Test infrastructure (`test/common/setup.go`) provides `TestEnvironment` with service lifecycle management and `HTTPTestHelper` for API calls
- Existing API tests (`auth_test.go`, `jobs_test.go`) follow patterns: setup/cleanup, subtests, helper functions, testify assertions

**Gap:**
- WebSocket endpoint lacks end-to-end integration tests verifying real-time message streaming with actual service
- No tests for WebSocket connection with running service (vs unit tests with mock server)
- No verification of WebSocket broadcasts triggered by API actions (e.g., job creation → log messages)
- No tests for concurrent client connections in integration environment
- No validation of message types/payloads received from real service

### Approach

Create comprehensive API integration tests for the WebSocket endpoint (`/ws`) in new file `test/api/websocket_test.go`. Tests will verify connection lifecycle, message streaming (logs, status updates, auth broadcasts), connection cleanup, and multiple concurrent clients. Use `gorilla/websocket` client library with patterns from existing tests (`auth_test.go`, `jobs_test.go`) and unit test examples (`internal/handlers/websocket_test.go`). Tests will trigger real actions (create jobs, capture auth) to verify WebSocket broadcasts work end-to-end with the running test service.

### Reasoning

Read relevant files (`internal/handlers/websocket.go`, `internal/server/routes.go`, `test/api/jobs_test.go`, `test/api/auth_test.go`, `test/common/setup.go`, `internal/handlers/websocket_test.go`, `go.mod`) to understand WebSocket handler implementation, existing test patterns, available libraries, and test infrastructure. Identified that unit tests exist but integration tests are missing, and gathered patterns for WebSocket client usage and API test structure.

## Mermaid Diagram

sequenceDiagram
    participant Test as Test Suite
    participant Env as TestEnvironment
    participant WS as WebSocket Client
    participant Server as Quaero Service
    participant API as HTTP API
    
    Note over Test,Server: Connection Lifecycle
    Test->>Env: SetupTestEnvironment()
    Env->>Server: Start service on port 18085
    Test->>WS: connectWebSocket()
    WS->>Server: WebSocket Upgrade /ws
    Server-->>WS: 101 Switching Protocols
    Server-->>WS: Initial status message
    WS-->>Test: Connection established
    
    Note over Test,Server: Auth Broadcast Test
    Test->>API: POST /api/auth (capture credentials)
    API->>Server: Store auth data
    Server-->>WS: Broadcast auth_captured message
    WS->>Test: waitForMessageType("auth_captured")
    Test->>Test: Assert message received
    
    Note over Test,Server: Log Streaming Test
    Test->>API: POST /api/jobs/create
    API->>Server: Create job
    Server-->>WS: Stream log messages (type="log")
    WS->>Test: readWebSocketMessage() loop
    Test->>Test: Assert log messages received
    
    Note over Test,Server: Concurrent Clients Test
    par Client 1
        Test->>WS: Connect client 1
        WS->>Server: WebSocket /ws
    and Client 2
        Test->>WS: Connect client 2
        WS->>Server: WebSocket /ws
    and Client 3
        Test->>WS: Connect client 3
        WS->>Server: WebSocket /ws
    end
    Test->>API: Trigger broadcast event
    Server-->>WS: Broadcast to all clients
    Test->>Test: Assert all clients received message
    
    Note over Test,Server: Cleanup
    Test->>WS: closeWebSocket()
    WS->>Server: Close connection
    Test->>Env: Cleanup()
    Env->>Server: Stop service

## Proposed File Changes

### test\api\websocket_test.go(NEW)

References: 

- test\common\setup.go
- test\api\auth_test.go
- test\api\jobs_test.go
- internal\handlers\websocket_test.go

Create comprehensive API integration tests for WebSocket endpoint `/ws` with the following structure:

**Package and Imports:**
- Package: `api`
- Imports: `testing`, `time`, `fmt`, `strings`, `sync`, `github.com/gorilla/websocket`, `github.com/stretchr/testify/assert`, `github.com/stretchr/testify/require`, `github.com/ternarybob/quaero/test/common`

**Helper Functions (following patterns from `test/api/auth_test.go` and `test/api/jobs_test.go`):**

1. `connectWebSocket(t *testing.T, env *common.TestEnvironment) *websocket.Conn`
   - Constructs WebSocket URL from `env.GetBaseURL()` by replacing `http://` with `ws://`
   - Uses `websocket.DefaultDialer.Dial(wsURL, nil)` to connect (pattern from `internal/handlers/websocket_test.go`)
   - Returns connection or fails test with `require.NoError`
   - Sets read deadline to prevent hanging (e.g., 5 seconds)

2. `readWebSocketMessage(t *testing.T, conn *websocket.Conn, timeout time.Duration) (map[string]interface{}, error)`
   - Sets read deadline based on timeout parameter
   - Reads JSON message with `conn.ReadJSON(&msg)`
   - Returns parsed message as `map[string]interface{}` for flexible assertion
   - Returns error if timeout or read fails (non-fatal, caller decides)

3. `waitForMessageType(t *testing.T, conn *websocket.Conn, messageType string, timeout time.Duration) map[string]interface{}`
   - Loops reading messages until finding one with `msg["type"] == messageType`
   - Uses `readWebSocketMessage` helper internally
   - Returns matching message or fails test if timeout reached
   - Useful for filtering specific message types from stream

4. `closeWebSocket(t *testing.T, conn *websocket.Conn)`
   - Gracefully closes WebSocket connection
   - Logs closure for debugging
   - Non-fatal if already closed

**Test Functions (following subtest pattern from existing tests):**

**TestWebSocketConnection** - Connection lifecycle tests:
- Setup: `common.SetupTestEnvironment(t.Name())` with defer cleanup
- Subtests:
  - `Success`: Connect to `/ws`, verify connection succeeds, verify initial status message received (type="status"), close connection cleanly
  - `MultipleConnections`: Connect 3 clients concurrently, verify all connect successfully, close all, verify no errors
  - `ReconnectAfterClose`: Connect, close, reconnect, verify second connection succeeds
  - `InvalidUpgrade`: Use regular HTTP GET (not WebSocket upgrade) to `/ws`, verify appropriate error response (400 or 426)

**TestWebSocketStatusMessages** - Status update broadcasting:
- Setup: `common.SetupTestEnvironment(t.Name())` with defer cleanup
- Subtests:
  - `InitialStatus`: Connect, wait for initial status message (type="status"), verify payload contains expected fields (e.g., `connected`, `timestamp`)
  - `StatusBroadcast`: Connect 2 clients, trigger status update (e.g., via API action), verify both clients receive status message

**TestWebSocketAuthBroadcast** - Authentication capture broadcasting:
- Setup: `common.SetupTestEnvironment(t.Name())` with defer cleanup, cleanup auth credentials before/after
- Subtests:
  - `AuthCaptured`: Connect WebSocket client, capture auth via `POST /api/auth` (reuse `createTestAuthData()` from `test/api/auth_test.go`), wait for auth broadcast message (type="auth_captured" or similar), verify message received within timeout (2 seconds)
  - `MultipleClientsReceiveAuth`: Connect 2 WebSocket clients, capture auth, verify both clients receive auth broadcast

**TestWebSocketLogStreaming** - Real-time log message streaming:
- Setup: `common.SetupTestEnvironment(t.Name())` with defer cleanup
- Subtests:
  - `JobCreationLogs`: Connect WebSocket, create test job via `POST /api/jobs/create` (reuse helper from `test/api/jobs_test.go`), wait for log messages (type="log"), verify at least one log message received related to job creation (check message content contains job ID or "job" keyword)
  - `MultipleLogMessages`: Connect WebSocket, trigger multiple actions (create 2 jobs), collect log messages for 2 seconds, verify multiple log messages received (count >= 2)
  - `LogMessageStructure`: Connect WebSocket, trigger action, wait for log message, verify payload structure (has `level`, `message`, `timestamp` fields)

**TestWebSocketConcurrentClients** - Multiple concurrent client handling:
- Setup: `common.SetupTestEnvironment(t.Name())` with defer cleanup
- Subtests:
  - `FiveClients`: Connect 5 WebSocket clients concurrently using goroutines and `sync.WaitGroup`, trigger broadcast event (e.g., create job), verify all 5 clients receive at least one message, close all connections, verify no panics or errors
  - `ClientIsolation`: Connect 2 clients, close first client, trigger broadcast, verify second client still receives messages (first client closure doesn't affect second)

**TestWebSocketConnectionCleanup** - Connection cleanup and resource management:
- Setup: `common.SetupTestEnvironment(t.Name())` with defer cleanup
- Subtests:
  - `CloseFromClient`: Connect, close from client side, verify connection closes cleanly without errors
  - `ReadAfterClose`: Connect, close, attempt to read message, verify error returned (not panic)
  - `DoubleClose`: Connect, close twice, verify second close is idempotent (no panic)

**TestWebSocketMessageTimeout** - Timeout handling:
- Setup: `common.SetupTestEnvironment(t.Name())` with defer cleanup
- Subtests:
  - `ReadTimeout`: Connect, set short read deadline (100ms), attempt to read without triggering events, verify timeout error returned (not indefinite hang)
  - `NoMessagesOK`: Connect, wait 500ms without triggering events, verify connection remains open (no forced disconnect)

**Implementation Notes:**
- Follow patterns from `test/api/auth_test.go`: use `require.NoError` for setup, `assert` for assertions, `t.Logf` for success messages
- Use `defer conn.Close()` for WebSocket connections to ensure cleanup
- Set read deadlines to prevent tests hanging indefinitely
- Use `time.Sleep` sparingly (only when waiting for async broadcasts), prefer polling with timeout
- Message type constants should match those in `internal/handlers/websocket.go` (e.g., "log", "status", "auth_captured")
- For concurrent tests, use `sync.WaitGroup` and goroutines (pattern from `internal/handlers/websocket_test.go`)
- Keep test functions under 80 lines by extracting helpers
- Each test should be self-contained with setup/cleanup
- Use `t.Run()` for subtests to organize related test cases
- Log test progress with `t.Logf("✓ Test completed")` for consistency with existing tests

**Limitations/Future Work (document in comments):**
- WebSocket message throttling not tested (requires high-frequency event generation)
- Event whitelist filtering not tested (requires EventService integration)
- Specific event types (crawl_progress, job_spawn, etc.) not exhaustively tested (focus on core message types: status, auth, log)
- Load testing with 100+ concurrent clients deferred to performance test suite
- WebSocket ping/pong keepalive not tested (implementation detail)

**Error Handling:**
- All WebSocket dial errors should fail test immediately (`require.NoError`)
- Read timeout errors in message waiting should fail test with descriptive message
- Connection close errors should be logged but not fail test (graceful degradation)
- Unexpected message types should be logged for debugging but not fail test (allow extra messages)