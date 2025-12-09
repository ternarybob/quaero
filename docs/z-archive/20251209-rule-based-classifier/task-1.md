# Task 1: Create rule_classifier agent with pattern-based classification

- **Depends:** none
- **Skill:** go
- **Critical:** no
- **Files:** internal/services/agents/rule_classifier.go

## Actions
- Create `rule_classifier.go` implementing `AgentExecutor` interface
- Implement pattern matching rules for file classification
- Use filepath/path patterns and regex for matching
- Return structured output matching category_classifier format
- Handle "unknown" category for unmatched files

## Classification Output Format
```go
{
  "category": "source|test|config|docs|build|ci|script|data|unknown",
  "subcategory": "specific-type",
  "purpose": "Brief description",
  "importance": "high|medium|low",
  "tags": ["tag1", "tag2"],
  "rule_matched": "pattern-name-or-empty"
}
```

## Acceptance
- [ ] File `rule_classifier.go` exists with proper structure
- [ ] Implements `AgentExecutor` interface (Execute, GetType)
- [ ] `GetType()` returns "rule_classifier"
- [ ] Correctly classifies `*_test.go` files as test
- [ ] Correctly classifies `main.go` as source/entrypoint
- [ ] Returns "unknown" for ambiguous files
- [ ] No LLM calls - pure pattern matching
