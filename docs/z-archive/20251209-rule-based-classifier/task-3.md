# Task 3: Add rule_classifier to valid agent types in agent_worker

- **Depends:** 2
- **Skill:** go
- **Critical:** no
- **Files:** internal/queue/workers/agent_worker.go

## Actions
- Add "rule_classifier" to `validAgentTypes` map in `ValidateConfig` method
- This allows the agent_worker to accept rule_classifier as a valid agent type in job definitions

## Acceptance
- [ ] "rule_classifier" is in validAgentTypes map
- [ ] Agent jobs with agent_type="rule_classifier" pass validation
- [ ] No compilation errors
