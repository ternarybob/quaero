# Task 13: Build verification and cleanup

Depends: 10,11,12 | Critical: no | Model: sonnet

## Addresses User Intent

Ensure everything builds, tests pass, and no temporary files remain.

## Do

- Run `go build ./...` to verify compilation
- Run unit tests: `go test ./internal/jobs/actions/...`
- Run API tests: `go test ./test/api/devops_api_test.go`
- Run UI tests: `go test ./test/ui/devops_enrichment_test.go`
- Cleanup /tmp/3agents/ sandbox directory
- Verify no uncommitted debug code or TODOs

## Accept

- [ ] `go build ./...` succeeds
- [ ] All unit tests pass
- [ ] All API tests pass
- [ ] All UI tests pass (or skip if no Chrome)
- [ ] /tmp/3agents/ cleaned up
- [ ] No debug code left behind
