# Task 2: Replace emoji prefixes with clean messages

Depends: 1 | Critical: no | Model: sonnet

## Addresses User Intent

Replace emoji log levels with clean text - User Intent #4. Enable `[worker]` context display.

## Do

1. Update `internal/queue/workers/crawler_worker.go`:
   - Replace `✓ Completed:` with `Completed:`
   - Replace `✗ Failed:` with `Failed:`
   - Replace `▶ Started:` with `Started:`
   - Replace any other emoji prefixes
2. Ensure worker logs have originator="worker" set correctly
3. Keep message content informative (URL, elapsed time, etc.)

## Accept

- [ ] No emoji characters in crawler worker log messages
- [ ] Worker logs include useful context (URL, timing)
- [ ] originator field set to "worker" for all crawler logs
- [ ] Code compiles without errors
