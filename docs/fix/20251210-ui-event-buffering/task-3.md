# Task 3: Update serviceLogs Alpine component to use trigger-based refresh
Depends: 2 | Critical: no | Model: sonnet | Skill: frontend

## Addresses User Intent
Service Logs panel should fetch from API on trigger, not receive push-based individual logs.

## Skill Patterns to Apply
- Alpine.js WebSocket subscription
- API fetch pattern
- Throttling to prevent rapid fetches

## Do
1. In `serviceLogs` component in `pages/static/common.js`:
   - Add subscription to `refresh_logs` WebSocket message type
   - On trigger: call existing `loadRecentLogs()` to fetch from API
   - Add throttling: don't fetch more than once per 500ms
   - Keep existing `log` subscription for backward compatibility (but it won't be called when aggregator is active)

2. Optional: Track last fetch time to skip redundant fetches

## Accept
- [ ] serviceLogs subscribes to `refresh_logs` message
- [ ] On `refresh_logs`: fetches from `/api/logs/recent`
- [ ] Throttling prevents rapid API calls
- [ ] UI updates in batches, not per-log
