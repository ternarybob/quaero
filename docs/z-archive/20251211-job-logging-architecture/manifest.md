# Feature: Job Logging Architecture Refactor
- Slug: job-logging-architecture | Type: feature | Date: 2025-12-11
- Request: "Review and rearchitect the job logging/events/monitor and UI display"
- Prior: none

## User Intent
Simplify and clean up the job logging, monitoring, and UI display architecture by:
1. Workers log simple messages with key/value context (job_id, step_id, worker_id)
2. Job monitor maintains job status with steps and operational metadata
3. API assembles job status + logs into structured JSON for UI
4. UI renders job status/logs from JSON, triggered by WebSocket events
5. Clear separation of concerns between Worker, Logging, Monitor, WebSocket, and UI

## Fundamental Issues Identified
1. Backend is drifting into specific code for specific issues - need VERY simple approach using queue manager, badgerhold, and arbor
2. UI is using code to trigger expansion/status (see screenshot) - should be driven from backend JSON structure
3. Need clear separation: Worker (work + log), Logging (store indexed entries), Monitor (job status/counts), WebSocket (change notifications), UI (render)

## Success Criteria
- [ ] Workers log with simple messages and context keys (job_id, step_id, worker_id)
- [ ] Monitor service maintains job status independently from logging
- [ ] API serves structured JSON that UI can render directly (tree structure from backend)
- [ ] UI expansion/collapse logic driven by JSON structure, not client-side code
- [ ] WebSocket sends change notifications, UI re-fetches structured data
- [ ] Clear separation of concerns between components

## Skills Assessment
| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | ✅ | ✅ | Backend refactoring of handlers, services, storage |
| frontend | .claude/skills/frontend/SKILL.md | ✅ | ✅ | UI simplification for queue.html |

**Active Skills:** go, frontend
