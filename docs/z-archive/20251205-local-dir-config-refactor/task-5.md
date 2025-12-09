# Task 5: Run UI tests and fix failures
Depends: 3 | Critical: no | Model: sonnet

## Addresses User Intent
Execute the tests and iterate to success

## Do
- Run go test ./test/ui/... -run TestLocalDir -v
- Fix any compilation errors
- Fix any test failures
- Iterate until all tests pass

## Accept
- [ ] All UI local_dir tests pass
- [ ] No compilation errors
