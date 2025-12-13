# Fix: WebSocket UI Sync for Running Jobs
- Slug: websocket-ui-sync | Type: fix | Date: 2025-12-11
- Request: "Fix UI not updating from WebSocket triggers for running jobs. Steps showing incorrect status (pending when should be completed). Need simpler backend endpoints and cleaner WebSocket message protocol."
- Prior: docs/feature/20251211-job-logging-architecture/ (related work on job logging)

## User Intent
The UI is not correctly updating when a job is running. The screenshot shows:
1. Parent job marked "Completed" but steps (import_files, rule_classify_files) showing yellow/pending status
2. code_map step shows "1 pending, 0 running, 0 completed" despite logs indicating activity
3. WebSocket triggers are not properly updating the UI state

User wants:
1. **Simplified backend endpoint** for job structure: overall status + steps with their status and log counts
2. **WebSocket messages with clear context**: `context=job` or `context=job_step` with job_id, step_id, status, log_refresh flag
3. **UI driven by WebSocket triggers**: When log_refresh=true, UI fetches logs for that step
4. **Collapsed steps don't retain logs**: Only expanded steps fetch/show logs

## Success Criteria
- [ ] Backend provides `/api/jobs/{id}/structure` endpoint with: overall job status, steps array with status and log counts
- [ ] WebSocket messages include context field (job or job_step), job_id, step_id, status, log_refresh flag
- [ ] UI correctly updates job and step status from WebSocket messages
- [ ] UI only fetches logs for expanded steps when log_refresh=true
- [ ] Running jobs show correct status in real-time without page refresh
- [ ] Step status indicators (pending/running/completed) match actual backend state

## Skills Assessment
| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | ✅ | ✅ | Backend changes to handlers, WebSocket protocol |
| frontend | .claude/skills/frontend/SKILL.md | ✅ | ✅ | UI changes to Alpine.js components (note: skill file is copy of Go, but patterns apply) |

**Active Skills:** go
