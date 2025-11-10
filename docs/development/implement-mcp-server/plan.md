---
task: "Implement MCP Server for Quaero"
complexity: medium
steps: 10
---

# Implementation Plan: MCP Server for Quaero

## Overview

This plan details the implementation of a Model Context Protocol (MCP) server that exposes Quaero's search functionality to AI assistants like Claude. The implementation leverages the existing `internal/services/search` package and wraps it with a thin MCP stdio/JSON-RPC interface.

**Current Status:** MCP server skeleton exists in `cmd/quaero-mcp/main.go` (379 lines) but has compilation issues and needs refinement.

**Architectural Impact:** LOW - This is an additive change that wraps existing search functionality without modifying core services.

---

## Step 1: Fix Dependency Issues and Verify Compilation

**Why:** The MCP server currently has missing go.sum entries that prevent compilation. Must establish a clean baseline before making improvements.

**Depends:** none

**Validates:**
- go.mod has all required dependencies
- MCP SDK (github.com/mark3labs/mcp-go v0.43.0) and transitive dependencies are properly resolved
- Code compiles without errors

**Files:**
- `go.mod`
- `go.sum`

**Commands:**
```powershell
cd C:\development\quaero
go get github.com/mark3labs/mcp-go/mcp@v0.43.0
go mod tidy
go build -o bin/quaero-mcp.exe ./cmd/quaero-mcp
```

**Risk:** low

**User decision required:** no

**Acceptance Criteria:**
- ✅ `go build ./cmd/quaero-mcp` completes without errors
- ✅ Binary created at `bin/quaero-mcp.exe`
- ✅ All transitive dependencies (jsonschema, cast, uritemplate) in go.sum

---

## Step 2: Fix Tool Handler Function Signatures

**Why:** The current implementation has inconsistent tool handler signatures. Some use the correct `mcp.CallToolRequest` signature while others use the old `map[string]interface{}` pattern.

**Depends:** Step 1 (compilation must work)

**Validates:**
- All tool handlers use consistent `mcp.ToolHandler` signature
- Handlers accept `context.Context` and `mcp.CallToolRequest`
- Return values are `(*mcp.CallToolResult, error)`

**Files:**
- `cmd/quaero-mcp/main.go` (lines 182-262: handleGetDocument, handleListRecent, handleGetRelated)

**Changes:**
1. Update `handleGetDocument` signature from `func(args map[string]interface{})` to `func(ctx context.Context, request mcp.CallToolRequest)`
2. Update `handleListRecent` signature similarly
3. Update `handleGetRelated` signature similarly
4. Fix parameter extraction to use `request.Params.Arguments` instead of `args`

**Risk:** low

**User decision required:** no

**Acceptance Criteria:**
- ✅ All handlers follow same pattern as `handleSearchDocuments` (lines 124-179)
- ✅ Type assertions use `request.Params.Arguments["param_name"].(type)`
- ✅ Code compiles without errors

---

## Step 3: Fix Type Mismatches in Formatter Functions

**Why:** The formatter functions use `*interfaces.Document` but should use `*models.Document` (the actual type returned by search service).

**Depends:** Step 2 (handlers must be fixed first)

**Validates:**
- Formatter functions accept correct document type
- No interface/struct type mismatches
- All field accesses are valid for models.Document

**Files:**
- `cmd/quaero-mcp/main.go` (lines 264-379: formatSearchResults, formatDocument, formatRecentDocuments, formatRelatedDocuments)

**Changes:**
1. Change all `*interfaces.Document` to `*models.Document` in function signatures
2. Verify field access compatibility (Title, SourceType, SourceID, URL, ContentMarkdown, Metadata, CreatedAt, UpdatedAt)
3. Import `github.com/ternarybob/quaero/internal/models` (already imported on line 14)

**Risk:** low

**User decision required:** no

**Acceptance Criteria:**
- ✅ All formatter functions accept `*models.Document` or `[]*models.Document`
- ✅ Field accesses match models.Document struct definition
- ✅ Code compiles without errors

---

## Step 4: Reduce Code to < 200 Lines (Constraint Compliance)

**Why:** Project requires MCP wrapper to be minimal (< 200 lines). Current implementation is 379 lines. Need to extract formatters to separate file.

**Depends:** Step 3 (types must be correct before refactoring)

**Validates:**
- main.go is under 200 lines
- Code organization follows Quaero patterns
- Functionality is preserved

**Files:**
- `cmd/quaero-mcp/main.go` (reduce to ~150 lines)
- `cmd/quaero-mcp/formatters.go` (NEW - extract ~200 lines of formatting logic)

**Changes:**
1. Create `cmd/quaero-mcp/formatters.go` with package `main`
2. Move all `format*` functions (formatSearchResults, formatDocument, formatRecentDocuments, formatRelatedDocuments) to formatters.go
3. Keep only core MCP setup and tool registration in main.go

**Risk:** low

**User decision required:** no

**Acceptance Criteria:**
- ✅ main.go is 140-160 lines (setup, tool registration, handlers)
- ✅ formatters.go is 200-220 lines (formatting logic)
- ✅ Both files compile together
- ✅ No duplicate code between files

---

## Step 5: Add Build Script Integration

**Why:** MCP server should be built alongside main Quaero binary using the project's build.ps1 script.

**Depends:** Step 4 (code must be clean and minimal)

**Validates:**
- MCP binary is built during standard build process
- Binary placed in bin/ directory
- Build follows Windows PowerShell conventions

**Files:**
- `scripts/build.ps1` (add MCP server build step)

**Changes:**
1. Add MCP build command after main quaero binary build
2. Build to `bin/quaero-mcp.exe`
3. Use same error handling pattern as main build
4. Log MCP build step with Write-Host

**Risk:** low

**User decision required:** no

**Acceptance Criteria:**
- ✅ `.\scripts\build.ps1` builds both quaero.exe and quaero-mcp.exe
- ✅ Both binaries in bin/ directory
- ✅ Build output shows MCP compilation step
- ✅ Build fails gracefully if MCP compilation fails

---

## Step 6: Create MCP Configuration Documentation

**Why:** Users need to know how to configure and use the MCP server with Claude CLI.

**Depends:** Step 5 (build must work before documenting usage)

**Validates:**
- Clear setup instructions for Claude CLI integration
- Configuration examples provided
- Troubleshooting guidance included

**Files:**
- `docs/implement-mcp-server/mcp-configuration.md` (NEW)
- `README.md` (add MCP server section)

**Changes:**
1. Create mcp-configuration.md with:
   - Claude CLI setup steps
   - MCP server configuration in claude_desktop_config.json
   - Database path configuration
   - Tool usage examples
2. Add MCP section to README.md with link to detailed docs

**Risk:** low

**User decision required:** no

**Acceptance Criteria:**
- ✅ Documentation includes working claude_desktop_config.json example
- ✅ Clear explanation of each tool (search_documents, get_document, list_recent_documents, get_related_documents)
- ✅ Troubleshooting section covers common issues
- ✅ Example queries for each tool type

---

## Step 7: Create API Integration Test

**Why:** Verify MCP server integrates correctly with search service using real database queries.

**Depends:** Step 6 (implementation must be complete)

**Validates:**
- MCP server can initialize with test config
- Search service integration works
- Tool handlers return valid responses
- Database queries execute correctly

**Files:**
- `test/api/mcp_server_test.go` (NEW - ~150 lines)

**Changes:**
1. Create test that:
   - Initializes storage with test database
   - Creates search service
   - Calls each tool handler directly (not via stdio)
   - Verifies response format and content
2. Test cases:
   - TestMCPSearchDocuments - verify search works
   - TestMCPGetDocument - verify ID lookup works
   - TestMCPListRecent - verify listing works
   - TestMCPGetRelated - verify reference search works

**Risk:** medium (testing MCP handlers requires understanding SDK internals)

**User decision required:** no

**Acceptance Criteria:**
- ✅ Test file in test/api/ directory
- ✅ All 4 test cases pass
- ✅ Tests use SetupTestEnvironment() pattern
- ✅ Run with: `cd test/api && go test -v -run TestMCP`

---

## Step 8: Create End-to-End Integration Test (Optional)

**Why:** Verify MCP server works end-to-end via stdio transport (not just handler testing).

**Depends:** Step 7 (API tests must pass)

**Validates:**
- stdio transport works correctly
- JSON-RPC protocol implementation
- Claude CLI can discover and call tools
- Full request/response cycle

**Files:**
- `test/api/mcp_stdio_test.go` (NEW - ~100 lines)

**Changes:**
1. Create test that:
   - Spawns quaero-mcp binary as subprocess
   - Sends JSON-RPC requests via stdin
   - Reads JSON-RPC responses from stdout
   - Verifies protocol compliance
2. Test cases:
   - TestMCPStdioInitialize - verify server handshake
   - TestMCPStdioToolsList - verify tools enumeration
   - TestMCPStdioCallSearch - verify tool invocation

**Risk:** high (subprocess testing is complex, Windows-specific issues possible)

**User decision required:** YES - Skip this step if complexity outweighs value

**Acceptance Criteria:**
- ✅ Test spawns binary correctly on Windows
- ✅ JSON-RPC handshake completes
- ✅ Tool list matches expected 4 tools
- ✅ Search tool returns valid markdown response

---

## Step 9: Add Usage Examples and Screenshots

**Why:** Help users understand MCP server value and usage patterns.

**Depends:** Step 7 or Step 8 (tests verify functionality)

**Validates:**
- Users can see real-world usage examples
- Claude CLI integration is demonstrated
- Output format is clear

**Files:**
- `docs/implement-mcp-server/usage-examples.md` (NEW)
- `docs/implement-mcp-server/screenshots/` (NEW directory - optional)

**Changes:**
1. Create usage-examples.md with:
   - Claude CLI conversation examples
   - Tool invocation examples
   - Response formatting examples
   - Common use cases (search knowledge base, find related docs, etc.)
2. Optionally add screenshots of Claude CLI using MCP tools

**Risk:** low

**User decision required:** no

**Acceptance Criteria:**
- ✅ At least 3 realistic usage examples
- ✅ Examples show both query and response
- ✅ Covers all 4 tool types
- ✅ Examples use real Quaero data patterns (Jira, Confluence, etc.)

---

## Step 10: Update CLAUDE.md with MCP Server Guidelines

**Why:** AI agents working on Quaero need to understand MCP server architecture and conventions.

**Depends:** Step 9 (all implementation and documentation complete)

**Validates:**
- CLAUDE.md documents MCP server
- Architectural patterns are clear
- Development guidelines are provided

**Files:**
- `CLAUDE.md` (add MCP server section)

**Changes:**
1. Add section under "Architecture Overview" titled "MCP Server Architecture"
2. Document:
   - Purpose: Expose search via Model Context Protocol
   - Location: cmd/quaero-mcp/
   - Transport: stdio/JSON-RPC
   - Tools: 4 search tools (brief description)
   - Testing: API tests in test/api/mcp_*.go
3. Add to "Code Conventions":
   - MCP server must remain thin wrapper (< 200 lines main.go)
   - Use existing search service (no duplication)
   - Follow stdio logging rules (minimal logging to avoid breaking protocol)

**Risk:** low

**User decision required:** no

**Acceptance Criteria:**
- ✅ CLAUDE.md has MCP server section
- ✅ Architecture diagram updated (if applicable)
- ✅ Guidelines prevent bloat and duplication
- ✅ Testing patterns documented

---

## User Decision Points

### Decision 1: End-to-End Stdio Testing (Step 8)
**Question:** Should we implement full stdio transport testing (Step 8), or is API-level handler testing (Step 7) sufficient?

**Context:**
- API tests (Step 7) verify handler logic and search integration (low complexity, high value)
- Stdio tests (Step 8) verify JSON-RPC protocol and subprocess management (high complexity, medium value)
- Stdio tests are Windows-specific and may be brittle

**Recommendation:** **Skip Step 8** initially. Implement API tests (Step 7) first. If Claude CLI integration issues arise during real usage, add stdio tests later.

**Decision:** [ ] Implement Step 8  [ ] Skip Step 8 (recommended)

---

## Constraints

### Technical Constraints
1. ✅ Use existing `internal/services/search` package (do not reinvent search logic)
2. ✅ MCP server in `cmd/quaero-mcp/` directory
3. ✅ Use `github.com/mark3labs/mcp-go` SDK (v0.43.0)
4. ✅ Keep main.go < 200 lines (thin wrapper only)
5. ✅ Follow Quaero logging conventions (arbor logger, minimal logging for stdio)
6. ✅ Tests only in `/test/api` or `/test/ui` directories (no unit tests in cmd/)

### Architectural Constraints
1. ✅ No modifications to `internal/services/search` (already complete)
2. ✅ No new database queries (use existing SearchService interface)
3. ✅ No new dependencies beyond mcp-go SDK
4. ✅ Follow Windows build patterns (PowerShell build script)

### Code Quality Constraints
1. ✅ All errors must be handled (no `_ = err` patterns)
2. ✅ Use arbor logger for all logging (no fmt.Println)
3. ✅ Follow existing code organization patterns
4. ✅ Maintain test coverage for new code

---

## Success Criteria

### Compilation & Build
1. ✅ `go build ./cmd/quaero-mcp` succeeds without errors
2. ✅ `.\scripts\build.ps1` builds both quaero.exe and quaero-mcp.exe
3. ✅ Binary size is reasonable (< 50MB with dependencies)

### Code Quality
1. ✅ main.go is under 200 lines (wrapper logic only)
2. ✅ No code duplication from internal/services/search
3. ✅ All tool handlers use consistent signature pattern
4. ✅ Type safety: no interface{} in formatter functions

### Integration
1. ✅ MCP server can connect to existing Quaero database
2. ✅ Search service initializes correctly via factory
3. ✅ All 4 tools return valid markdown responses
4. ✅ Error handling returns user-friendly messages (not panics)

### Testing
1. ✅ API tests pass in test/api/mcp_server_test.go
2. ✅ Test coverage > 80% for handler functions
3. ✅ Tests use SetupTestEnvironment() pattern
4. ✅ Tests verify response format and content accuracy

### Documentation
1. ✅ README.md includes MCP server section with quickstart
2. ✅ mcp-configuration.md provides complete setup guide
3. ✅ usage-examples.md shows real-world usage patterns
4. ✅ CLAUDE.md documents MCP server architecture and conventions

### Claude CLI Integration (Manual Verification)
1. ✅ Claude CLI discovers all 4 tools via MCP server
2. ✅ search_documents returns relevant results with full markdown
3. ✅ get_document retrieves complete document content
4. ✅ list_recent_documents shows recently updated documents
5. ✅ get_related_documents finds cross-references correctly

---

## Risk Assessment

### Low Risk Steps (1-7, 9-10)
- Dependency fixes (Step 1)
- Type corrections (Steps 2-3)
- Code organization (Step 4)
- Build integration (Step 5)
- Documentation (Steps 6, 9-10)
- API testing (Step 7)

### Medium Risk Steps
- None (Step 8 is optional and marked high risk)

### High Risk Steps
- Step 8 (stdio testing) - Complex, Windows-specific, optional

### Mitigation Strategies
1. **Compilation Issues:** Test after each step to catch errors early
2. **Type Mismatches:** Verify against interfaces.SearchService and models.Document definitions
3. **Build Script Integration:** Test build.ps1 changes on clean environment
4. **Stdio Testing:** Make Step 8 optional, rely on manual Claude CLI testing

---

## Implementation Notes

### Current MCP Implementation Status
- ✅ Skeleton exists in cmd/quaero-mcp/main.go (379 lines)
- ✅ Uses mcp-go SDK v0.43.0 (already in go.mod)
- ✅ Defines 4 tools: search_documents, get_document, list_recent_documents, get_related_documents
- ❌ Has compilation errors (missing dependencies)
- ❌ Handler signatures inconsistent (Step 2 issue)
- ❌ Type mismatches in formatters (Step 3 issue)
- ❌ Exceeds 200-line constraint (Step 4 issue)

### Search Service Interface (Already Implemented)
From `internal/interfaces/search_service.go`:
```go
type SearchService interface {
    Search(ctx context.Context, query string, opts SearchOptions) ([]*models.Document, error)
    GetByID(ctx context.Context, id string) (*models.Document, error)
    SearchByReference(ctx context.Context, reference string, opts SearchOptions) ([]*models.Document, error)
}
```

All methods needed for MCP tools already exist. **No new search functionality required.**

### MCP Tool Mapping
| MCP Tool | Search Service Method | Purpose |
|----------|----------------------|---------|
| search_documents | Search(query, opts) | Full-text search (FTS5) |
| get_document | GetByID(id) | Retrieve by document ID |
| list_recent_documents | Search("", opts) | List by updated_at DESC |
| get_related_documents | SearchByReference(ref, opts) | Find cross-references |

### Logging Guidelines for MCP Server
**Critical:** MCP uses stdio for JSON-RPC communication. Logging must not interfere with protocol.

From current implementation (lines 34-40):
```go
logger := arbor.NewLogger().WithConsoleWriter(models.WriterConfiguration{
    Type:             models.LogWriterTypeConsole,
    TimeFormat:       "15:04:05",
    TextOutput:       true,
    DisableTimestamp: false,
}).WithLevelFromString("warn") // Minimal logging
```

**Rule:** MCP server must log only warnings/errors to stderr, never to stdout (reserved for JSON-RPC).

---

## Dependencies

### Existing Dependencies (No Changes Needed)
- `github.com/mark3labs/mcp-go` v0.43.0 (MCP SDK)
- `github.com/ternarybob/arbor` (logging)
- `github.com/ternarybob/quaero/internal/common` (config)
- `github.com/ternarybob/quaero/internal/services/search` (search service)
- `github.com/ternarybob/quaero/internal/storage` (storage manager)
- `modernc.org/sqlite` (database)

### New Dependencies Required (Step 1)
- `github.com/invopop/jsonschema` (transitive via mcp-go)
- `github.com/spf13/cast` (transitive via mcp-go)
- `github.com/yosida95/uritemplate/v3` (transitive via mcp-go)

**Resolution:** Run `go get github.com/mark3labs/mcp-go/mcp@v0.43.0` to populate go.sum

---

## Testing Strategy

### Test Levels
1. **Unit Tests:** None (MCP wrapper too thin, logic is in search service which has its own tests)
2. **API Tests:** test/api/mcp_server_test.go (Step 7 - REQUIRED)
3. **Stdio Tests:** test/api/mcp_stdio_test.go (Step 8 - OPTIONAL)
4. **Manual Tests:** Claude CLI integration (final verification)

### Test Data
Use existing test data from `test/common/test_data.go`:
- Sample documents with Jira, Confluence, GitHub source types
- Documents with cross-references for SearchByReference testing
- Recent documents for list_recent_documents testing

### Test Execution
```powershell
# API tests (Step 7)
cd test/api
go test -v -run TestMCP

# All API tests including MCP
cd test/api
go test -v ./...

# Stdio tests (Step 8 - optional)
cd test/api
go test -v -run TestMCPStdio
```

---

## Rollback Plan

If implementation fails or issues arise:

1. **Step 1-3 (Fixes):** Revert cmd/quaero-mcp/main.go to working state
2. **Step 4 (Refactor):** Keep single main.go file if extraction causes issues
3. **Step 5 (Build):** Remove MCP build from build.ps1, build manually
4. **Step 6-10 (Docs/Tests):** Delete new files, no impact on core system

**Safety:** MCP server is entirely additive. Failure does not affect main Quaero application.

---

## Post-Implementation Validation

After completing all steps, verify:

1. ✅ Run build script: `.\scripts\build.ps1` → both binaries created
2. ✅ Run API tests: `cd test/api && go test -v -run TestMCP` → all pass
3. ✅ Start MCP server: `.\bin\quaero-mcp.exe` → initializes without errors
4. ✅ Configure Claude CLI with MCP server (see docs/implement-mcp-server/mcp-configuration.md)
5. ✅ Ask Claude: "Search my knowledge base for 'authentication'" → returns results
6. ✅ Ask Claude: "List recent documents" → returns recent docs
7. ✅ Verify markdown formatting in Claude's responses

If all validation passes → Implementation complete ✅
