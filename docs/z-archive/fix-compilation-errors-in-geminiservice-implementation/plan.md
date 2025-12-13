# Plan: Fix Compilation Errors in GeminiService Implementation

## Steps

1. **Fix GeminiService Implementation**
   - Skill: @go-coder
   - Files: internal/services/llm/gemini_service.go
   - User decision: no

## Success Criteria
- All compilation errors resolved in gemini_service.go
- Code compiles cleanly without warnings
- Uses genai.Client for both embeddings and chat (not ADK models)
- Updated model name from text-embedding-004 to gemini-embedding-001
- Removed complex agent/runner pattern for simple direct API calls
- Health checks and Close method updated accordingly
