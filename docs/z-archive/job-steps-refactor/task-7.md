# Task 7: Remove Deprecated Code and Clean Up

- Group: 7 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 6
- Sandbox: /tmp/3agents/task-7/ | Source: ./ | Output: ./docs/feature/job-steps-refactor/

## Requirements

1. Remove backward compatibility code for [[steps]] format (if added)

2. Clean up any unused fields/structs:
   - Old nested config handling code
   - Legacy parsing branches

3. Update code comments and documentation

4. Run full test suite:
   ```bash
   go build ./...
   go test ./...
   ```

5. Update plan.md in workdir with completion status

## Acceptance
- [ ] No deprecated code remains
- [ ] All tests pass
- [ ] Build succeeds
- [ ] Code is clean and well-documented
