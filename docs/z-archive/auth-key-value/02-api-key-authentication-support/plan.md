# Plan: API Key Authentication Support Implementation

## Overview
Implement comprehensive API key authentication support following the detailed analysis in `02-api-key-authentication-support.md`. The plan involves 4 phases to add file-based API key storage, UI management, and integration with existing services.

## Steps

### Step 1: File Loading Infrastructure
- **Skill:** @go-coder
- **Files:** `internal/storage/sqlite/load_auth_credentials.go(NEW)`, `internal/app/app.go(MODIFY)`
- **Description:** Create `load_auth_credentials.go` following the exact pattern from `load_job_definitions.go`, then integrate into app startup sequence.
- **User decision:** No
- **Status:** ✅ COMPLETED

### Step 2: API Routes Registration
- **Skill:** @go-coder
- **Files:** `internal/server/routes.go(MODIFY)`, `internal/handlers/auth_handler.go(MODIFY)`
- **Description:** Register API key CRUD routes in routes.go and update handleAuthRoutes to route to new handlers.
- **User decision:** No
- **Status:** ✅ COMPLETED

### Step 3: UI Implementation
- **Skill:** @go-coder
- **Files:** `pages/auth.html(MODIFY)`, `internal/handlers/auth_handler.go(MODIFY)`
- **Description:** Add API Keys section to auth.html with Alpine.js component for CRUD operations, and update auth_handler.go to mask API keys in responses.
- **User decision:** No
- **Status:** ✅ COMPLETED

### Step 4: Example Files & Documentation
- **Skill:** @none
- **Files:** `deployments/local/auth/(NEW)`, `deployments/local/auth/.gitignore(NEW)`, `deployments/local/auth/example-api-keys.toml(NEW)`, `deployments/local/quaero.toml(MODIFY)`
- **Description:** Create example TOML files, .gitignore, and update config documentation with auth storage guidance.
- **User decision:** No
- **Status:** ✅ COMPLETED

## Success Criteria
- ✅ API keys can be loaded from TOML files at startup
- ✅ CRUD operations work via REST API endpoints
- ✅ UI provides secure management interface with key masking
- ✅ Example files demonstrate proper usage patterns
- ✅ Configuration documentation explains auth storage features
- ✅ Security: API keys masked in all responses, never logged in plain text
- ✅ Separation: Cookie and API key auth displayed in separate sections
- ✅ Integration: Works with existing LLM, Agent, and Places services
