# Fix: Step Events Architecture

- Slug: step-events-architecture | Type: fix | Date: 2025-12-02
- Request: "The events/logging is not correct. Step search_nearby_restaurants completed should NOT appear in extract_keywords. Review the service structure first, the UI should be just the receiver. Create a new API test, which performs an integration test, starts the job, monitors the web socket output. Ensure the architecture and layers comply to the rules."
- Prior: none

## User Intent

Fix the event/logging architecture so that:
1. Events from one step do NOT appear in another step's panel (e.g., "Step search_nearby_restaurants completed" should not appear in "extract_keywords")
2. The 3-layer architecture is enforced:
   - **Workers**: Only publish events/logs to step and job queues (single line: `log.debug("this goes to both queues")`)
   - **Step Manager**: Monitors workers, publishes worker/queue stats for the step, sends events to step message queue
   - **Job Manager**: Sends events for entire job, manages overall job status and worker counts
3. WebSocket only subscribes to job and step message queues (not direct worker events)
4. Database logger subscribes to job queue
5. UI receives messages from WebSocket and updates components accordingly

## Success Criteria

- [ ] No worker sends events/logs directly to WebSocket/UI
- [ ] Workers use a single logging mechanism that publishes to both step and job queues
- [ ] Step events are properly filtered by step_name in WebSocket messages
- [ ] Integration test exists that starts a job, monitors WebSocket output, and verifies events have correct step context
- [ ] Events from step A do not appear in step B's panel in the UI
- [ ] Architecture complies with the 3-layer model (Job Manager > Step Manager > Workers)
