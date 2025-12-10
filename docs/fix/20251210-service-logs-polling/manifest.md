# Fix: Service Logs Duplicate Polling
- Slug: service-logs-polling | Type: fix | Date: 2025-12-10
- Request: "The service logs are doubling up on requests, even though nothing is happening. Service logs UI should operate on websocket trigger. Backend should monitor logs and provide triggers via websockets - default to every second, or if no logs in last second, don't refresh. Page load should get last 100."
- Prior: none

## User Intent
Stop unnecessary polling/duplicate requests in Service Logs UI:
1. **WebSocket-driven refresh** - UI should only fetch logs when backend sends "refresh_logs" trigger
2. **Smart backend triggers** - Backend should only send trigger if logs occurred in last second (no activity = no trigger)
3. **Initial page load** - Get last 100 logs on page load
4. **No duplicate requests** - Stop the continuous /recent API calls when idle

## Success Criteria
- [ ] No /api/logs/recent calls when service is idle (no log activity)
- [ ] Logs refresh via WebSocket trigger only when new logs exist
- [ ] Page load fetches last 100 logs initially
- [ ] Backend only sends refresh_logs when hasPendingLogs is true
- [ ] Network panel shows no duplicate/unnecessary requests

## Skills Assessment
| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | Yes | Yes | Backend log aggregator changes |
| frontend | .claude/skills/frontend/SKILL.md | Yes | Yes | UI service logs component changes |

**Active Skills:** go, frontend
