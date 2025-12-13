# Fix: WebSocket Log Debounce and Job Status UI
- Slug: websocket-log-debounce | Type: fix | Date: 2025-12-12
- Request: "Fix excessive log API calls (should be buffered to 1sec or 500 log entries or job/step status change) and fix running job UI not showing correct status or auto-expanding steps when job starts/logs available. Run test test\ui\job_definition_codebase_classify_test.go until pass."
- Prior: none

## User Intent
1. **Stop excessive log API calls**: The UI is calling the log API way too often (flooding). The trigger should be via WebSocket, buffered to:
   - 1 second interval, OR
   - 500 log entries threshold, OR
   - Job/step status change

2. **Fix job status display**: The running job UI shows incorrect status:
   - Job shows "Completed" status badge but steps show "running" spinner icons
   - Steps are not auto-expanding when logs become available (via WebSocket notification)
   - Status icons don't match actual step status

3. **Pass the test**: Run `test\ui\job_definition_codebase_classify_test.go` and refactor until it passes

## Success Criteria
- [ ] Log API calls are debounced (triggered by WebSocket only, max 1 call per second unless status changes)
- [ ] No API flooding visible in network panel (screenshot 1 shows issue)
- [ ] Job status badge reflects actual job status correctly
- [ ] Step status icons match step status (no "running" spinner on completed/pending steps)
- [ ] Steps auto-expand when logs become available via WebSocket
- [ ] Test `test\ui\job_definition_codebase_classify_test.go` passes

## Skills Assessment
| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | Yes | Yes | Backend WebSocket handler may need changes |
| frontend | .claude/skills/frontend/SKILL.md | Yes | Yes | Alpine.js UI code for job queue page |

**Active Skills:** go, frontend (note: frontend skill is duplicate of go skill - using go patterns for both)
