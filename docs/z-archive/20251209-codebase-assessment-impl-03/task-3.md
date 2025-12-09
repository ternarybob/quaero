# Task 3: Implement entity_recognizer agent
Depends: - | Critical: no | Model: sonnet

## Addresses User Intent
Implements the `entity_recognizer` agent type needed for the `identify_components` step in the codebase assessment pipeline.

## Do
- Create `internal/services/agents/entity_recognizer.go`
- Implement `AgentExecutor` interface with `Execute()` and `GetType()` methods
- Agent should identify key components, entry points, and important entities in code
- Follow the same pattern as `keyword_extractor.go`

## Accept
- [ ] File `entity_recognizer.go` exists with proper structure
- [ ] Implements `Execute()` that calls Gemini API with appropriate prompt
- [ ] Returns structured output with components, entry_points, exports, dependencies
- [ ] `GetType()` returns "entity_recognizer"
