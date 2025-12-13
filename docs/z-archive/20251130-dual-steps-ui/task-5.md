# Task 5: Create and run multi-step job test
- Group: 5 | Mode: sequential | Model: sonnet
- Skill: @test-automator | Critical: no | Depends: 2,3,4
- Sandbox: /tmp/3agents/task-5/ | Source: ./ | Output: docs/feature/20251130-dual-steps-ui/

## Files
- `test/ui/queue_test.go` - add TestNearbyRestaurantsKeywordsMultiStep

## Requirements
Create test following TestNewsCrawlerCrash pattern:
1. Trigger multi-step job via UI
2. Monitor until completion (not hung in running)
3. Verify documents created
4. Verify keywords extracted

Test must pass - iterate if failures.

## Acceptance
- [ ] Test created
- [ ] Test passes
- [ ] Job completes (not hung)
- [ ] Documents created
