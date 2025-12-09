# Task 3: Verify TOML output format is correct

Depends: 2 | Critical: no | Model: sonnet

## Addresses User Intent

Final verification that the TOML output matches expected format.

## Do

- Build the test code
- Verify generated TOML has correct structure
- Confirm `depends` field is preserved

## Accept

- [ ] Build passes
- [ ] TOML output format is correct (no `[step]` only `[step.{name}]`)
- [ ] `depends` field preserved in step definitions
