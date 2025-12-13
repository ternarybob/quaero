# Fix: Step Manager Panel Not Updating in Real-Time
- Slug: step-manager-realtime | Type: fix | Date: 2025-12-02
- Request: "The step manager is NOT updating in real time. The job queue toolbar @ top of page is updating however the step manager panel is not. Events/logs should bubble up from worker to step manager to manager."
- Prior: none

## User Intent
Fix the step manager UI panel to receive and display real-time updates from worker events/logs. Currently, the job toolbar updates correctly but the step manager panel (showing step progress, worker status, etc.) stops updating during job execution until the job completes.

## Problem Analysis
From logs and code analysis:

1. **Step Monitor IS publishing events** - Log shows `step_progress` events being published every 5 seconds
2. **WebSocket handler IS receiving them** - `EventStepProgress` subscription exists in websocket.go
3. **UI IS listening** - `jobList:updateStepProgress` event handler exists in queue.html
4. **HOWEVER**: The step events panel in the screenshot shows "Events (7)" with events only up to 06:42:54, then STOPS updating even though the job was "Running" at the time

Root Cause Analysis from logs:
- At 07:13:51: `Step monitor started for extract_keywords step`
- At 07:13:51: First `step_progress` event published
- At 07:13:56: `All step children completed` - only 5 seconds later!
- The step completed very quickly but the UI didn't show real-time progress DURING execution

The core issue: **The step events panel and progress updates rely on the step monitor's 5-second polling interval**, which means:
1. Fast jobs complete before the first progress poll
2. UI only sees initial "running" and final "completed" states
3. Real-time worker events/logs are NOT being forwarded to the step events panel

## Success Criteria
- [ ] Step manager panel updates in real-time as workers execute (not just on 5-second poll)
- [ ] Worker events/logs bubble up immediately through step manager to UI
- [ ] Events panel shows real-time worker activity during job execution
- [ ] Progress bar and status update as each child job starts/completes
