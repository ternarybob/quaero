# Feature: Event-Driven Job UI with Unified Process Model
- Slug: event-driven-job-ui | Type: feature | Date: 2025-12-01
- Request: "ALL jobs need to execute using the same process steps. This will enable both monitoring and UI output. Simplify to 2-level hierarchy (parent + jobs). UI becomes event/log display with websocket real-time updates."
- Prior: docs/feature/20251130-dual-steps-ui/ (step expansion work)

## Scope
1. Unify all job types to single process model with parent event/logging
2. All jobs publish events and logs to parent
3. Child jobs can create more child jobs (under same parent, managed by queue)
4. UI changes to log/event display interface
5. WebSocket-driven updates during job execution, poll after completion
