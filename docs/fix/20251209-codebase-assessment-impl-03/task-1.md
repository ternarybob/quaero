# Task 1: Implement metadata_enricher agent
Depends: - | Critical: no | Model: sonnet

## Addresses User Intent
Implements the `metadata_enricher` agent type needed for the `extract_build_info` step in the codebase assessment pipeline.

## Do
- Create `internal/services/agents/metadata_enricher.go`
- Implement `AgentExecutor` interface with `Execute()` and `GetType()` methods
- Agent should extract build/run/test metadata from code documents
- Follow the same pattern as `keyword_extractor.go`

## Accept
- [ ] File `metadata_enricher.go` exists with proper structure
- [ ] Implements `Execute()` that calls Gemini API with appropriate prompt
- [ ] Returns structured output with build_commands, run_commands, test_commands, dependencies
- [ ] `GetType()` returns "metadata_enricher"
