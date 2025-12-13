# Step 3: Add step progress events
- Task: task-3.md | Group: 3 | Model: sonnet

## Actions
1. Added step start event emission in ExecuteJobDefinition loop
2. Added step complete event emission after successful step execution
3. Added WebSocket subscription for EventJobProgress
4. Broadcast `job_step_progress` to all connected clients

## Files
- `internal/queue/manager.go` - lines 884-906, 973-995: event emission
- `internal/handlers/websocket.go` - lines 1070-1129: websocket handler

## Decisions
- Async publish: Use goroutines to avoid blocking step execution
- Reuse EventJobProgress: Existing event type matches payload structure

## Verify
Compile: ✅ | Tests: ⚙️

## Status: ✅ COMPLETE
