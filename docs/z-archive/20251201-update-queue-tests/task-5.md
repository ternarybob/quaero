# Task 5: Run Tests and Verify They Fail
Depends: 4 | Critical: no | Model: sonnet

## Do
1. Run `go test ./test/ui/... -run TestNearbyRestaurantsKeywordsMultiStep -v`
2. Capture the test output
3. Verify that the new tests fail as expected
4. Document which tests fail and why

## Accept
- [ ] Tests run successfully (no compile errors)
- [ ] New tests FAIL (as expected given current codebase bugs)
- [ ] Failure reasons are documented
