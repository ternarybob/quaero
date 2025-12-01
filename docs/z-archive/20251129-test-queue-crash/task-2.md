# Task 2: Run test to verify fix

- Group: 2 | Mode: sequential | Model: sonnet
- Skill: @test-automator | Critical: no | Depends: 1
- Sandbox: /tmp/3agents/task-2/ | Source: ./ | Output: ./docs/fix/20251129-test-queue-crash/

## Files
- `test/ui/queue_test.go` - Test file to run

## Requirements
1. Run TestNewsCrawlerCrash test
2. Verify it no longer deadlocks in TestMain
3. If test fails for different reason, diagnose and fix

## Acceptance
- [ ] Test passes TestMain without deadlock
- [ ] TestNewsCrawlerCrash runs (may complete or timeout based on service)
