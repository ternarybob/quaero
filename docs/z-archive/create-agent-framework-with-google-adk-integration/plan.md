# Plan: Create Agent Framework with Google ADK Integration

## Overview
Create a new agent service layer using Google's ADK (Agent Development Kit) with Gemini models to process documents. The framework follows Quaero's established patterns: interface-based design, dependency injection, queue-based job execution, and TOML configuration.

## Architecture
Agent jobs follow the standard queue-based execution path:
- JobDefinition (TOML) → Queue → JobProcessor → AgentExecutor → AgentService → Google ADK
- Agents store results in Document.Metadata (e.g., keywords, confidence scores)
- Supports agent chaining via job steps (output from one agent feeds into the next)

## Design Decisions
1. **No Fallback Policy**: If Google API key is missing/invalid, agent jobs fail immediately
2. **Metadata Storage**: Agent results stored in existing Document.Metadata map
3. **Job Execution**: Uses standard queue-based system (matches CrawlerExecutor pattern)
4. **Agent Chaining**: Job definitions can chain multiple agent steps
5. **First Agent Type**: Keyword extraction (with confidence scores)

## Steps

### 1. Add Google ADK dependencies to go.mod
- Skill: @go-coder
- Files: `go.mod`
- User decision: no
- Actions:
  - Add `google.golang.org/adk v0.1.0`
  - Add `google.golang.org/genai v0.19.0`
  - Run `go mod tidy`

### 2. Add agent configuration to config files
- Skill: @go-coder
- Files: `internal/common/config.go`, `deployments/local/quaero.toml`
- User decision: no
- Actions:
  - Create AgentConfig struct (GoogleAPIKey, ModelName, MaxTurns, Timeout)
  - Add Agent field to main Config struct
  - Add environment variable overrides
  - Add [agent] section to quaero.toml with documentation

### 3. Create AgentService interface
- Skill: @code-architect
- Files: `internal/interfaces/agent_service.go`
- User decision: no
- Actions:
  - Define AgentService interface
  - Methods: Execute(), HealthCheck(), Close()
  - Document interface contract

### 4. Implement agent service with Google ADK
- Skill: @go-coder
- Files: `internal/services/agents/service.go`
- User decision: no
- Actions:
  - Create agents/ directory
  - Implement Service struct with Gemini model
  - Constructor validates API key and initializes model
  - RegisterAgent() for agent type registration
  - Execute() routes to registered agents
  - HealthCheck() validates connectivity

### 5. Implement keyword extraction agent
- Skill: @go-coder
- Files: `internal/services/agents/keyword_extractor.go`
- User decision: no
- Actions:
  - Implement KeywordExtractor using llmagent
  - Build instruction prompt for keyword extraction
  - Parse JSON response (keywords + confidence scores)
  - Validate output structure and keyword count

### 6. Create AgentExecutor for queue-based execution
- Skill: @go-coder
- Files: `internal/jobs/processor/agent_executor.go`
- User decision: no
- Actions:
  - Implement AgentExecutor (implements JobExecutor interface)
  - Load document, execute agent, update metadata
  - Publish DocumentUpdated events
  - Handle job status updates

### 7. Create AgentStepExecutor for job definitions
- Skill: @go-coder
- Files: `internal/jobs/executor/agent_step_executor.go`
- User decision: no
- Actions:
  - Implement AgentStepExecutor (implements StepExecutor interface)
  - Create and enqueue agent jobs
  - Poll for job completion
  - Return results for agent chaining

### 8. Integrate agent service in app initialization
- Skill: @go-coder
- Files: `internal/app/app.go`
- User decision: no
- Actions:
  - Add AgentService field to App struct
  - Initialize agent service in initServices()
  - Register AgentExecutor with JobProcessor
  - Register AgentStepExecutor with JobExecutor
  - Add cleanup in Close()

### 9. Create example job definition
- Skill: @none
- Files: `deployments/local/job-definitions/keyword-extractor-agent.toml`
- User decision: no
- Actions:
  - Create keyword extractor job definition
  - Document configuration options
  - Add commented example of agent chaining

### 10. Verify compilation and integration
- Skill: @go-coder
- Files: All modified files
- User decision: no
- Actions:
  - Run `go mod tidy` and `go build -o /tmp/quaero`
  - Verify all imports resolve
  - Confirm no compilation errors

## Success Criteria
- ✅ Google ADK dependencies added and downloaded
- ✅ Agent configuration integrated into config system
- ✅ AgentService interface defined
- ✅ Agent service implemented with Gemini model
- ✅ Keyword extraction agent functional
- ✅ AgentExecutor handles queue-based agent jobs
- ✅ AgentStepExecutor enables job definition agent steps
- ✅ Agent service integrated in app initialization
- ✅ Example job definition created
- ✅ Code compiles successfully

## Notes
- This is beta mode - breaking changes allowed
- Agent service requires valid Google API key (no fallback)
- Keyword extractor is the first agent type (more can be added later)
- Agent chaining supported through job step sequencing
