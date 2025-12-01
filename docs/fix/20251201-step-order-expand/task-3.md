# Task 3: Run Tests and Verify
Depends: 2 | Critical: no | Model: sonnet

## Do
1. Run `go build ./...` to verify compilation
2. Run `go test ./test/ui/... -run TestNearbyRestaurantsKeywordsMultiStep -v`
3. Verify step order is correct in UI
4. Verify child jobs expand when clicked

## Accept
- [ ] Build succeeds
- [ ] Tests pass or show expected behavior
- [ ] Step order matches dependency order in UI
