# Task 3: Add step progress events
- Group: 3 | Mode: concurrent | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 1
- Sandbox: /tmp/3agents/task-3/ | Source: ./ | Output: docs/feature/20251130-dual-steps-ui/

## Files
- `internal/queue/manager.go` - emit step progress events
- `internal/handlers/websocket.go` - add websocket handler

## Requirements
Add WebSocket events for step progress in multi-step job definitions:
1. Emit `job_progress` event when step starts (status: "running")
2. Emit `job_progress` event when step completes (status: "completed")
3. Add WebSocket handler to broadcast `job_step_progress` to clients

## Acceptance
- [ ] Events emitted on step start/complete
- [ ] WebSocket handler broadcasts to clients
- [ ] Compiles
