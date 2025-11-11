# Step 1: Update AGENTS.md with agent framework architecture

**Skill:** @none
**Files:** `AGENTS.md`

---

## Iteration 1 - COMPLETED

### Agent 2 - Implementation

Added comprehensive agent framework architecture documentation to AGENTS.md following the existing documentation style.

**Changes completed:**

- `AGENTS.md` (lines 390-569): Added Agent Framework Architecture section after Chrome Extension section (line 389), before Go Structure Standards section (line 570)
  - Overview: Description of agent framework capabilities and integration with Google ADK
  - Core Components: AgentService, AgentExecutor interface, AgentExecutor job processor, AgentStepExecutor
  - Agent Execution Flow: Complete sequence diagram from user trigger to metadata update
  - Google ADK Integration: Model initialization, agent loop pattern, no offline fallback explained
  - Agent Types: Keyword extractor implementation details and future agent types
  - Agent Chaining: How metadata flows between agents, configuration pattern, best practices
  - Configuration: TOML config section, environment variables, API key setup instructions

- `AGENTS.md` (line 263): Updated Service Initialization Flow to include Agent Service as step 12

- `AGENTS.md` (lines 1116-1173): Added Agent Service Issues subsection to Troubleshooting section
  - Agent Service Not Initialized (symptom, cause, solution, verification)
  - Agent Jobs Fail with "Unknown Agent Type" (symptom, cause, solution, available types)
  - Agent Execution Timeout (symptom, cause, solution, note on timeout scope)
  - Keywords Not Appearing in Document Metadata (symptom, cause, solution, metadata structure)
  - Gemini API Rate Limit Errors (symptom, cause, solution, free tier limits)

**Quality:** ✅ High - All documentation follows existing AGENTS.md patterns with clear section headers, code blocks, bullet points, and comprehensive troubleshooting guidance.

