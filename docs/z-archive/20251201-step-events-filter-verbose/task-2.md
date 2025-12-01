# Task 2: Add MinUILogLevel option to JobLogOptions

Depends: 1 | Critical: no | Model: sonnet

## Do

1. Add `MinUILogLevel` field to `JobLogOptions` struct in manager.go
2. Default to INFO if not specified
3. Allow workers to override (e.g., send DEBUG to UI for special cases)

## Accept

- [ ] JobLogOptions has MinUILogLevel field
- [ ] Default behavior remains INFO minimum for UI
- [ ] Workers can override to allow lower levels if needed
