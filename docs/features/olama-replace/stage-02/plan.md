# Plan: Update ChatService to Use Google ADK LLM Service

## Steps
1. **Remove offline mode infrastructure from ChatService**
   - Skill: @go-coder
   - Files: `internal/services/chat/chat_service.go`
   - User decision: no

2. **Verify agent_loop.go needs no changes**
   - Skill: @go-coder
   - Files: `internal/services/chat/agent_loop.go`
   - User decision: no

## Success Criteria
- ChatService GetServiceStatus() returns cloud-mode-appropriate status
- Removed port-based health checks (8086, 8087)
- Simplified status response with mode, health, service_type, and timestamp
- Agent loop verified to work through LLMService interface without changes
- All existing Chat(), GetMode(), and HealthCheck() methods continue to work correctly

## Implementation Details
The plan calls for:
- **Remove checkServerHealth() function** - TCP connection checks to local ports
- **Simplify GetServiceStatus()** - Return cloud-mode status using LLMService interface
- **Remove unused imports** - Specifically the `net` import
- **Verify interface compliance** - Agent loop already works through LLMService abstraction