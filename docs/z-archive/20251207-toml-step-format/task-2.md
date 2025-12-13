# Task 2: Add test for step dependency ordering validation

Depends: 1 | Critical: no | Model: sonnet

## Addresses User Intent

User wants a test to verify that step dependencies are respected - specifically that index-files executes before generate-summary.

## Do

- Add assertion in the existing test to verify step execution order
- Check job logs or step completion order to confirm index-files runs before generate-summary
- The `depends` field should already be in the TOML - verify it's being used correctly

## Accept

- [ ] Test includes verification of step execution order
- [ ] Test confirms index-files step completes before generate-summary starts
