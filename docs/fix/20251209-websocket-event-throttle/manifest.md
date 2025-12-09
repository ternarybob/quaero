# Fix: WebSocket Event Throttling for Step Events

- Slug: websocket-event-throttle | Type: fix | Date: 2025-12-09
- Request: "WebSocket event pushing for step events slows processing when 100+ jobs in queue. Refactor to use trigger-based polling instead of direct event push."
- Prior: none

## User Intent
1. **Remove direct event push** - Stop pushing individual step events through WebSocket, which blocks processing
2. **Implement trigger-based updates** - WebSocket sends a "refresh trigger" instead of full events
3. **Add event aggregator** - Collect events in memory and trigger refresh when: 100 events accumulated OR 1 second elapsed (configurable)
4. **Create/enhance events API** - Support paged, time-based, or "last N" queries for step events
5. **Refactor UI** - Step panels fetch events from API when triggered, not from WebSocket payload

## Success Criteria
- [ ] Step events no longer pushed directly through WebSocket
- [ ] Event aggregator accumulates events and triggers refresh at threshold (100 events or 1 second)
- [ ] Throttle settings configurable in global TOML
- [ ] Events API supports pagination/last-N queries per step
- [ ] UI step panels fetch events via API on WebSocket trigger
- [ ] Processing speed improves significantly with 100+ jobs
- [ ] Build compiles successfully

## Skills Assessment
| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | ✅ | ✅ | Backend event aggregator, API, WebSocket handler |
| frontend | .claude/skills/frontend/SKILL.md | ✅ | ✅ | UI refactoring for trigger-based updates |

**Active Skills:** go, frontend
