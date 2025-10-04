# quaero-storage

Implements the RavenDB storage layer for Quaero, providing document storage with full-text and vector search capabilities.

## Usage

```
/quaero-storage <quaero-project-path>
```

## Arguments

- `quaero-project-path` (required): Path to Quaero monorepo

## What it does

### Phase 1: RavenDB Client Implementation

1. **Storage Interface** (`pkg/models/storage.go`)
   - Define Storage interface for all storage operations
   - Support for multiple storage backends (RavenDB primary)

2. **RavenDB Store** (`internal/storage/ravendb/store.go`)
   - RavenDB client initialization
   - Document CRUD operations
   - Index management
   - Connection pooling

3. **Core Methods**
   - `Store(doc *models.Document) error` - Store single document
   - `StoreBatch(docs []*models.Document) error` - Bulk insert
   - `Get(id string) (*models.Document, error)` - Retrieve by ID
   - `Delete(id string) error` - Delete document
   - `Update(doc *models.Document) error` - Update document

### Phase 2: Search Implementation

1. **Query Interface** (`internal/storage/ravendb/queries.go`)
   - Full-text search
   - Vector similarity search
   - Filtered search (by source, date, etc.)
   - Faceted search

2. **Search Methods**
   - `FullTextSearch(query string, opts SearchOptions) ([]*models.Document, error)`
   - `VectorSearch(embedding []float64, opts SearchOptions) ([]*models.Document, error)`
   - `FilterBySource(source string) ([]*models.Document, error)`
   - `FilterByDate(start, end time.Time) ([]*models.Document, error)`

3. **Search Options**
   ```go
   type SearchOptions struct {
       Limit       int
       Offset      int
       Sources     []string
       MinScore    float64
       IncludeBody bool
   }
   ```

### Phase 3: Indexing

1. **Index Definitions** (`internal/storage/ravendb/indexes.go`)
   - Full-text index on ContentMD
   - Metadata index (source, dates, etc.)
   - Vector index for embeddings

2. **Index Management**
   - Create indexes on startup
   - Index optimization
   - Index statistics

### Phase 4: Vector Storage

1. **Embedding Storage**
   - Store vector embeddings with documents
   - Efficient vector similarity search
   - Configurable embedding dimensions

2. **Vector Operations**
   - Cosine similarity search
   - K-nearest neighbors
   - Approximate nearest neighbor (ANN)

### Phase 5: Statistics & Monitoring

1. **Stats Methods** (`internal/storage/ravendb/stats.go`)
   - `GetDocumentCount() (int, error)`
   - `GetStats() (*StorageStats, error)`
   - `GetSourceBreakdown() (map[string]int, error)`

2. **Stats Structure**
   ```go
   type StorageStats struct {
       TotalDocuments int
       TotalChunks    int
       TotalImages    int
       BySource       map[string]int
       LastUpdated    time.Time
       DatabaseSize   int64
   }
   ```

### Phase 6: Testing

1. **Unit Tests** (`internal/storage/ravendb/store_test.go`)
   - CRUD operations
   - Search functionality
   - Index creation
   - Vector operations

2. **Integration Tests** (`test/integration/storage_test.go`)
   - Full workflow tests
   - Performance tests
   - Concurrent access tests

### Phase 7: Mock Implementation

1. **Mock Store** (`internal/storage/mock/store.go`)
   - In-memory implementation for testing
   - Implements Storage interface
   - No external dependencies

## Storage Interface

```go
// pkg/models/storage.go
type Storage interface {
    // CRUD
    Store(doc *Document) error
    StoreBatch(docs []*Document) error
    Get(id string) (*Document, error)
    Delete(id string) error
    Update(doc *Document) error

    // Search
    FullTextSearch(query string, opts SearchOptions) ([]*Document, error)
    VectorSearch(embedding []float64, opts SearchOptions) ([]*Document, error)

    // Filtering
    FilterBySource(source string) ([]*Document, error)
    FilterByDateRange(start, end time.Time) ([]*Document, error)

    // Stats
    GetStats() (*StorageStats, error)
    GetDocumentCount() (int, error)

    // Cleanup
    Close() error
}
```

## RavenDB Implementation

```go
// internal/storage/ravendb/store.go
type Store struct {
    store  *ravendb.DocumentStore
    logger *arbor.Logger
    config *Config
}

type Config struct {
    URLs     []string
    Database string
}

func NewStore(config *Config, logger *arbor.Logger) (*Store, error) {
    store := ravendb.NewDocumentStore(config.URLs, config.Database)
    err := store.Initialize()
    if err != nil {
        return nil, err
    }

    s := &Store{
        store:  store,
        logger: logger,
        config: config,
    }

    // Create indexes
    s.createIndexes()

    return s, nil
}

func (s *Store) Store(doc *models.Document) error {
    session := s.store.OpenSession()
    defer session.Close()

    err := session.Store(doc)
    if err != nil {
        return err
    }

    return session.SaveChanges()
}

func (s *Store) FullTextSearch(query string, opts SearchOptions) ([]*models.Document, error) {
    session := s.store.OpenSession()
    defer session.Close()

    var results []*models.Document
    q := session.Query(reflect.TypeOf(&models.Document{})).
        Search("ContentMD", query).
        Take(opts.Limit).
        Skip(opts.Offset)

    if len(opts.Sources) > 0 {
        q = q.WhereIn("Source", opts.Sources)
    }

    err := q.GetResults(&results)
    return results, err
}
```

## Test-Driven Development (TDD) Workflow

**CRITICAL**: Follow TDD methodology for ALL code implementation.

### TDD Cycle (Red-Green-Refactor)

For EACH component, function, or feature:

1. **RED - Write Failing Test First**
   ```bash
   # Create test file before implementation
   touch internal/component/component_test.go

   # Write test that describes desired behavior
   func TestComponentBehavior(t *testing.T) {
       // Arrange - setup test data
       // Act - call the function (doesn't exist yet)
       // Assert - verify expected behavior
   }

   # Run test - should FAIL
   go test ./internal/component/... -v
   # Output: undefined: ComponentFunction
   ```

2. **GREEN - Write Minimal Code to Pass**
   ```bash
   # Implement just enough code to make test pass
   # Run test again
   go test ./internal/component/... -v
   # Output: PASS
   ```

3. **REFACTOR - Improve Code Quality**
   ```bash
   # Refactor while keeping tests green
   # Run test after each change
   go test ./internal/component/... -v
   ```

4. **REPEAT** - For next feature/function

### Testing Requirements by Component

**Before writing ANY implementation code:**

1. **Interfaces** - Create interface test
   ```go
   func TestInterfaceImplementation(t *testing.T) {
       var _ models.Source = (*ConfluenceSource)(nil) // Compile-time check
   }
   ```

2. **Core Functions** - Table-driven tests
   ```go
   func TestProcessMarkdown(t *testing.T) {
       tests := []struct{
           name     string
           input    string
           expected string
       }{
           {"basic html", "<p>test</p>", "test"},
           // Add more cases
       }
       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               result := ProcessMarkdown(tt.input)
               assert.Equal(t, tt.expected, result)
           })
       }
   }
   ```

3. **API Clients** - Mock HTTP responses
   ```go
   func TestGetPages(t *testing.T) {
       // Setup mock server
       server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
           w.WriteHeader(200)
           json.NewEncoder(w).Encode(mockPages)
       }))
       defer server.Close()

       // Test with mock
       client := NewClient(server.URL)
       pages, err := client.GetPages()

       assert.NoError(t, err)
       assert.Len(t, pages, 2)
   }
   ```

4. **Integration Tests** - Test component interactions
   ```go
   func TestFullWorkflow(t *testing.T) {
       // Setup
       storage := mock.NewStorage()
       source := NewSource(config, storage)

       // Execute
       docs, err := source.Collect(context.Background())

       // Verify
       assert.NoError(t, err)
       assert.Greater(t, len(docs), 0)
   }
   ```

### Continuous Testing Workflow

**After EVERY code change:**

```bash
# 1. Run specific component tests
go test ./internal/component/... -v

# 2. If pass, run all tests
go test ./... -v

# 3. Check coverage (must be >80%)
go test ./... -cover

# 4. If all pass, proceed to next test/feature
```

### Test Organization

```
internal/component/
├── component.go           # Implementation
├── component_test.go      # Unit tests
└── testdata/              # Test fixtures
    ├── input.json
    └── expected.json

test/integration/
├── component_flow_test.go # Integration tests
└── fixtures/              # Shared fixtures
```

### Testing Checklist

Before marking ANY component complete:
- [ ] Unit tests written BEFORE implementation
- [ ] All tests passing (`go test ./... -v`)
- [ ] Test coverage >80% (`go test -cover`)
- [ ] Table-driven tests for multiple cases
- [ ] Mock external dependencies
- [ ] Integration tests for workflows
- [ ] Edge cases tested (nil, empty, errors)
- [ ] Error paths tested


## Examples

### Implement Storage Layer
```
/quaero-storage C:\development\quaero
```

### Test Storage Operations
```
/quaero-storage C:\development\quaero --test
```

## Validation

After implementation, verifies:
- ✓ RavenDB client initialized
- ✓ Storage interface implemented
- ✓ CRUD operations working
- ✓ Full-text search functional
- ✓ Vector search implemented
- ✓ Indexes created successfully
- ✓ Stats and monitoring working
- ✓ Mock store for testing
- ✓ Unit tests passing
- ✓ Integration tests passing

## Output

Provides detailed report:
- Files created/modified
- Storage interface defined
- RavenDB implementation status
- Search capabilities
- Index definitions
- Tests created

---

**Agent**: quaero-storage

**Prompt**: Implement the RavenDB storage layer for Quaero at {{args.[0]}}.

## Implementation Tasks

1. **Define Storage Interface** (`pkg/models/storage.go`)
   - Complete Storage interface with all operations
   - SearchOptions struct
   - StorageStats struct

2. **Implement RavenDB Store** (`internal/storage/ravendb/store.go`)
   - Store initialization and connection
   - CRUD operations (Store, Get, Update, Delete)
   - Batch operations
   - Connection management

3. **Implement Search** (`internal/storage/ravendb/queries.go`)
   - Full-text search with RavenDB queries
   - Vector similarity search
   - Filtering operations
   - Pagination support

4. **Create Indexes** (`internal/storage/ravendb/indexes.go`)
   - Full-text index definition
   - Metadata indexes
   - Vector index for embeddings
   - Auto-create on startup

5. **Add Statistics** (`internal/storage/ravendb/stats.go`)
   - Document count queries
   - Source breakdown
   - Storage statistics
   - Performance metrics

6. **Create Mock Store** (`internal/storage/mock/store.go`)
   - In-memory implementation
   - Implements Storage interface
   - For testing without RavenDB

7. **Create Tests**
   - Unit tests for all operations
   - Integration tests with RavenDB
   - Performance benchmarks
   - Concurrent access tests

## Code Quality Standards

- Implements Storage interface completely
- Thread-safe operations
- Structured logging (arbor)
- Comprehensive error handling
- Connection pooling
- Resource cleanup
- 80%+ test coverage

## Success Criteria

✓ Storage interface fully defined
✓ RavenDB client working
✓ CRUD operations functional
✓ Full-text search working
✓ Vector search implemented
✓ Indexes created automatically
✓ Stats and monitoring functional
✓ Mock store for testing
✓ All tests passing
