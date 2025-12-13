# Task 2: Implement category_classifier agent
Depends: - | Critical: no | Model: sonnet

## Addresses User Intent
Implements the `category_classifier` agent type needed for the `classify_files` step in the codebase assessment pipeline.

## Do
- Create `internal/services/agents/category_classifier.go`
- Implement `AgentExecutor` interface with `Execute()` and `GetType()` methods
- Agent should classify document/file purpose and role (e.g., "source", "test", "config", "docs")
- Follow the same pattern as `keyword_extractor.go`

## Accept
- [ ] File `category_classifier.go` exists with proper structure
- [ ] Implements `Execute()` that calls Gemini API with appropriate prompt
- [ ] Returns structured output with category, subcategory, purpose, confidence
- [ ] `GetType()` returns "category_classifier"
