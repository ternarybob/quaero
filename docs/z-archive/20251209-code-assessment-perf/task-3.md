# Task 3: Build and verify

Depends: 1,2 | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent
Verify changes compile and don't break existing functionality.

## Skill Patterns to Apply
- Use go build with output to /tmp

## Do
- Run `go build -o /tmp/quaero ./cmd/quaero`

## Accept
- [ ] Build succeeds
