# Plan: Implement Keyword Extractor Agent with Google ADK

## Overview
Refactor the existing keyword extractor agent from raw Gemini API calls to use Google's Agent Development Kit (ADK) with proper `llmagent` agent loop pattern.

## Steps

1. **Add Google ADK Dependency**
   - Skill: @none
   - Files: `go.mod`, `go.sum`
   - User decision: no
   - Add `google.golang.org/adk` package to project dependencies

2. **Refactor Agent Service Architecture**
   - Skill: @code-architect
   - Files: `internal/services/agents/service.go`
   - User decision: no
   - Replace raw `genai.Client` with ADK's `model.LLM` interface
   - Update `AgentExecutor` interface to accept `model.LLM`
   - Refactor service initialization to use `gemini.NewModel()`

3. **Refactor Keyword Extractor Implementation**
   - Skill: @go-coder
   - Files: `internal/services/agents/keyword_extractor.go`
   - User decision: no
   - Replace raw API calls with ADK's `llmagent.New()` pattern
   - Implement agent loop with event stream processing
   - Add response cleaning for markdown code fences
   - Maintain backward-compatible input/output contract

4. **Verify Compilation and Integration**
   - Skill: @none
   - Files: All modified files
   - User decision: no
   - Test compilation with `go build -o /tmp/quaero`
   - Verify no breaking changes to existing job executors
   - Check agent service initialization in app.go

## Success Criteria
- ✅ Google ADK dependency added and resolved
- ✅ Agent service uses `model.LLM` instead of raw `genai.Client`
- ✅ Keyword extractor uses proper `llmagent` agent loop
- ✅ All code compiles without errors
- ✅ Backward compatibility maintained with existing job definitions
- ✅ Response parsing handles both clean JSON and markdown-wrapped responses
