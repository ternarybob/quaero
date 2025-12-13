# Plan: Implement Google ADK LLM Service

## Steps
1. **Create Gemini LLM Service**
   - Skill: @go-coder
   - Files: `internal/services/llm/gemini_service.go` (new file)
   - User decision: no

2. **Add LLM Configuration Support**
   - Skill: @go-coder
   - Files: `internal/common/config.go` (modify existing)
   - User decision: no

## Success Criteria
- New `GeminiService` struct implementing the `LLMService` interface
- Configuration support for Google ADK API key and model settings
- Embedding functionality with 768-dimensional output for database compatibility
- Chat functionality using gemini-2.0-flash model
- Health check, mode detection, and resource cleanup methods
- Environment variable override support for all LLM settings

## Implementation Details
The plan calls for:
- **Embedding Model**: `gemini-embedding-001` with 768 output dimensions
- **Chat Model**: `gemini-2.0-flash` (fast and cost-effective)
- **Timeout**: 5 minutes default for LLM operations
- **Environment Variables**: QUAERO_LLM_* prefix for all configuration
- **Interface Compliance**: Follow existing `LLMService` interface from `internal/interfaces/llm_service.go`