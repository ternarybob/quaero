# Fix: TOML Step Format Output

- Slug: toml-step-format | Type: fix | Date: 2025-12-07
- Request: "combined-job-definition.toml - [step] is included and should not be. [dependencies] is missing. Specifically step (generate-summary), depends on the index-files. Include as test in the test, where index-files executes first."
- Prior: none

## User Intent

1. Remove the redundant `[step]` line from generated TOML output - only `[step.{name}]` sections should appear
2. Ensure the `depends` field is properly preserved in the TOML output (it currently is - `depends = 'index-files'` on line 11)
3. Add a test to verify step dependency ordering (index-files executes before generate-summary)

## Success Criteria

- [ ] Generated TOML does NOT contain standalone `[step]` line - only `[step.{name}]` sections
- [ ] The `depends` field is preserved in step definitions
- [ ] Test verifies step execution order respects dependencies (index-files before generate-summary)
