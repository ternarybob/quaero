# Task 4: Update Managers for Flat Config

- Group: 4 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 3
- Sandbox: /tmp/3agents/task-4/ | Source: ./ | Output: ./docs/feature/job-steps-refactor/

## Files
- `internal/queue/managers/agent_manager.go` - Update config reading
- `internal/queue/managers/web_search_manager.go` - Update config reading
- Any other managers that read step.Config

## Requirements

1. Update managers to read flat config:
   - `stepConfig["document_filter"]` → `stepConfig["filter_*"]` fields
   - Direct field access instead of nested maps

2. Example changes for agent_manager.go:
   ```go
   // OLD
   filter := stepConfig["document_filter"].(map[string]interface{})
   limit := filter["limit"].(int)

   // NEW
   limit := stepConfig["filter_limit"].(int)
   tags := stepConfig["filter_tags"].([]interface{})
   ```

3. Search for all managers reading nested config and update them

## Acceptance
- [ ] Agent manager reads flat filter_* fields
- [ ] Web search manager reads flat config
- [ ] All managers compile with new config structure
- [ ] Compiles: `go build ./...`
