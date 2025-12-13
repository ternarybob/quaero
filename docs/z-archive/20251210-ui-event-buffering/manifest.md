# Fix: UI Event Buffering
- Slug: ui-event-buffering | Type: fix | Date: 2025-12-10
- Request: "1. The UI is displaying all the events (scrolling), in the step. The step workers finish within 1 second the UI should only display initial 100, then last 100. i.e. The websocket will message [step_1] start, UI will get the events from the api, websocket will says [step_1] complete, UI will get the events from the api. 2. The Service Logs also need to be updated to same buffering approach. When there is high a volume in logging, the UI is not able to keep up, and creates a bottle neck in the UI. The UI (service logs) should display the logs in batches and be triggered by the websocket."
- Prior: none

## User Intent
Fix UI performance issues caused by overwhelming event/log volume:

1. **Step Events Panel** (queue.html): When steps complete quickly (<1s), the UI should NOT try to display every event in real-time. Instead:
   - On step START: fetch initial 100 events from API
   - On step COMPLETE: fetch last 100 events from API
   - During step execution: don't flood UI with events

2. **Service Logs Panel** (service-logs.html): Apply similar buffering - instead of displaying every log in real-time, batch updates triggered by websocket and fetch from API.

The screenshot shows the problem: many API requests to `jobs?parent_id=...` being made long after the job completed, indicating the UI is overwhelmed and creating a bottleneck.

## Success Criteria
- [ ] Step Events panel fetches events on step START (initial 100)
- [ ] Step Events panel fetches events on step COMPLETE (last 100)
- [ ] Step Events panel does NOT receive/display individual events during execution
- [ ] Service Logs panel uses websocket-triggered batching (not real-time individual logs)
- [ ] Service Logs panel fetches from API on trigger (not push-based)
- [ ] No UI bottleneck from high event volume
- [ ] Build succeeds with no errors

## Skills Assessment
| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | Y | Y | Websocket handler modifications, aggregator pattern |
| frontend | .claude/skills/frontend/SKILL.md | Y | Y | Alpine.js UI components in queue.html, service-logs.html |

**Active Skills:** go, frontend
