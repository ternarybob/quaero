# Task 5: Update tests for new UI model
Depends: 4 | Critical: no | Model: sonnet

## Context
Existing UI tests expect step rows and child job expansion. Need to update tests to match new event-driven UI model.

## Do
- Update `test/ui/queue_test.go` test expectations
- Remove tests for step row expansion
- Add tests for events panel display
- Test WebSocket log subscription
- Test log level filtering
- Test auto-scroll behavior (if feasible)
- Ensure multi-step job tests still pass

## Tests to Modify
- `TestNearbyRestaurantsKeywordsMultiStep/ExpandCollapseChildren` → Update for events panel
- `TestNearbyRestaurantsKeywordsMultiStep/ChildJobDocumentCounts` → May need adjustment
- Add new test: `TestJobEventsDisplay`
- Add new test: `TestWebSocketLogSubscription`

## Accept
- [ ] All UI tests pass with new model
- [ ] Events panel tested
- [ ] No flaky tests
- [ ] Build and tests pass
