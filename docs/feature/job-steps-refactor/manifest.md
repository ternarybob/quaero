# Feature: Job Steps TOML Refactor

- Slug: job-steps-refactor
- Type: feature
- Created: 2025-11-29T15:40:00Z
- Request: "Change the job steps to be defined using [step.{step-name}] syntax with flat configuration, dependencies support, and removal of redundant fields"

## Summary
Refactor job step definitions from `[[steps]]` array syntax to `[step.{name}]` table syntax for cleaner, flatter configuration. Add dependency support between steps. Remove redundant configuration fields.
