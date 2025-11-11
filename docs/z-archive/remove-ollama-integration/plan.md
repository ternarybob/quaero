---
task: "Remove Ollama/llama integration. AI services will be configured as API calls instead. Core functions of collection/crawling and MCP can operate without AI integration."
complexity: high
steps: 8
---

# Plan: Remove Ollama/llama Integration

## Overview
Remove the local llama.cpp/Ollama integration from Quaero. The application will continue to function for crawling, document storage, and MCP server operations without AI/LLM capabilities. Future AI integration will be API-based only (e.g., OpenAI, Anthropic, Gemini).

## Step 1: Remove LLM Service Initialization
**Why:** LLM service is the core of llama integration - removing it first prevents downstream services from attempting to use it
**Depends:** none
**Validates:** code_compiles, follows_conventions
**Files:**
- internal/app/app.go (remove LLMService, AuditLogger initialization)
- internal/app/app.go (App struct - remove LLMService, AuditLogger fields)
**Risk:** medium
**User decision required:** no

## Step 2: Remove Chat Service (Depends on LLM)
**Why:** ChatService uses LLMService for agent-based chat - must be removed
**Depends:** 1
**Validates:** code_compiles, follows_conventions
**Files:**
- internal/app/app.go (remove ChatService initialization and field)
- internal/handlers/chat_handler.go (mark as deprecated or remove)
- internal/server/routes.go (remove chat routes)
- pages/chat.html (optional - can keep UI with "feature disabled" message)
**Risk:** low
**User decision required:** no

## Step 3: Remove LLM Configuration
**Why:** Configuration structures and defaults are no longer needed
**Depends:** 1, 2
**Validates:** code_compiles, tests_must_pass, follows_conventions
**Files:**
- internal/common/config.go (remove LLMConfig, OfflineLLMConfig, CloudLLMConfig, AuditConfig, RAGConfig)
- internal/common/config.go (NewDefaultConfig - remove LLM/RAG sections)
- internal/common/config.go (applyEnvOverrides - remove LLM env var handling)
- deployments/local/quaero.toml (remove [llm], [rag] sections)
- deployments/local/quaero-mcp.toml (remove [llm], [rag] sections)
- test/config/test-config.toml (remove [llm], [rag] sections)
**Risk:** low
**User decision required:** no

## Step 4: Remove LLM Service Implementation
**Why:** Implementation files are no longer used after removal from app initialization
**Depends:** 1, 2, 3
**Validates:** code_compiles, no_unused_imports
**Files:**
- internal/services/llm/ (entire directory - factory.go, audit.go, offline/)
- internal/interfaces/llm_service.go (entire file)
**Risk:** low
**User decision required:** no

## Step 5: Clean Up Documentation
**Why:** Remove references to llama/Ollama from user-facing documentation
**Depends:** 1, 2, 3, 4
**Validates:** follows_conventions
**Files:**
- CLAUDE.md (remove LLM Service Architecture section, update architecture diagrams)
- README.md (remove LLM/embedding sections, update features list)
- AGENTS.md (update if contains LLM references)
- docs/architecture.md (remove LLM components from diagrams)
**Risk:** low
**User decision required:** no

## Step 6: Update Build Script
**Why:** Remove llama-related checks and model directory references
**Depends:** none (independent)
**Validates:** code_compiles, use_build_script
**Files:**
- scripts/build.ps1 (remove llama-cli checks, model directory validation)
**Risk:** low
**User decision required:** no

## Step 7: Remove Server Configuration
**Why:** LlamaDir server config no longer needed
**Depends:** 3
**Validates:** code_compiles
**Files:**
- internal/common/config.go (ServerConfig - remove LlamaDir field)
- internal/common/config.go (NewDefaultConfig - remove LlamaDir default)
- internal/common/config.go (applyEnvOverrides - remove QUAERO_SERVER_LLAMA_DIR)
**Risk:** low
**User decision required:** no

## Step 8: Verify and Test
**Why:** Ensure application compiles, starts, and core features work without LLM
**Depends:** 1, 2, 3, 4, 5, 6, 7
**Validates:** code_compiles, tests_must_pass, use_build_script
**Files:**
- test/api/ (verify API tests pass)
- test/ui/ (verify UI tests pass for crawler, jobs, documents)
**Risk:** medium
**User decision required:** no

## User Decision Points
- None - all steps can proceed automatically

## Constraints
- Beta mode: Breaking changes allowed, database rebuilds acceptable
- Core functionality must remain: crawling, document storage, search, MCP server
- Chat UI can remain but show "feature disabled" message
- No backward compatibility required for LLM-related features

## Success Criteria
- Application compiles without errors
- API tests pass (except LLM/chat tests which will be removed/disabled)
- UI tests pass for core features (crawler, jobs, search)
- Application starts and serves web UI
- Crawler can collect documents
- MCP server can search documents
- No references to llama/Ollama in logs on startup
- Documentation updated to reflect removal
