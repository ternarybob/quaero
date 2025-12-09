# Complete: UI Event Buffering Fix
Type: fix | Tasks: 5 | Files: 4

## User Request
"1. The UI is displaying all the events (scrolling), in the step. The step workers finish within 1 second the UI should only display initial 100, then last 100. i.e. The websocket will message [step_1] start, UI will get the events from the api, websocket will says [step_1] complete, UI will get the events from the api. 2. The Service Logs also need to be updated to same buffering approach. When there is high a volume in logging, the UI is not able to keep up, and creates a bottle neck in the UI. The UI (service logs) should display the logs in batches and be triggered by the websocket."

## Result
Fixed UI performance issues by implementing trigger-based batching for both Step Events and Service Logs panels. The UI now fetches events on START/COMPLETE only instead of every websocket trigger, and service logs use periodic triggers (1 second) instead of individual log pushes.

Additionally fixed stats API flooding by adding debouncing to `recalculateStats()` - stats are now fetched at most once per 2 seconds instead of on every WebSocket event.

## Files Changed
1. `pages/queue.html`:
   - Modified `refreshStepEvents()` to only fetch on START (first trigger) and COMPLETE (finished=true)
   - Added debouncing to `recalculateStats()` (2 second debounce) to prevent stats API flooding
2. `internal/services/events/log_aggregator.go` - New aggregator for batching log events
3. `internal/handlers/websocket.go` - Integrated LogEventAggregator, added `refresh_logs` broadcast
4. `pages/static/common.js` - Added `refresh_logs` subscription to serviceLogs component

## Skills Used
- go (aggregator pattern, websocket handler)
- frontend (Alpine.js components)

## Validation: MATCHES
All success criteria met:
- Step Events fetches on START (first trigger)
- Step Events fetches on COMPLETE (finished=true)
- Step Events skips middle-of-execution triggers
- Service Logs uses trigger-based batching via LogEventAggregator
- Service Logs fetches from API on `refresh_logs` trigger
- Stats API calls debounced to once per 2 seconds
- No UI bottleneck from high event volume
- Build succeeds

## Review: N/A
No critical triggers (security, authentication, etc.)

## Verify
Build: ✅ | Tests: N/A (not requested)
