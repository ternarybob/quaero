# Task 3: Change API default to return INFO+ logs

Depends: 2 | Critical: no | Model: sonnet

## Do

1. Modify `/api/jobs/{id}/logs` handler to default to INFO+ logs when no level specified
2. Only return ALL logs (including debug) when `level=all` is explicitly passed
3. Keep backward compatibility - level=debug/warn/error/info still work as before

## Accept

- [ ] API returns INFO+ logs by default (no level param)
- [ ] API returns ALL logs only when level=all is passed
- [ ] Existing level filters (debug, info, warn, error) still work
