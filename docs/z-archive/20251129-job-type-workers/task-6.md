# Task 6: Execute Tests and Fix Failures

- Group: 6 | Mode: sequential | Model: sonnet
- Skill: @debugger | Critical: no | Depends: 5
- Sandbox: /tmp/3agents/task-6/ | Source: ./ | Output: ./docs/feature/20251129-job-type-workers/

## Files
- `test/api/*.go` - API tests
- `test/ui/*.go` - UI tests
- Any files requiring fixes based on test failures

## Requirements

1. Run full test suite:
   ```bash
   go test ./test/api/... ./test/ui/... -v
   ```

2. Analyze any failures:
   - Identify root cause
   - Determine if test needs update or code has bug
   - Fix accordingly

3. Common expected issues:
   - Tests checking for `action` field instead of `type`
   - Mock data using old format
   - Validation errors from missing `type` field

4. Iterate until all tests pass

## Acceptance
- [ ] All API tests pass
- [ ] All UI tests pass
- [ ] No regressions introduced
- [ ] Test output documented
