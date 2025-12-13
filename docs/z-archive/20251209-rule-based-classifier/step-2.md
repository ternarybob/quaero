# Step 2: Register rule_classifier in agent service

- **Status:** complete
- **Skill:** go
- **Duration:** ~30s

## Files Modified
- `internal/services/agents/service.go` - Added registration for RuleClassifier

## Skill Compliance
- [x] Error wrapping with context - N/A
- [x] Structured logging (arbor) - N/A
- [x] Interface-based DI - Follows existing pattern
- [x] Constructor injection - Follows existing pattern

## Changes
Added registration in `NewService()`:
```go
ruleClassifier := &RuleClassifier{}
service.RegisterAgent(ruleClassifier)
```

## Notes
- Registered alongside existing agents (keyword_extractor, category_classifier, etc.)
- Service can now execute "rule_classifier" agent type via `Execute(ctx, "rule_classifier", input)`
