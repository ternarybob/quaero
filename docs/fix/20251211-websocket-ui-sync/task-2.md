# Task 2: Add job_update WebSocket message for status changes
Depends: 1 | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent
Provides a unified WebSocket message format with clear context (job or job_step) so UI knows exactly what to update.

## Skill Patterns to Apply
- Single responsibility: One helper function for broadcasting job updates
- Clear message structure: Type + payload with well-defined fields
- Reuse existing broadcast pattern from WebSocketHandler

## Do
1. Add `JobUpdatePayload` struct to websocket.go
2. Create `BroadcastJobUpdate(jobID, context, stepName, status, refreshLogs bool)` helper method
3. The method should:
   - Build message with type "job_update"
   - Include context: "job" or "job_step"
   - Include job_id, step_name (if job_step), status, refresh_logs flag
   - Broadcast to all connected clients

## Accept
- [ ] `BroadcastJobUpdate` method exists and is callable
- [ ] Message format matches specification: `{type: "job_update", payload: {...}}`
- [ ] Context field is "job" or "job_step"
- [ ] refresh_logs flag included in payload
- [ ] Broadcast uses existing client list and mutex pattern
