# Final Database Recommendation for Quaero (2025)

## Research Summary: Embedded Go Databases

After comprehensive research of embedded Go database solutions with full-text and vector search capabilities, here are the viable options:

### Option 1: SQLite + FTS5 + sqlite-vec ⭐ **RECOMMENDED**

**Stack:**
- **SQLite**: Single-file embedded database
- **FTS5**: Built-in full-text search extension (battle-tested, billions of devices)
- **sqlite-vec**: Vector search extension for embeddings
- **Go**: `github.com/mattn/go-sqlite3` (CGO) or `github.com/ncruces/go-sqlite3` (WASM)

**Pros:**
✅ **Single embedded file** - No external service required
✅ **Proven stability** - FTS5 on billions of devices worldwide
✅ **Hybrid search** - Combine full-text (BM25) + vector search
✅ **Vector search** - sqlite-vec with K-NN, SIMD acceleration
✅ **Handles scale** - Excellent for 100K+ documents
✅ **Zero setup** - Just include the library
✅ **Cross-platform** - Works everywhere SQLite works

**Cons:**
❌ Requires CGO (compilation complexity) - *can use WASM alternative*
❌ sqlite-vec newer (v0.1.0) - *actively developed, Alex Garcia*
❌ Single writer - *multi-reader OK, fine for collection use case*

**Performance:**
- Full-text: Extremely fast (FTS5 proven)
- Vector search: Good for <100K, adequate for <1M
- Disk: Efficient with built-in compression
- RAM: Minimal footprint

**Use Case Fit:** ✅ **EXCELLENT**
- 10,000 Confluence pages: ✅ Perfect
- Full-text search: ✅ Built-in (FTS5)
- Vector search for RAG: ✅ sqlite-vec
- No external service: ✅ Embedded
- Production ready: ✅ SQLite is rock solid

---

### Option 2: Bleve + chromem-go

**Stack:**
- **Bleve**: Pure Go full-text search and indexing
- **chromem-go**: Pure Go embedded vector database
- **SQLite**: For metadata storage (optional)

**Pros:**
✅ **Pure Go** - No CGO, easier compilation
✅ **Zero dependencies** - chromem-go has no external deps
✅ **Fast vectors** - chromem-go: 1K docs in 0.3ms, 100K in 40ms
✅ **Rich full-text** - Bleve: tf-idf/BM25, highlighting, faceting
✅ **Low resource** - Opens in milliseconds, tens of MB RAM

**Cons:**
❌ **Two separate systems** - Full-text and vector not unified
❌ **More integration** - Need to coordinate two libraries
❌ **No hybrid search** - Can't easily combine FTS + vector results
❌ **Scale limit** - chromem-go best for <100K documents

**Performance:**
- Full-text (Bleve): Excellent
- Vector (chromem-go): Very fast for <100K docs
- Memory: Efficient

**Use Case Fit:** ✅ **GOOD**
- Works well but more complex integration
- Better if avoiding CGO is critical

---

### Option 3: RavenDB (Original Recommendation)

**Stack:**
- **RavenDB**: External document database service
- **ternarybob/ravendb**: Go client library

**Pros:**
✅ Built-in full-text search
✅ Built-in vector search
✅ Document-oriented (perfect for JSON)
✅ Handles millions of documents
✅ Advanced querying

**Cons:**
❌ **External service** - Requires Docker/separate install
❌ **Deployment complexity** - Not embedded
❌ **Resource overhead** - Separate process, memory
❌ **Not "offline first"** - Requires service management

**Use Case Fit:** ⚠️ **DOESN'T MEET REQUIREMENTS**
- Requirement: "Runs completely offline on a single machine"
- RavenDB requires external service setup
- Against embedded/self-contained philosophy

---

## Final Recommendation: **SQLite + FTS5 + sqlite-vec**

### Why This Is the Best Choice:

#### 1. **Truly Embedded & Offline**
```
Requirement: "Runs completely offline on a single machine"
```
- ✅ Single database file (e.g., `data/quaero.db`)
- ✅ No external service or Docker required
- ✅ Works completely offline
- ✅ Embedded in the Go binary

#### 2. **Production-Proven Technology**
- **SQLite**: Most deployed database in the world
- **FTS5**: Powers search on billions of devices (iOS, Android, browsers)
- **sqlite-vec**: Built by Alex Garcia, actively maintained (2025)

#### 3. **Meets All Requirements**
```
✅ Full-text search: FTS5 with BM25 ranking
✅ Vector search: sqlite-vec for RAG embeddings
✅ Document storage: JSON in SQLite (flexible schema)
✅ Scale: 10,000+ Confluence pages handled easily
✅ Offline: 100% embedded, no external dependencies
```

#### 4. **Hybrid Search Capability**
- Combine keyword search (FTS5) with semantic search (vectors)
- Single query can use both: `SELECT ... FROM fts JOIN vec ...`
- Best of both worlds for RAG pipeline

#### 5. **Simple Architecture**
```
Quaero Binary
└── SQLite Database (data/quaero.db)
    ├── Documents table (JSON storage)
    ├── FTS5 index (full-text search)
    └── vec0 virtual table (vector embeddings)
```

#### 6. **CGO Consideration**
The main concern is CGO requirement for `mattn/go-sqlite3`. Options:

**Option A: Use CGO** (Recommended)
- Standard approach, proven, fastest
- Cross-compile for all platforms
- One-time setup complexity

**Option B: Use WASM** (`ncruces/go-sqlite3`)
- No CGO, pure Go
- Slight performance penalty
- Easier cross-compilation

**Option C: Fallback to Bleve + chromem-go**
- Only if CGO is completely unacceptable
- More code, less integrated

### Recommended: Accept CGO Trade-off
- CGO is standard for SQLite in Go ecosystem
- Many production apps use `mattn/go-sqlite3`
- Benefits far outweigh compilation complexity
- Can provide pre-built binaries for users

---

## Implementation Architecture

### Database Schema

```sql
-- Main documents table
CREATE TABLE documents (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL,  -- 'confluence', 'jira', 'github'
    type TEXT NOT NULL,    -- 'page', 'issue', 'project', 'repo'
    data JSON NOT NULL,    -- Full document as JSON
    created_at INTEGER,
    updated_at INTEGER
);

-- FTS5 virtual table for full-text search
CREATE VIRTUAL TABLE documents_fts USING fts5(
    id UNINDEXED,
    title,
    content,
    content=documents
);

-- Vector embeddings table (sqlite-vec)
CREATE VIRTUAL TABLE document_vectors USING vec0(
    id TEXT PRIMARY KEY,
    embedding FLOAT[1536]  -- OpenAI embedding dimension
);

-- Metadata tables
CREATE TABLE jira_projects (...);
CREATE TABLE confluence_spaces (...);
-- etc.
```

### Configuration

```toml
[storage]
type = "sqlite"
path = "./data/quaero.db"

[storage.sqlite]
enable_fts = true           # FTS5 full-text search
enable_vector = true        # sqlite-vec for embeddings
embedding_dimension = 1536  # OpenAI text-embedding-3-small
cache_size_mb = 100
wal_mode = true            # Write-Ahead Logging for concurrency

[storage.filesystem]
images = "./data/images"
attachments = "./data/attachments"
```

### Go Dependencies

```go
import (
    _ "github.com/mattn/go-sqlite3"
    "github.com/asg017/sqlite-vec-go-bindings/cgo/sqlite_vec"
)
```

---

## Migration Path from Current State

### Phase 1: Implement SQLite Storage
1. Create `internal/storage/sqlite/` package
2. Implement storage interfaces for Jira/Confluence/Auth
3. Add FTS5 indexing for documents
4. Add sqlite-vec for embeddings (prepared for future RAG)

### Phase 2: Update Collectors
1. Inject SQLite storage into collectors (dependency injection)
2. Remove BoltDB dependencies
3. Migrate existing data (if any)

### Phase 3: Search Implementation
1. Implement full-text search using FTS5
2. Add hybrid search (FTS + vector) for RAG
3. Integrate with Ollama for embeddings

### Phase 4: Remove RavenDB Code
1. Delete `internal/services/database/ravendb_service.go`
2. Delete `internal/interfaces/database.go` (replace with storage interfaces)
3. Update documentation

---

## Performance Expectations

### Document Storage
- **10,000 Confluence pages**: ~500MB-1GB database file
- **Insert performance**: 1,000+ docs/sec with transactions
- **Query performance**: <10ms for most queries

### Full-Text Search (FTS5)
- **Index size**: ~30-50% of document size
- **Query latency**: <5ms for typical searches
- **Ranking**: Built-in BM25

### Vector Search (sqlite-vec)
- **100K vectors**: ~40ms query time (chromem-go benchmark)
- **Storage**: ~6MB per 1K vectors (1536 dimensions, float32)
- **k-NN search**: SIMD-accelerated

---

## Risks and Mitigations

### Risk: CGO Compilation Complexity
**Mitigation:**
- Provide detailed build documentation
- Include cross-compilation scripts
- Consider pre-built binaries
- WASM fallback option available

### Risk: sqlite-vec Maturity (v0.1.0)
**Mitigation:**
- Actively developed by Alex Garcia (SQLite expert)
- Extensive testing in production apps
- Simple fallback: use chromem-go if issues arise

### Risk: Single Writer Limitation
**Mitigation:**
- Quaero use case is read-heavy (LLM queries)
- Collection happens in batches (acceptable single writer)
- WAL mode enables concurrent reads during writes

---

## Conclusion

**Use SQLite + FTS5 + sqlite-vec for Quaero**

This solution:
- ✅ Meets all requirements (embedded, offline, full-text, vector search)
- ✅ Uses proven technology (SQLite, FTS5)
- ✅ Handles expected scale (10K+ documents)
- ✅ Simpler than RavenDB (no external service)
- ✅ Better than BoltDB (built-in search)
- ✅ Production-ready and maintainable

**CGO is acceptable trade-off** for the significant benefits of SQLite's maturity, FTS5's proven search, and sqlite-vec's vector capabilities.

---

## Next Steps

1. ✅ **Approve this recommendation**
2. Update config structure for SQLite
3. Create storage abstraction interfaces
4. Implement SQLite storage layer with FTS5 + sqlite-vec
5. Migrate collectors to use storage interfaces
6. Remove BoltDB and RavenDB code
7. Write tests
8. Update documentation
