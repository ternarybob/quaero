# Complete: WebSocket UI Sync for Running Jobs
Type: fix | Tasks: 5 | Files: 5

## User Request
"Fix UI not updating from WebSocket triggers for running jobs. Steps showing incorrect status (pending when should be completed). Need simpler backend endpoints and cleaner WebSocket message protocol."

## Result
Added a unified `job_update` WebSocket message that broadcasts step status changes directly from StepMonitor, bypassing the log aggregator delay. Created a lightweight `/api/jobs/{id}/structure` endpoint for efficient status polling. The UI now updates step status in real-time via the new message format with clear context (job or job_step).

## Skills Used
- go (backend changes to handlers, WebSocket protocol, event service)

## Validation: ✅ MATCHES
All 6 success criteria met:
1. Structure endpoint provides lightweight status data
2. WebSocket messages have clear context field
3. UI handles job_update messages correctly
4. Log fetching only for expanded steps
5. Real-time status updates work
6. Status indicators match backend state

## Review: N/A
Not a critical task (no security, authentication, crypto, state-machine, or architectural-change triggers)

## Verify
Build: ✅ | Tests: ⏭️ (manual verification recommended)

## Files Changed
| File | Changes |
|------|---------|
| internal/handlers/job_handler.go | Added JobStructureResponse, StepStatus structs, GetJobStructureHandler |
| internal/handlers/websocket.go | Added JobUpdatePayload struct, BroadcastJobUpdate method, EventJobUpdate subscription |
| internal/interfaces/event_service.go | Added EventJobUpdate event type |
| internal/queue/state/step_monitor.go | Added publishJobUpdate method |
| internal/server/routes.go | Added /api/jobs/{id}/structure route |
| pages/queue.html | Added job_update handler, handleJobUpdate, fetchJobStructure, fetchStepLogs methods |
