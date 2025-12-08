# Task 1: Add api_key to summary steps in codebase_assess.toml

Depends: - | Critical: no | Model: sonnet

## Addresses User Intent

Adds the missing api_key configuration to the three summary steps so they can authenticate with the Gemini API.

## Do

1. Edit `bin/job-definitions/codebase_assess.toml`
2. Add `api_key = "{google_gemini_api_key}"` to:
   - `[step.generate_index]`
   - `[step.generate_summary]`
   - `[step.generate_map]`

## Accept

- [ ] All three summary steps have api_key configured
- [ ] Variable uses existing `{google_gemini_api_key}` pattern
- [ ] TOML syntax is valid
