# Quaero - Remaining Requirements

**Version:** 2.0
**Date:** 2025-10-05
**Status:** Planning Document

---

## Overview

This document outlines the features and improvements not yet implemented in Quaero. The current implementation provides a solid foundation with Atlassian (Confluence, Jira) collection and web-based browsing. The remaining work focuses on advanced search capabilities, additional data sources, and user experience enhancements.

---

## Current Implementation Status

### ✅ Completed (v1.0 - v2.0)

**Core Infrastructure:**
- ✅ Web-based UI (index.html, confluence.html, jira.html)
- ✅ SQLite storage with FTS5 full-text search
- ✅ Chrome extension authentication
- ✅ WebSocket real-time updates
- ✅ Atlassian collectors (Jira & Confluence)
- ✅ Structured logging (arbor)
- ✅ Configuration system (TOML)
- ✅ HTTP server with graceful shutdown
- ✅ Dependency injection architecture
- ✅ Test suite (integration & unit tests)
- ✅ Build and deployment scripts

**Atlassian Integration:**
- ✅ Jira project and issue collection
- ✅ Confluence space and page collection
- ✅ Authentication management (cookies + tokens)
- ✅ Real-time collection progress
- ✅ Error handling and retry logic

**Web UI:**
- ✅ Dashboard with status
- ✅ Confluence browser and collection
- ✅ Jira browser and collection
- ✅ Real-time log streaming
- ✅ Responsive design

---

## Roadmap

### Phase 1: Search & Query (v2.1) 🎯 PRIORITY

**Goal:** Enable intelligent search and natural language queries

#### 1.1 Vector Embeddings

**Implementation:**
- [ ] Integrate `sqlite-vec` extension
- [ ] Add embeddings column to documents table
- [ ] Implement embedding generation service
- [ ] Connect to local embedding model (e.g., `nomic-embed-text` via Ollama)
- [ ] Generate embeddings for existing documents
- [ ] Background job for new document embedding

**Database Schema Changes:**
```sql
-- Add to confluence_pages and jira_issues
ALTER TABLE confluence_pages ADD COLUMN embedding BLOB;
ALTER TABLE jira_issues ADD COLUMN embedding BLOB;

-- Vector search index
CREATE VIRTUAL TABLE vec_pages USING vec0(
    embedding float[768]
);
```

**Service Implementation:**
```go
// internal/services/embeddings/
type EmbeddingService struct {
    ollamaClient *ollama.Client
    storage      interfaces.StorageManager
    logger       arbor.ILogger
}

func (e *EmbeddingService) GenerateEmbedding(text string) ([]float32, error)
func (e *EmbeddingService) EmbedDocument(docID string) error
func (e *EmbeddingService) SearchBySimilarity(query string, limit int) ([]Document, error)
```

**Deliverables:**
- [ ] Embedding service implementation
- [ ] Migration scripts for schema updates
- [ ] Background embedding job
- [ ] Vector search API endpoints

#### 1.2 RAG Pipeline

**Implementation:**
- [ ] Implement Ollama client for LLM calls
- [ ] Create RAG orchestration service
- [ ] Implement context building from search results
- [ ] Add prompt engineering utilities
- [ ] Create query interface (CLI & Web)

**Architecture:**
```go
// internal/rag/
type RAGEngine struct {
    llmClient       *ollama.Client
    searchService   interfaces.SearchService
    contextBuilder  *ContextBuilder
    logger          arbor.ILogger
}

func (r *RAGEngine) Query(question string) (*Answer, error)
func (r *RAGEngine) buildContext(docs []Document) string
func (r *RAGEngine) generateAnswer(context, question string) (*Answer, error)
```

**Deliverables:**
- [ ] Ollama client integration
- [ ] RAG engine implementation
- [ ] Context building logic
- [ ] Answer generation with citations

#### 1.3 Natural Language Query Interface

**CLI Command:**
```bash
# New query command
quaero query "How do I configure SSO?"
quaero query "What are the latest P0 issues?" --sources
quaero query "Show me the data architecture" --images
```

**Web UI:**
- [ ] Add query page (`pages/query.html`)
- [ ] Query input with suggestions
- [ ] Answer display with formatting
- [ ] Source citations with links
- [ ] Follow-up question support

**API Endpoints:**
```
POST /api/query
{
  "question": "How do I configure SSO?",
  "includeImages": false,
  "maxResults": 5
}

Response:
{
  "answer": "To configure SSO...",
  "sources": [
    {"id": "conf-123", "title": "SSO Guide", "relevance": 0.92},
    ...
  ],
  "confidence": 0.85
}
```

**Deliverables:**
- [ ] Query command implementation
- [ ] Web query interface
- [ ] API endpoints for querying
- [ ] Answer formatting utilities

---

### Phase 2: GitHub Integration (v2.2)

**Goal:** Add GitHub as a data source

#### 2.1 GitHub Collector

**Implementation:**
- [ ] Create GitHub service (`internal/services/github/`)
- [ ] Implement GitHub API client
- [ ] Add repository storage schema
- [ ] Implement README collection
- [ ] Implement wiki collection
- [ ] Optional: Issues and PR collection

**Service Structure:**
```go
// internal/services/github/
type GitHubService struct {
    client  *github.Client
    storage interfaces.GitHubStorage
    logger  arbor.ILogger
}

func (g *GitHubService) FetchRepositories() error
func (g *GitHubService) FetchREADME(repo string) error
func (g *GitHubService) FetchWiki(repo string) error
```

**Configuration:**
```toml
[github]
enabled = true
token = "${GITHUB_TOKEN}"
repos = [
    "owner/repo1",
    "owner/repo2"
]
collect_issues = false
collect_prs = false
```

**Web UI:**
- [ ] GitHub page (`pages/github.html`)
- [ ] Repository browser
- [ ] Collection trigger
- [ ] Real-time progress

**Deliverables:**
- [ ] GitHub service implementation
- [ ] Storage schema for GitHub data
- [ ] Web UI for GitHub
- [ ] API endpoints
- [ ] Documentation

#### 2.2 GitHub Authentication

**Options:**

**Option A: Personal Access Token (Simplest)**
- [ ] Add token field to config
- [ ] Environment variable support
- [ ] Token validation

**Option B: OAuth Flow (Better UX)**
- [ ] Implement GitHub OAuth app
- [ ] Authorization flow in extension
- [ ] Token refresh logic

**Deliverables:**
- [ ] Authentication mechanism
- [ ] Token storage and refresh
- [ ] UI for authentication status

---

### Phase 3: Enhanced Search (v2.3)

**Goal:** Improve search capabilities and user experience

#### 3.1 Hybrid Search

**Implementation:**
- [ ] Combine FTS5 keyword search with vector similarity
- [ ] Implement search ranking algorithm
- [ ] Add filters (source, date, type)
- [ ] Implement faceted search

**Search Service:**
```go
type SearchService struct {
    ftsSearch    interfaces.FullTextSearch
    vectorSearch interfaces.VectorSearch
    logger       arbor.ILogger
}

func (s *SearchService) HybridSearch(query string, filters SearchFilters) ([]Result, error)
func (s *SearchService) RankResults(fts, vector []Result) []Result
```

**Web UI:**
- [ ] Search page with advanced filters
- [ ] Search suggestions/autocomplete
- [ ] Filter UI (source, date, type)
- [ ] Result previews
- [ ] Pagination

**Deliverables:**
- [ ] Hybrid search implementation
- [ ] Ranking algorithm
- [ ] Search UI improvements
- [ ] Filter system

#### 3.2 Image Processing

**Implementation:**
- [ ] OCR for diagrams and screenshots
- [ ] Image embedding for visual similarity
- [ ] Vision model integration (Llama3.2-Vision)
- [ ] Image search by description

**Features:**
- [ ] Extract text from images
- [ ] Generate image descriptions
- [ ] Visual similarity search
- [ ] Include images in RAG context

**Deliverables:**
- [ ] OCR service
- [ ] Vision model integration
- [ ] Image search capabilities
- [ ] Image-aware RAG

---

### Phase 4: Additional Data Sources (v3.0)

**Goal:** Expand beyond Atlassian and GitHub

#### 4.1 Slack Integration

**Implementation:**
- [ ] Slack API client
- [ ] Channel and message collection
- [ ] Thread support
- [ ] File attachment handling
- [ ] Real-time message updates (optional)

**Schema:**
```sql
CREATE TABLE slack_channels (...)
CREATE TABLE slack_messages (...)
CREATE TABLE slack_threads (...)
```

**Authentication:**
- [ ] Slack OAuth app
- [ ] Bot token management
- [ ] Workspace configuration

**Deliverables:**
- [ ] Slack service
- [ ] Storage schema
- [ ] Web UI
- [ ] Documentation

#### 4.2 Linear Integration

**Implementation:**
- [ ] Linear API client
- [ ] Issue collection
- [ ] Project collection
- [ ] Comment collection

**Schema:**
```sql
CREATE TABLE linear_projects (...)
CREATE TABLE linear_issues (...)
CREATE TABLE linear_comments (...)
```

**Deliverables:**
- [ ] Linear service
- [ ] Storage schema
- [ ] Web UI
- [ ] Documentation

#### 4.3 Notion Integration (Optional)

**Implementation:**
- [ ] Notion API client
- [ ] Database collection
- [ ] Page collection
- [ ] Block content parsing

---

### Phase 5: User Experience (v3.1)

**Goal:** Improve usability and features

#### 5.1 Multi-User Support

**Implementation:**
- [ ] User authentication system
- [ ] Per-user configurations
- [ ] Access control (optional)
- [ ] User preferences

**Schema:**
```sql
CREATE TABLE users (...)
CREATE TABLE user_configs (...)
CREATE TABLE user_bookmarks (...)
```

**Features:**
- [ ] Login/logout
- [ ] User dashboard
- [ ] Personal bookmarks
- [ ] Search history

**Deliverables:**
- [ ] Authentication system
- [ ] User management
- [ ] UI updates
- [ ] Access control

#### 5.2 Bookmarks & Collections

**Implementation:**
- [ ] Bookmark documents
- [ ] Create custom collections
- [ ] Share collections (optional)
- [ ] Export collections

**Features:**
- [ ] Bookmark button in UI
- [ ] Collections page
- [ ] Collection management
- [ ] Export to markdown/PDF

**Deliverables:**
- [ ] Bookmark system
- [ ] Collection management
- [ ] UI components
- [ ] Export functionality

#### 5.3 Notifications

**Implementation:**
- [ ] New document notifications
- [ ] Collection completion alerts
- [ ] Query result updates
- [ ] Error notifications

**Channels:**
- [ ] Web UI notifications
- [ ] Email (optional)
- [ ] Slack (optional)

**Deliverables:**
- [ ] Notification service
- [ ] UI notifications
- [ ] Email integration (optional)

---

### Phase 6: Advanced Features (v4.0)

**Goal:** Enterprise and power-user features

#### 6.1 Scheduled Collections

**Implementation:**
- [ ] Cron-based scheduler
- [ ] Collection schedules per source
- [ ] Incremental updates
- [ ] Collision detection

**Configuration:**
```toml
[confluence.schedule]
enabled = true
cron = "0 */6 * * *"  # Every 6 hours
incremental = true

[jira.schedule]
enabled = true
cron = "0 */2 * * *"  # Every 2 hours
incremental = true
```

**Deliverables:**
- [ ] Scheduler service
- [ ] Incremental update logic
- [ ] Configuration UI
- [ ] Schedule management

#### 6.2 Document Versioning

**Implementation:**
- [ ] Track document changes
- [ ] Version comparison
- [ ] Rollback capability
- [ ] Change notifications

**Schema:**
```sql
CREATE TABLE document_versions (
    id INTEGER PRIMARY KEY,
    document_id TEXT,
    version INTEGER,
    content TEXT,
    changed_at TIMESTAMP,
    change_type TEXT
);
```

**Features:**
- [ ] Version history view
- [ ] Diff visualization
- [ ] Restore previous version

**Deliverables:**
- [ ] Versioning system
- [ ] UI for version history
- [ ] Diff utilities

#### 6.3 API Key Management

**Implementation:**
- [ ] Generate API keys for external access
- [ ] Rate limiting
- [ ] Usage tracking
- [ ] Key rotation

**Features:**
- [ ] API key generation
- [ ] Key management UI
- [ ] Usage dashboard
- [ ] Rate limit configuration

**Deliverables:**
- [ ] API key system
- [ ] Rate limiting
- [ ] Usage tracking
- [ ] Management UI

---

### Phase 7: Deployment & Operations (v4.1)

**Goal:** Production-ready deployment options

#### 7.1 Docker Support

**Implementation:**
- [ ] Dockerfile for application
- [ ] Docker Compose setup
- [ ] Volume management
- [ ] Health checks

**Files:**
```yaml
# docker-compose.yml
version: '3.8'
services:
  quaero:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
    environment:
      - QUAERO_LOG_LEVEL=info
```

**Deliverables:**
- [ ] Dockerfile
- [ ] Docker Compose configuration
- [ ] Documentation
- [ ] CI/CD integration

#### 7.2 Cloud Deployment

**Options:**

**Option A: AWS**
- [ ] EC2 deployment guide
- [ ] RDS for SQLite alternative
- [ ] S3 for attachments
- [ ] CloudFormation templates

**Option B: Azure**
- [ ] Azure VM deployment
- [ ] Azure SQL
- [ ] Blob Storage

**Option C: GCP**
- [ ] GCE deployment
- [ ] Cloud SQL
- [ ] Cloud Storage

**Deliverables:**
- [ ] Cloud deployment guides
- [ ] Infrastructure as Code templates
- [ ] Migration tools
- [ ] Cost estimation guides

#### 7.3 Monitoring & Observability

**Implementation:**
- [ ] Prometheus metrics export
- [ ] Grafana dashboards
- [ ] Structured logging enhancements
- [ ] Tracing (optional)

**Metrics:**
- Collection counts and durations
- Query latencies
- Error rates
- Storage usage

**Deliverables:**
- [ ] Metrics exporter
- [ ] Grafana dashboards
- [ ] Alerting rules
- [ ] Runbooks

---

## Priority Matrix

### 🔴 High Priority (Next Sprint)

1. **Vector Embeddings** - Core search capability
2. **RAG Pipeline** - Natural language queries
3. **Query Interface** - User-facing feature

### 🟡 Medium Priority (Next Quarter)

4. **GitHub Integration** - Expand data sources
5. **Hybrid Search** - Improve search quality
6. **Image Processing** - Handle diagrams

### 🟢 Low Priority (Future)

7. **Additional Sources** - Slack, Linear, Notion
8. **Multi-User Support** - Team features
9. **Advanced Features** - Scheduling, versioning
10. **Cloud Deployment** - Production options

---

## Technical Debt & Improvements

### Code Quality

- [ ] Increase test coverage to 80%+
- [ ] Add integration tests for all collectors
- [ ] Implement property-based testing
- [ ] Add benchmarking suite

### Performance

- [ ] Optimize SQLite queries with EXPLAIN QUERY PLAN
- [ ] Add query caching
- [ ] Implement connection pooling
- [ ] Add pagination to all list endpoints

### Security

- [ ] Implement CSRF protection
- [ ] Add rate limiting
- [ ] Secure credential storage (encryption at rest)
- [ ] Add input validation and sanitization
- [ ] Implement content security policy

### Documentation

- [ ] API documentation with OpenAPI/Swagger
- [ ] Developer guide for adding collectors
- [ ] Architecture decision records (ADRs)
- [ ] Performance tuning guide

---

## Migration Path

### From Current (v2.0) to v2.1 (Search & Query)

**Steps:**
1. Add `sqlite-vec` extension to build
2. Update schema with embeddings columns
3. Implement embedding service
4. Create RAG engine
5. Add query command and UI
6. Generate embeddings for existing data

**Estimated Effort:** 3-4 weeks

### From v2.1 to v2.2 (GitHub)

**Steps:**
1. Implement GitHub service
2. Add storage schema
3. Create GitHub UI
4. Add authentication
5. Test and document

**Estimated Effort:** 2-3 weeks

### From v2.2 to v3.0 (Additional Sources)

**Steps:**
1. Implement Slack integration (2 weeks)
2. Implement Linear integration (1-2 weeks)
3. Add unified search across sources

**Estimated Effort:** 4-6 weeks

---

## Success Metrics

### Phase 1 (Search & Query)
- [ ] Users can ask natural language questions
- [ ] Average query response time < 3 seconds
- [ ] Answer accuracy > 80% (based on user feedback)
- [ ] Support for 10,000+ documents

### Phase 2 (GitHub)
- [ ] Collect from 50+ repositories
- [ ] Include GitHub data in search results
- [ ] GitHub UI fully functional

### Phase 3 (Enhanced Search)
- [ ] Hybrid search relevance > 90%
- [ ] Image OCR accuracy > 85%
- [ ] Search includes images and diagrams

### Overall Success
- [ ] 10,000+ documents indexed
- [ ] < 5 second query response time
- [ ] 90%+ search relevance
- [ ] Support for 5+ data sources

---

## Resources & Dependencies

### External Services

**Required:**
- Ollama (local LLM)
  - `nomic-embed-text` (embeddings)
  - `qwen2.5:32b` (text generation)
  - `llama3.2-vision:11b` (vision, optional)

**Optional:**
- Slack workspace
- Linear workspace
- Notion workspace

### Infrastructure

**Development:**
- Local machine with 16GB+ RAM
- SQLite with vec extension
- Chrome browser

**Production:**
- Server with 32GB+ RAM
- SSD storage (for SQLite performance)
- Backup solution

### Team Skills Needed

- Go development
- SQLite optimization
- Vector search expertise
- LLM/RAG knowledge
- Frontend development (HTML/JS)
- Chrome extension development

---

## Risk Assessment

### Technical Risks

**High:**
- 🔴 Vector search performance at scale
- 🔴 LLM response quality and hallucinations
- 🔴 Memory usage with large embeddings

**Medium:**
- 🟡 SQLite limitations for concurrent writes
- 🟡 Chrome extension API changes
- 🟡 Third-party API rate limits

**Low:**
- 🟢 Schema migrations
- 🟢 UI complexity
- 🟢 Testing coverage

### Mitigation Strategies

**Vector Search Performance:**
- Use approximate nearest neighbor (ANN)
- Implement result caching
- Consider PostgreSQL with pgvector for scale

**LLM Quality:**
- Prompt engineering
- Result validation
- User feedback loop
- Confidence scoring

**Memory Usage:**
- Lazy loading of embeddings
- Quantization (float32 → int8)
- Disk-based vector index

---

## Open Questions

1. **Storage:** Should we support PostgreSQL as alternative to SQLite for larger deployments?

2. **Embeddings:** Use local model (Ollama) or cloud service (OpenAI)?

3. **Multi-tenancy:** Single database per user or shared with row-level security?

4. **Real-time:** Should we support real-time document updates from sources?

5. **Search:** Hybrid search weighting - how to balance keyword vs semantic?

6. **UI:** Should we build a native desktop app (Electron) or keep web-only?

---

## Next Steps

### Immediate (This Week)

1. Review and approve this requirements doc
2. Create GitHub issues for Phase 1 tasks
3. Research sqlite-vec integration
4. Prototype embedding generation
5. Design RAG pipeline architecture

### Short Term (This Month)

1. Implement vector embeddings
2. Build RAG engine
3. Create query interface
4. Test with real data
5. Document learnings

### Long Term (This Quarter)

1. Complete Phase 1 (Search & Query)
2. Start Phase 2 (GitHub)
3. Plan Phase 3 (Enhanced Search)
4. Gather user feedback
5. Adjust roadmap based on usage

---

**Last Updated:** 2025-10-05
**Version:** 1.0
**Status:** Planning Document
