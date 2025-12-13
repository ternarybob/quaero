# Plan: Implement Complete Backend for API Key Authentication Support

## Steps

1. **Database Schema Extension**
   - Skill: @go-coder
   - Files: internal/storage/sqlite/schema.go, internal/models/auth.go
   - User decision: no

2. **Storage Layer Updates**
   - Skill: @go-coder
   - Files: internal/storage/sqlite/auth_storage.go, internal/interfaces/storage.go
   - User decision: no

3. **API Key Resolution Helper**
   - Skill: @code-architect
   - Files: internal/common/config.go
   - User decision: no

4. **Service Integration Updates**
   - Skill: @go-coder
   - Files: internal/services/llm/gemini_service.go, internal/services/agents/service.go, internal/services/places/service.go
   - User decision: no

5. **Application Integration**
   - Skill: @go-coder
   - Files: internal/app/app.go, internal/jobs/manager/agent_manager.go, internal/jobs/manager/places_search_manager.go
   - User decision: no

6. **Auth Handler Extensions**
   - Skill: @go-coder
   - Files: internal/handlers/auth_handler.go, internal/server/routes.go, internal/handlers/job_definition_handler.go
   - User decision: no

7. **File Loading System**
   - Skill: @go-coder
   - Files: internal/storage/sqlite/load_auth_credentials.go, deployments/local/auth/example-api-keys.toml, deployments/local/quaero.toml
   - User decision: no

8. **Final Integration and Testing**
   - Skill: @test-writer
   - Files: All modified files
   - User decision: no

## Success Criteria
- Database schema extended with api_key and auth_type fields
- Storage layer supports API key CRUD operations
- Services can resolve API keys from auth storage with config fallback
- Job definitions can reference API keys by name
- File-based API key loading system implemented
- API endpoints for API key management
- All existing functionality preserved with backward compatibility
- Complete backward compatibility maintained for cookie-based auth