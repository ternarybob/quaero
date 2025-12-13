# Plan: UI Event Buffering Fix
Type: fix | Workdir: ./docs/fix/20251210-ui-event-buffering/

## User Intent (from manifest)
Fix UI performance issues caused by overwhelming event/log volume:

1. **Step Events Panel** (queue.html): When steps complete quickly (<1s), the UI should NOT try to display every event in real-time. Instead:
   - On step START: fetch initial 100 events from API
   - On step COMPLETE: fetch last 100 events from API
   - During step execution: don't flood UI with events

2. **Service Logs Panel** (service-logs.html): Apply similar buffering - instead of displaying every log in real-time, batch updates triggered by websocket and fetch from API.

## Active Skills
- go (websocket handler, aggregator patterns)
- frontend (Alpine.js components)

## Analysis

### Current Architecture
1. **Step Events**: Already has a `StepEventAggregator` in `internal/services/events/aggregator.go` that sends `refresh_step_events` websocket messages with `finished=true/false` flag
2. **Service Logs**: Currently subscribes to individual `log` events via WebSocket and adds each log directly to the UI
3. The problem: UI fetches events on EVERY `refresh_step_events` message, not just START/COMPLETE

### Solution Approach
1. **Step Events**: Modify `refreshStepEvents()` in queue.html to ONLY fetch when:
   - First event for a step (step just started)
   - `finished=true` flag is set (step completed)
   - Skip fetches during middle of step execution

2. **Service Logs**: Create a similar aggregator pattern:
   - Backend: Create `LogEventAggregator` that batches log events and sends periodic `refresh_logs` trigger
   - Frontend: Service Logs component fetches from `/api/logs/recent` when triggered, not on each log

## Tasks
| # | Desc | Depends | Critical | Model | Skill |
|---|------|---------|----------|-------|-------|
| 1 | Modify queue.html refreshStepEvents to only fetch on START/COMPLETE | - | no | sonnet | frontend |
| 2 | Create LogEventAggregator for service logs buffering | - | no | sonnet | go |
| 3 | Update serviceLogs Alpine component to use trigger-based refresh | 2 | no | sonnet | frontend |
| 4 | Build and verify no errors | 1,2,3 | no | sonnet | go |

## Order
[1,2] -> [3] -> [4]
