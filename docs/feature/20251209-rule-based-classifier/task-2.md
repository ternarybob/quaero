# Task 2: Register rule_classifier in agent service

- **Depends:** 1
- **Skill:** go
- **Critical:** no
- **Files:** internal/services/agents/service.go

## Actions
- Import rule_classifier if needed
- Add registration in `NewService()` function
- Register alongside existing agents (keyword_extractor, category_classifier, etc.)

## Acceptance
- [ ] rule_classifier is registered in agent service
- [ ] Service can execute "rule_classifier" agent type
- [ ] No compilation errors
