# Task 1: Fix saveJobToml to not output redundant [step] line

Depends: - | Critical: no | Model: sonnet

## Addresses User Intent

Removes the redundant `[step]` standalone line from TOML output - user expects only `[step.{name}]` sections.

## Do

- Modify `saveJobToml` in `test/ui/local_dir_jobs_test.go` to generate clean TOML without redundant `[step]` header
- The go-toml library generates `[step]` when marshalling `map[string]map[string]interface{}`
- Solution: Build TOML string manually or post-process to remove redundant line

## Accept

- [ ] Generated TOML contains only `[step.{name}]` sections, no standalone `[step]`
- [ ] Code compiles successfully
