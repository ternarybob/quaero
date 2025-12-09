# Plan: Fix TOML Step Format Output

Type: fix | Workdir: ./docs/fix/20251207-toml-step-format/

## User Intent (from manifest)

1. Remove the redundant `[step]` line from generated TOML output - only `[step.{name}]` sections should appear
2. Ensure the `depends` field is properly preserved in the TOML output
3. Add a test to verify step dependency ordering (index-files executes before generate-summary)

## Tasks

| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Fix saveJobToml to not output redundant [step] line | - | no | sonnet |
| 2 | Add test for step dependency ordering validation | 1 | no | sonnet |
| 3 | Verify TOML output format is correct | 2 | no | sonnet |

## Order

[1] → [2] → [3]
