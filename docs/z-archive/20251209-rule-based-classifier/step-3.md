# Step 3: Add rule_classifier to valid agent types in agent_worker

- **Status:** complete
- **Skill:** go
- **Duration:** ~30s

## Files Modified
- `internal/queue/workers/agent_worker.go` - Added "rule_classifier" to validAgentTypes map

## Skill Compliance
- [x] Error wrapping with context - N/A
- [x] Structured logging (arbor) - N/A
- [x] Interface-based DI - N/A
- [x] Constructor injection - N/A

## Changes
Added to `ValidateConfig` method:
```go
validAgentTypes := map[string]bool{
    // ... existing types ...
    "rule_classifier":     true,
    // ...
}
```

## Notes
- Agent jobs with `agent_type="rule_classifier"` now pass validation
- Worker can dispatch rule_classifier jobs to agent service
