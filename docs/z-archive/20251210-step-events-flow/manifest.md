# Fix: Step Events Flow + Timestamp Ordering
- Slug: step-events-flow | Type: fix | Date: 2025-12-10
- Request: "UI not flowing/updating correctly. Step should load events via API either triggered by websocket (if running) or last 100 (if complete). Events not able to be ordered - need millisecond timestamps."
- Prior: docs/fix/20251210-step-events-crash/

## User Intent
1. Step events should load from API when triggered by WebSocket (running steps with >1sec duration)
2. Step events should load last 100 from API when step completes/fails/cancels
3. Event timestamps need millisecond precision for proper ordering (currently second-only)
4. On page load with completed job, all steps should show their events (not "0 events")

## Success Criteria
- [ ] Completed steps load last 100 events from API on page load
- [ ] Running steps with >1sec duration get WebSocket-triggered refreshes
- [ ] Event timestamps stored/displayed with millisecond precision
- [ ] Events ordered correctly by millisecond timestamp
- [ ] Build succeeds

## Skills Assessment
| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | Y | Y | Event timestamp handling, API handlers |
| frontend | .claude/skills/frontend/SKILL.md | Y | Y | Alpine.js queue.html event loading |

**Active Skills:** go, frontend
