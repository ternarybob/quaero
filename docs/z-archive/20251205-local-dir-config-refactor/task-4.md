# Task 4: Run API tests and fix failures
Depends: 2 | Critical: no | Model: sonnet

## Addresses User Intent
Execute the tests and iterate to success

## Do
- Run go test ./test/api/... -run TestLocalDir -v
- Fix any compilation errors
- Fix any test failures
- Iterate until all tests pass

## Accept
- [ ] All API local_dir tests pass
- [ ] No compilation errors
