# Task 9: Remove Redundant Code and TOML Fields

- Group: 9 | Mode: sequential | Model: sonnet
- Skill: @code-reviewer | Critical: no | Depends: 7, 8
- Sandbox: /tmp/3agents/task-9/ | Source: ./ | Output: ./docs/feature/20251129-job-type-workers/

## Files
- `internal/models/job_definition.go` - Remove deprecated fields
- `internal/jobs/service.go` - Remove backward compatibility code
- `internal/queue/managers/*.go` - Remove old managers if consolidated
- `internal/queue/orchestrator.go` - Clean up old routing logic

## Requirements

1. Remove deprecated TOML fields support:
   - Remove `action` field parsing (now `type`)
   - Remove `name` field in step config
   - Remove backward compatibility warnings

2. Remove redundant code:
   - Old manager implementations if fully migrated to workers
   - Old routing logic in orchestrator
   - Unused helper functions

3. Clean up models:
   - Remove `Action` field from JobStep if fully migrated
   - Remove `JobDefinition.Type` if steps now define type
   - Update validation to not check removed fields

4. Final cleanup:
   - Remove commented code
   - Remove TODO comments that are now done
   - Ensure consistent naming

## Acceptance
- [ ] Deprecated fields removed
- [ ] Redundant code removed
- [ ] Models cleaned up
- [ ] No backward compatibility code remains
- [ ] Compiles: `go build ./...`
- [ ] Tests pass: `go test ./...`
