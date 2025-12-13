# Step 2: Add job_update WebSocket message
Model: opus | Skill: go | Status: ✅

## Done
- Added `JobUpdatePayload` struct to websocket.go
- Created `BroadcastJobUpdate(jobID, context, stepName, status, refreshLogs)` method
- Method broadcasts to all connected WebSocket clients

## Files Changed
- `internal/handlers/websocket.go` - Added struct (lines 281-289) and method (lines 855-899)

## Skill Compliance
- [x] Single responsibility: One helper function for broadcasting job updates
- [x] Clear message structure: Type + payload with well-defined fields
- [x] Reuses existing broadcast pattern from WebSocketHandler

## Build Check
Build: ✅ | Tests: ⏭️
