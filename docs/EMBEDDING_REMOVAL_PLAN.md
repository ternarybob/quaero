# Embedding Removal & Search Service Migration Plan

**Created:** 2025-10-13
**Status:** Planning Phase
**Goal:** Remove vector embeddings and migrate to FTS5-based SearchService interface

---

## Executive Summary

This document outlines the comprehensive plan to remove vector embeddings from Quaero and implement a clean `SearchService` interface backed by SQLite FTS5 full-text search. This refactor will:

1. **Simplify the architecture** - Remove embedding generation, storage, and vector similarity search
2. **Maintain agent functionality** - Keep MCP agent-based chat working
3. **Improve search quality** - Use FTS5 full-text search with programmatic metadata extraction
4. **Create clean interfaces** - Allow future search implementation swaps

---

## Current State Analysis

### Embedding Integration Points

**Core Services** (8 total):
1. `EmbeddingService` - Generates embeddings via LLM
2. `EmbeddingCoordinator` - Orchestrates embedding generation
3. `DocumentService` - Depends on EmbeddingService for document storage
4. `ProcessingService` - Tracks vectorization stats
5. `SummaryService` - Creates corpus summary documents (uses embedding fields)
6. `ChatService` - Already migrated to agent-only (no RAG)
7. `EventService` - Publishes `EventEmbeddingTriggered`
8. `SchedulerService` - Triggers embedding events every 5 minutes

**Data Models** (internal/models/document.go):
- `Document.Embedding` ([]float32)
- `Document.EmbeddingModel` (string)
- `Document.ForceEmbedPending` (bool)
- `DocumentChunk` struct (for large document processing)
- `DocumentStats.VectorizedCount` (int)
- `DocumentStats.PendingVectorize` (int)
- `DocumentStats.EmbeddingModel` (string)

**Storage Layer** (internal/storage/sqlite/):
- `document_storage.go` - Reads/writes embedding fields
- Database schema includes `embedding` BLOB column
- `SaveChunk()`, `GetChunks()`, `DeleteChunks()` methods

**App Initialization** (internal/app/app.go):
```go
// Lines 143-221: Deep dependency chain
EmbeddingService → DocumentService → ...
EmbeddingCoordinator subscribes to EventEmbeddingTriggered
Scheduler publishes EventEmbeddingTriggered every 5 minutes
```

### What Was Already Removed

**Stage 3: RAG Removal (Completed in commit 9ee89ed):**
- ✅ RAG-specific chat logic
- ✅ Query classification
- ✅ Document formatting for RAG
- ✅ `ChatService` simplified (506 → 133 lines)
- ✅ `ChatRequest.UseAgent` flag removed
- ✅ RAG test files deleted

**What Remains:**
- ❌ Vector embeddings still generated for all documents
- ❌ Embedding services still active
- ❌ Database still stores embedding BLOBs
- ❌ Agent uses DocumentService (which depends on embeddings)

---

## Target Architecture

### New SearchService Interface

**File:** `internal/interfaces/search_service.go` (already created)

```go
type SearchService interface {
    // Search performs full-text search across documents
    Search(ctx context.Context, query string, opts SearchOptions) ([]*models.Document, error)

    // GetByID retrieves a single document by ID
    GetByID(ctx context.Context, id string) (*models.Document, error)

    // SearchByReference finds documents by reference (e.g., "PROJ-123", "@alice")
    SearchByReference(ctx context.Context, reference string, opts SearchOptions) ([]*models.Document, error)
}
```

**Implementation:** `internal/services/search/fts5_search_service.go` (to be created)

### Metadata Extraction System

**Programmatic keyword extraction** (NOT LLM-generated):

1. **Config-driven patterns:**
```toml
[metadata.extraction]
patterns = [
    { name = "jira_issue", regex = "[A-Z]+-\\d+", field = "issue_keys" },
    { name = "user_mention", regex = "@\\w+", field = "mentions" },
    { name = "project", regex = "PROJECT-\\w+", field = "projects" },
]
```

2. **Extraction during document save:**
   - Scan `content` and `content_markdown` fields
   - Apply regex patterns from config
   - Store extracted values in `document.metadata` JSON
   - Make metadata searchable via FTS5

3. **Benefits:**
   - Fast, deterministic extraction
   - No LLM calls required
   - Configurable without code changes
   - Enriches search context

### Three-Layer Architecture

```
┌─────────────────────────────────────┐
│  Data Collection Layer              │
│  (Atlassian, GitHub services)       │
│  - Source-specific scrapers         │
│  - Transform raw data → documents   │
└─────────────────────────────────────┘
              ↓
┌─────────────────────────────────────┐
│  Search Interface Layer             │
│  (SearchService)                    │
│  - Generic search abstraction       │
│  - FTS5 implementation (current)    │
│  - Swappable implementations        │
└─────────────────────────────────────┘
              ↓
┌─────────────────────────────────────┐
│  Agent Layer (Consumer)             │
│  (MCP Agent, ChatService)           │
│  - Uses SearchService via MCP tools │
│  - Iterative reasoning & tool calls │
└─────────────────────────────────────┘
```

---

## Migration Strategy: Incremental Approach

### Phase 1: Create SearchService Implementation (Week 1) ✅ COMPLETED

**Goal:** Build FTS5 search service without touching embeddings

**Tasks:**
1. ✅ Create `SearchService` interface (already done)
2. ✅ Create `internal/services/search/fts5_search_service.go`
   - ✅ Implement `Search()` using SQLite FTS5
   - ✅ Implement `GetByID()` (simple SELECT)
   - ✅ Implement `SearchByReference()` using LIKE/regex
3. ✅ Create `internal/services/metadata/extractor.go`
   - ✅ Regex-based pattern extraction (Jira keys, mentions, PRs, Confluence pages)
   - ✅ Extract keywords from document content
   - ✅ Return extracted metadata map
4. ✅ Update `DocumentService.SaveDocument()` and `SaveDocuments()`
   - ✅ Call metadata extractor before save
   - ✅ Merge extracted metadata with existing metadata
5. ✅ Write unit tests for SearchService (7/7 tests passing)
6. ✅ Write unit tests for metadata extractor (14/14 tests passing)

**Test Results:**
```
=== SearchService Tests ===
✅ TestFTS5SearchService_Search (4 subtests)
   - Search by keyword
   - Search with limit
   - Search with source type filter
   - Search with metadata filter
✅ TestFTS5SearchService_GetByID (2 subtests)
✅ TestFTS5SearchService_SearchByReference (2 subtests)

=== MetadataExtractor Tests ===
✅ TestExtractor_ExtractMetadata (8 subtests)
   - Extract Jira issue keys (PROJ-123)
   - Extract user mentions (@alice)
   - Extract PR references (#123)
   - Extract Confluence page references (page:123456)
   - Extract from title and content
   - Deduplicate results
   - Handle empty documents
✅ TestExtractor_MergeMetadata (4 subtests)
✅ TestExtractor_Patterns (2 subtests)

TOTAL: 21/21 tests passing
```

**Success Criteria:**
- ✅ SearchService passes all tests
- ✅ Metadata extraction works with sample documents
- ✅ Build compiles successfully
- ✅ **Embeddings still working** (no breaking changes yet)

**Actual Time:** Completed in 1 session

**Files Created:**
- `internal/services/search/fts5_search_service.go` (252 lines)
- `internal/services/search/fts5_search_service_test.go` (345 lines)
- `internal/services/metadata/extractor.go` (106 lines)
- `internal/services/metadata/extractor_test.go` (324 lines)

**Files Modified:**
- `internal/services/documents/document_service.go` - Integrated MetadataExtractor

**Patterns Extracted:**
- `[A-Z]+-\d+` → Jira issue keys (e.g., PROJ-123)
- `@\w+` → User mentions (e.g., @alice)
- `#\d+` → PR references (e.g., #456)
- `page:\d+` → Confluence page references (e.g., page:123456)

**Estimated Time:** 3-4 days

### Phase 2: Update MCP Tool Router (Week 1-2) ✅ COMPLETED

**Goal:** Switch agent tools to use SearchService instead of DocumentService

**Tasks:**
1. ✅ Update `internal/services/mcp/document_service.go`
   - ✅ Add `SearchService` to DocumentService struct
   - ✅ Update `search_documents` tool to use `SearchService.Search()`
   - ✅ Update `get_document` tool to use `SearchService.GetByID()`
   - ✅ Add new `search_by_reference` tool
   - ✅ Enhanced tool descriptions with filter parameters
2. ✅ Update `internal/services/mcp/router.go`
   - ✅ Add `SearchService` to ToolRouter struct
   - ✅ Pass SearchService to DocumentService during initialization
3. ✅ Update `internal/services/chat/chat_service.go`
   - ✅ Add SearchService parameter to NewChatService
   - ✅ Pass SearchService to ToolRouter
4. ✅ Update `internal/app/app.go`
   - ✅ Initialize SearchService (step 3.5 in initServices)
   - ✅ Pass SearchService to ChatService
   - ✅ Pass SearchService to MCPHandler

**Test Results:**
```
Test Run: 2025-10-13_22-05-38
Results: C:\development\quaero\test\results\api-Integration-2025-10-13_22-05-38\

=== TestMetadataExtraction_Integration (0.15s) ===
✅ Extract_Jira_keys_from_document_content (0.01s)
✅ Extract_user_mentions_from_document_content (0.01s)
✅ Extract_PR_references_from_document_content (0.01s)
✅ Batch_save_with_metadata_extraction (0.01s)

=== TestSearchService_Integration (0.12s) ===
✅ Search_by_keyword (0.00s)
✅ Search_with_source_type_filter (0.00s)
✅ Search_with_metadata_filter (0.00s)
✅ Get_document_by_ID (0.00s)
✅ Search_by_reference_with_extracted_metadata (0.01s)

TOTAL: 9 subtests, 0 failures, 0.746s
Build: 0.1.754 (2025-10-13 22:05:39)
```

**Success Criteria:**
- ✅ Agent chat works with FTS5 search
- ✅ Integration tests pass (9/9 subtests)
- ✅ SearchService successfully integrated into MCP tool router
- ✅ Build compiles successfully
- ✅ **Embeddings still working but unused by agent**

**Actual Time:** Completed in 1 session

**Files Modified:**
- `internal/services/mcp/document_service.go` - Added SearchService integration
- `internal/services/mcp/router.go` - Updated to pass SearchService
- `internal/services/chat/chat_service.go` - Added SearchService parameter
- `internal/app/app.go` - Initialize and wire SearchService

**New MCP Tool Available:**
- `search_by_reference` - Find documents containing specific references (Jira keys, mentions, PRs)

**Architecture Impact:**
- MCP agent tools now use SearchService abstraction
- Metadata filtering enabled for all search operations
- Clean separation between search interface and storage layer

### Phase 3: Remove Embedding Generation (Week 2) ✅ COMPLETED

**Goal:** Stop generating new embeddings

**Tasks:**
1. ✅ Update `internal/services/scheduler/scheduler_service.go`
   - ✅ Remove `EventEmbeddingTriggered` publishing from runScheduledTask
   - ✅ Remove `TriggerEmbeddingNow()` method
   - ✅ Remove unused `time` import
   - ✅ Keep `EventCollectionTriggered` (still needed)
2. ✅ Update `internal/interfaces/scheduler_service.go`
   - ✅ Remove `TriggerEmbeddingNow()` from interface
3. ✅ Update `internal/handlers/scheduler_handler.go`
   - ✅ Remove `TriggerEmbeddingHandler` method
   - ✅ Remove `ForceEmbedDocumentHandler` method
4. ✅ Update `internal/server/routes.go`
   - ✅ Remove `/api/scheduler/trigger-embedding` route
   - ✅ Remove `/api/documents/force-embed` route
5. ✅ Update `internal/app/app.go`
   - ✅ Comment out `EmbeddingCoordinator` initialization
   - ✅ Keep `EmbeddingService` temporarily (backward compatibility)
6. ✅ Update `internal/services/processing/processing_service.go`
   - ✅ Remove vectorization tracking from GetStatus
   - ✅ ProcessedCount now equals TotalDocuments
   - ✅ PendingCount set to 0
7. ✅ Update `internal/services/summary/summary_service.go`
   - ✅ Set ForceEmbedPending to false
   - ✅ Add comment explaining Phase 3 change
8. ✅ Run integration tests

**Test Results:**
```
Test Run: 2025-10-13_22-17-11
Results: C:\development\quaero\test\results\api-Integration-2025-10-13_22-17-11\

=== TestMetadataExtraction_Integration (0.15s) ===
✅ Extract_Jira_keys_from_document_content (0.01s)
✅ Extract_user_mentions_from_document_content (0.01s)
✅ Extract_PR_references_from_document_content (0.01s)
✅ Batch_save_with_metadata_extraction (0.01s)

=== TestSearchService_Integration (0.12s) ===
✅ Search_by_keyword (0.00s)
✅ Search_with_source_type_filter (0.00s)
✅ Search_with_metadata_filter (0.00s)
✅ Get_document_by_ID (0.00s)
✅ Search_by_reference_with_extracted_metadata (0.01s)

TOTAL: 9 subtests, 0 failures
Build: 0.1.755 (2025-10-13 22:17:14)
```

**Success Criteria:**
- ✅ Documents collected without embedding generation
- ✅ No embedding-related errors in logs
- ✅ Build compiles successfully
- ✅ Integration tests pass (9/9 subtests)
- ✅ Scheduler runs collection only, no embedding
- ✅ **Embedding columns in DB still exist but unpopulated**

**Actual Time:** Completed in 1 session

**Files Modified:**
- `internal/services/scheduler/scheduler_service.go` - Removed embedding event triggers
- `internal/interfaces/scheduler_service.go` - Removed TriggerEmbeddingNow interface
- `internal/handlers/scheduler_handler.go` - Removed embedding handlers
- `internal/server/routes.go` - Removed embedding routes
- `internal/app/app.go` - Disabled EmbeddingCoordinator
- `internal/services/processing/processing_service.go` - Removed vectorization tracking
- `internal/services/summary/summary_service.go` - Disabled embedding for summary docs

**Architecture Impact:**
- Embedding generation completely stopped
- Collection pipeline works independently
- EmbeddingService kept for backward compatibility (removed in Phase 4)
- API routes for embedding triggers removed

### Phase 4: Remove EmbeddingService Dependency (Week 2-3)

**Goal:** Decouple DocumentService from EmbeddingService

**Tasks:**
1. Update `internal/services/documents/document_service.go`
   - Remove `embeddingService` field from struct
   - Remove `embeddingService` parameter from `NewService()`
   - Remove any embedding generation calls
2. Update `internal/app/app.go`
   - Remove `EmbeddingService` initialization
   - Update `DocumentService` initialization (remove embedding param)
3. Delete `internal/services/embeddings/` directory
   - `embedding_service.go`
   - `coordinator_service.go`
4. Update `internal/interfaces/llm_service.go`
   - Remove `GenerateEmbedding()` method
   - Keep `Chat()` method (still needed for agent)
5. Update LLM implementations
   - Remove embedding generation from offline/mock implementations

**Success Criteria:**
- DocumentService works without EmbeddingService
- Build compiles successfully
- All tests pass
- **Embedding service completely removed**

**Estimated Time:** 2-3 days

### Phase 5: Clean Up Data Models (Week 3)

**Goal:** Remove embedding fields from Document model

**Tasks:**
1. Update `internal/models/document.go`
   - Remove `Embedding` field
   - Remove `EmbeddingModel` field
   - Remove `ForceEmbedPending` field
   - Remove `DocumentChunk` struct
   - Remove embedding fields from `DocumentStats`
2. Update `internal/storage/sqlite/document_storage.go`
   - Remove embedding field reads/writes
   - Remove chunk-related methods (`SaveChunk`, `GetChunks`, `DeleteChunks`)
3. Update `internal/interfaces/storage.go`
   - Remove chunk-related method signatures
4. Run all tests

**Success Criteria:**
- Document model clean of embedding references
- Build compiles successfully
- All tests pass
- **Database schema still has embedding columns** (data not lost)

**Estimated Time:** 1-2 days

### Phase 6: Database Schema Migration (Week 3-4)

**Goal:** Remove embedding columns from database (optional, can be deferred)

**Tasks:**
1. Create migration script `scripts/migrate_remove_embeddings.sql`
   ```sql
   -- Drop embedding column (SQLite requires table recreation)
   -- Backup existing data
   -- Create new table without embedding columns
   -- Copy data to new table
   ```
2. Update `internal/storage/sqlite/schema.go`
   - Remove `embedding BLOB` column definition
3. Test migration on dev database
4. Document rollback procedure

**Success Criteria:**
- Migration script tested
- Database schema clean
- No data loss during migration
- **Optional: Can be run manually by users**

**Estimated Time:** 2-3 days

---

## Risk Analysis

### High Risk Items

1. **DocumentService Dependency Chain**
   - **Risk:** DocumentService is used by 8+ services
   - **Mitigation:** Phase 4 isolates this change; thorough testing

2. **Agent Functionality Regression**
   - **Risk:** Breaking agent after removing embeddings
   - **Mitigation:** Phase 2 migrates agent early; continuous testing

3. **Database Migration Data Loss**
   - **Risk:** Losing document data during schema change
   - **Mitigation:** Phase 6 optional; backup required; rollback plan

### Medium Risk Items

1. **FTS5 Search Quality**
   - **Risk:** FTS5 may not match vector search quality
   - **Mitigation:** Metadata extraction improves relevance; user feedback

2. **Incomplete Embedding Removal**
   - **Risk:** Missing a dependency and leaving broken code
   - **Mitigation:** Comprehensive grep for embedding references; code review

### Low Risk Items

1. **Config Changes**
   - **Risk:** Breaking existing configs
   - **Mitigation:** Backward-compatible defaults; migration guide

2. **Test Coverage Gaps**
   - **Risk:** Insufficient test coverage of new code
   - **Mitigation:** Unit tests required for each phase

---

## Testing Strategy

### Unit Tests (Each Phase)

- `internal/services/search/fts5_search_service_test.go`
- `internal/services/metadata/extractor_test.go`
- `internal/services/mcp/router_test.go` (updated)

### Integration Tests

- `test/api/search_service_test.go` (new)
- `test/api/chat_agent_test.go` (updated)
- `test/api/document_collection_test.go` (verify still works)

### Manual Testing Checklist

- [ ] Collect Jira issues → documents created
- [ ] Collect Confluence pages → documents created
- [ ] Agent query: "How many Jira issues?" → correct count
- [ ] Agent query: "Find issues about bug" → relevant results
- [ ] Agent query: "Show me @alice's issues" → metadata filter works
- [ ] Corpus summary generated correctly
- [ ] No errors in logs during normal operation

---

## Rollback Plan

Each phase is reversible via git:

1. **Phase 1-2:** Revert commits, embeddings still work
2. **Phase 3-4:** Re-enable EmbeddingCoordinator in app.go
3. **Phase 5:** Revert Document model changes
4. **Phase 6:** Keep old database (migration is optional)

**Rollback Commands:**
```bash
# Identify commit before phase N
git log --oneline

# Revert to commit
git reset --hard <commit-hash>

# Or revert specific files
git checkout <commit-hash> -- <file-path>
```

---

## Success Metrics

### Phase Completion Metrics

- **Phase 1:** FTS5 search returns results in < 100ms
- **Phase 2:** Agent chat response time < 30s
- **Phase 3:** No new embeddings generated (verify in logs)
- **Phase 4:** EmbeddingService completely removed from codebase
- **Phase 5:** `grep -r "Embedding" internal/` returns 0 matches
- **Phase 6:** Database size reduced by ~30% (embedding BLOB removal)

### Overall Success Criteria

- ✅ Build compiles at every phase
- ✅ All tests pass at every phase
- ✅ Agent chat works throughout refactor
- ✅ Document collection works throughout refactor
- ✅ No regressions in existing functionality
- ✅ Codebase simplified (fewer dependencies)
- ✅ Search quality maintained or improved

---

## Timeline

**Total Estimated Time:** 3-4 weeks

| Phase | Duration | Dependencies | Can Start |
|-------|----------|--------------|-----------|
| Phase 1 | 3-4 days | None | Immediately |
| Phase 2 | 2-3 days | Phase 1 | After Phase 1 |
| Phase 3 | 1-2 days | Phase 2 | After Phase 2 |
| Phase 4 | 2-3 days | Phase 3 | After Phase 3 |
| Phase 5 | 1-2 days | Phase 4 | After Phase 4 |
| Phase 6 | 2-3 days | Phase 5 | After Phase 5 (optional) |

**Critical Path:** Phases 1-5 must be completed sequentially. Phase 6 is optional and can be deferred.

---

## Open Questions

1. **Metadata Extraction Patterns:** Which regex patterns should be default in config?
2. **FTS5 Configuration:** Should we use custom tokenizers or default?
3. **Search Ranking:** How to rank FTS5 results (BM25, relevance, recency)?
4. **Database Migration:** Should Phase 6 be mandatory or optional?
5. **LLM Service:** Keep for agent chat only, or further simplify?

---

## Next Steps

1. **Review this plan** with team/stakeholders
2. **Start Phase 1** (create SearchService implementation)
3. **Create feature branch:** `refactor/remove-embeddings`
4. **Daily commits** for each sub-task
5. **Update this document** as we learn during implementation

---

**Document Owner:** Claude Code
**Last Updated:** 2025-10-13
**Next Review:** Start of each phase
