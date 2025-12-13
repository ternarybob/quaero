# Task 2: Remove AddJobLogWithEvent and JobLogOptions

Depends: 1 | Critical: no | Model: sonnet

## Addresses User Intent
Removes the unnecessary complexity of dual logging methods and options struct.

## Do
1. Delete the `JobLogOptions` struct from manager.go
2. Delete the `AddJobLogWithEvent` method from manager.go
3. Delete the `shouldPublishLogToUI` function (logic moved to AddJobLog)
4. Ensure no compile errors from removal

## Accept
- [ ] `JobLogOptions` struct is removed
- [ ] `AddJobLogWithEvent` method is removed
- [ ] `shouldPublishLogToUI` function is removed
- [ ] Code compiles (may have errors in workers that still reference these - that's expected, fixed in Task 3)
