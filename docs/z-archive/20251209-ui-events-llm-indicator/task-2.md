# Task 2: Verify LLM indicator in agent_worker.go

Depends: - | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent
Verify that events clearly indicate whether a worker is making an LLM call ("AI:") or a rule-based operation ("Rule:").

## Skill Patterns to Apply
- Code review
- Log message verification

## Do
- Review agent_worker.go to confirm LLM indicator logic is correct
- Verify `IsRuleBased()` is called and used for log prefix
- Confirm event messages include the correct prefix

## Accept
- [ ] `IsRuleBased()` determines log prefix
- [ ] Events show "AI: {agent}" for LLM agents
- [ ] Events show "Rule: {agent}" for rule-based agents
