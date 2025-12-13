# Task 4: Register all three agents in agent service
Depends: 1,2,3 | Critical: no | Model: sonnet

## Addresses User Intent
Ensures all three new agent types are available for the codebase assessment pipeline by registering them in the agent service.

## Do
- Edit `internal/services/agents/service.go`
- In `NewService()` function, add registration for:
  - MetadataEnricher
  - CategoryClassifier
  - EntityRecognizer
- Follow the same pattern as keyword_extractor registration

## Accept
- [ ] All three agents are registered in NewService()
- [ ] Code compiles without errors
- [ ] Agents are available when agentService.Execute() is called
