# Database Analysis for Quaero

## Business Requirements Analysis

### Data Volume Estimates

**Confluence:**
- Large organizations: 1,000-10,000+ pages
- Each page: ~50KB HTML + metadata
- Images/attachments: 1-10MB per page
- **Total**: 50GB-500GB+ (with attachments)

**Jira:**
- Projects: 10-100
- Issues per project: 1,000-50,000
- Each issue: ~10KB + comments
- **Total**: 10MB-500MB (metadata only)

**GitHub:**
- Repositories: 10-100
- README + wiki pages
- **Total**: 10MB-100MB

### Access Patterns

1. **Write Operations:**
   - Batch inserts during collection (1000s of documents)
   - Periodic updates (daily/weekly)
   - Low write concurrency

2. **Read Operations:**
   - Full-text search across all documents
   - Vector similarity search (RAG pipeline)
   - Individual document retrieval
   - High read concurrency (LLM queries)

3. **Query Requirements:**
   - Full-text search with ranking
   - Vector embeddings storage and search
   - Filtering by source/type/date
   - Pagination for large result sets

## Database Options Comparison

### 1. BoltDB (bbolt)

**Pros:**
✅ Embedded (no external service)
✅ Single file database
✅ ACID transactions
✅ Fast key-value operations
✅ Zero dependencies
✅ Production-proven (etcd, consul use it)

**Cons:**
❌ No built-in full-text search
❌ No vector search support
❌ Manual indexing required
❌ Single file can grow very large (50GB+)
❌ Read-only or single writer concurrency
❌ Complex querying requires loading data into memory

**File Size:**
- 1,000 Confluence pages: ~50MB-100MB
- 10,000 pages: 500MB-1GB
- With attachments: 50GB+ (NOT recommended)

**Use Case Fit:** ⚠️ MARGINAL
- Good for metadata storage
- Poor for large document collections
- Requires external search solution (Bleve, etc.)

### 2. RavenDB

**Pros:**
✅ Document-oriented (perfect for JSON documents)
✅ Built-in full-text search
✅ **Built-in vector search** (critical for RAG)
✅ Advanced querying (LINQ-like)
✅ Automatic indexing
✅ Excellent for large datasets
✅ Multi-reader/writer concurrency

**Cons:**
❌ Requires external service (docker/binary)
❌ More complex deployment
❌ Higher memory usage
❌ Learning curve for operations

**Use Case Fit:** ✅ EXCELLENT
- Designed for document collections
- Native vector search for RAG
- Handles 10,000+ documents easily
- Built for full-text search

### 3. SQLite + FTS5 (Alternative Option)

**Pros:**
✅ Embedded (single file)
✅ Full-text search (FTS5 extension)
✅ SQL querying
✅ Wide adoption
✅ Excellent performance for 100K+ records
✅ Multi-reader concurrency

**Cons:**
❌ No native vector search
❌ Relational model (requires schema design)
❌ Manual index management
❌ Single writer concurrency

**Use Case Fit:** ✅ GOOD
- Better than BoltDB for full-text search
- Can handle large datasets
- Requires vector search add-on

### 4. BadgerDB (Alternative Option)

**Pros:**
✅ Embedded key-value store
✅ Better concurrency than BoltDB
✅ LSM tree (better write performance)
✅ Smaller file sizes with compression

**Cons:**
❌ No full-text search
❌ No vector search
❌ Requires external search solution

**Use Case Fit:** ⚠️ MARGINAL
- Similar to BoltDB limitations
- Better performance but same search issues

## Recommendation

### **Use RavenDB - Here's Why:**

#### 1. Vector Search is Critical
```
Requirement: "Provides natural language query interface using local LLMs"
```
- RAG pipeline REQUIRES vector embeddings
- RavenDB has **native vector search**
- BoltDB/SQLite require external solutions (adds complexity)

#### 2. Full-Text Search is Essential
```
Requirement: "Processes and stores content with full-text search"
```
- RavenDB: Built-in, automatic indexing
- BoltDB: Requires Bleve or similar (more code)
- SQLite: FTS5 works but less flexible

#### 3. Document Model Fits Perfectly
- Confluence pages = JSON documents ✅
- Jira issues = JSON documents ✅
- No complex JOIN operations needed
- Schema-less flexibility

#### 4. Scale Requirements
```
"1,000-10,000+ Confluence pages"
```
- RavenDB: Designed for millions of documents
- BoltDB: Struggles with large datasets
- SQLite: Good, but not document-oriented

#### 5. Deployment is Acceptable
```
"Runs completely offline on a single machine"
```
- RavenDB can run locally via Docker or binary
- One-time setup, then runs like any service
- No internet required after installation

## Architecture Decision

### Single Database for All Layers

**Previous (Microservices):**
```
Auth Service → BoltDB (auth.db)
Jira Service → BoltDB (jira.db)
Confluence Service → BoltDB (confluence.db)
```

**New (Monolithic with Collections):**
```
Quaero Service → RavenDB (quaero database)
  ├── Collections:
  │   ├── AuthCredentials
  │   ├── JiraProjects
  │   ├── JiraIssues
  │   ├── ConfluenceSpaces
  │   ├── ConfluencePages
  │   ├── GitHubRepos
  │   └── DocumentEmbeddings (for RAG)
```

**Benefits:**
✅ Single connection pool
✅ Cross-collection queries (search across all sources)
✅ Unified backup/restore
✅ Simplified deployment
✅ Better resource utilization

## Implementation Strategy

### Phase 1: Core RavenDB Integration (Current)
- ✅ Database interface
- ✅ RavenDB service implementation
- ⏳ Storage abstraction layer
- ⏳ Factory pattern for initialization

### Phase 2: Collector Integration
- Update collectors to use storage interfaces
- Migrate from BoltDB bucket pattern to RavenDB collections
- Ensure backward compatibility during migration

### Phase 3: Search & RAG
- Implement full-text search
- Add vector embedding storage
- Create RAG pipeline with Ollama

### Phase 4: Migration Tools
- Tool to migrate existing BoltDB data to RavenDB
- Verification scripts
- Rollback capability

## Configuration

### Proposed Config Structure

```toml
[storage]
type = "ravendb"  # Future: could support "sqlite", "boltdb"

[storage.ravendb]
urls = ["http://localhost:8080"]
database = "quaero"
# Optional:
certificate_path = ""
disable_topology_updates = false

# For file attachments (not in database)
[storage.filesystem]
images = "./data/images"
attachments = "./data/attachments"
```

### Environment Variables
```bash
QUAERO_STORAGE_TYPE=ravendb
QUAERO_RAVENDB_URLS=http://localhost:8080
QUAERO_RAVENDB_DATABASE=quaero
```

## Risks and Mitigations

### Risk: RavenDB Setup Complexity
**Mitigation:**
- Provide Docker Compose file
- Include setup scripts
- Document installation clearly
- Consider bundled binary option

### Risk: Vendor Lock-in
**Mitigation:**
- Design storage abstraction layer
- Keep interfaces generic
- Document migration path to SQLite if needed

### Risk: Memory Usage
**Mitigation:**
- RavenDB is efficient with memory
- Configure appropriate limits
- Monitor resource usage

## Conclusion

**Use RavenDB for Quaero**

The requirements explicitly state:
- ✅ Full-text search required
- ✅ Vector search for RAG pipeline
- ✅ Large document collections (10,000+ pages)
- ✅ Offline operation (RavenDB supports this)

BoltDB cannot meet these requirements without significant additional complexity (Bleve for FTS, separate vector DB, etc.). RavenDB provides everything needed out of the box.

**Next Steps:**
1. Complete storage abstraction layer
2. Remove BoltDB dependencies
3. Update all services to use RavenDB
4. Add deployment documentation
5. Create Docker Compose setup
