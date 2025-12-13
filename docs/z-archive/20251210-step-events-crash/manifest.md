# Fix: Step Events Loading Issues + Crash Investigation
- Slug: step-events-crash | Type: fix | Date: 2025-12-10
- Request: "UI still loading all events/logs - The screenshot is when the job page is loaded and steps/workers are running. Appears the flow of step completion is not working as required. The service also crashes (random) without logs."
- Prior: docs/fix/20251210-ui-event-buffering/

## User Intent
1. Fix completed steps showing "No events yet for this step" - events should load when step completes
2. Fix Step 3 loading all events instead of last 100 only - events are scrolling rapidly
3. Investigate random service crashes

## Success Criteria
- [ ] Completed steps load their events (last 100)
- [ ] Step events only load on COMPLETE, not during execution
- [ ] No individual job_log messages for step events (use trigger approach)
- [ ] Crash investigation completed
- [ ] Build succeeds

## Skills Assessment
| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | Y | Y | WebSocket handler modifications |
| frontend | .claude/skills/frontend/SKILL.md | Y | Y | Alpine.js queue.html |

**Active Skills:** go, frontend
