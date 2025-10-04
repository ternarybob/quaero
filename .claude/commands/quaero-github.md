# quaero-github

Implements the GitHub data source collector for Quaero, extracting repositories, README files, documentation, and issues.

## Usage

```
/quaero-github <quaero-project-path>
```

## Arguments

- `quaero-project-path` (required): Path to Quaero monorepo

## What it does

### Phase 1: API Client Implementation

1. **API Client** (`internal/sources/github/client.go`)
   - GitHub REST API v3 client
   - Token-based authentication
   - Rate limiting handling
   - Pagination support
   - Error handling and retries

2. **API Methods**
   - `GetRepository(owner, repo string) (*Repository, error)` - Get repo details
   - `GetREADME(owner, repo string) (string, error)` - Get README content
   - `GetDocs(owner, repo string) ([]*File, error)` - Get docs/ directory
   - `GetIssues(owner, repo string) ([]*Issue, error)` - Get repository issues
   - `GetFileContent(owner, repo, path string) (string, error)` - Get file content
   - `GetMarkdownFiles(owner, repo string) ([]*File, error)` - Find all .md files

### Phase 2: Document Processor

1. **Processor** (`internal/sources/github/processor.go`)
   - Converts GitHub content to Document model
   - Processes Markdown files
   - Extracts code documentation
   - Content chunking for RAG
   - Metadata extraction

2. **Processing Methods**
   - `FileToDocument(file *File) (*models.Document, error)` - Convert file
   - `IssueToDocument(issue *Issue) (*models.Document, error)` - Convert issue
   - `ProcessMarkdown(content string) (string, error)` - Clean Markdown
   - `ExtractCodeBlocks(content string) ([]string, error)` - Extract code
   - `ChunkContent(content string) ([]models.Chunk, error)` - Create chunks

### Phase 3: Source Implementation

1. **Source Interface** (`internal/sources/github/github.go`)
   - Implements `models.Source` interface
   - Orchestrates API client and processor
   - Handles collection workflow
   - Progress tracking and logging

2. **Interface Methods**
   ```go
   func (g *GitHubSource) Name() string
   func (g *GitHubSource) Collect(ctx context.Context) ([]*models.Document, error)
   func (g *GitHubSource) SupportsImages() bool
   ```

3. **Collection Workflow**
   - Parse configured repository list
   - For each repository:
     - Get README.md
     - Get all files in docs/ directory
     - Find all .md files in root
     - Optionally get issues
     - Convert to Document model
   - Return all documents

### Phase 4: Configuration

1. **Config Structure**
   ```go
   type GitHubConfig struct {
       Enabled       bool     `toml:"enabled"`
       Token         string   `toml:"token"`
       Repos         []string `toml:"repos"`
       IncludeIssues bool     `toml:"include_issues"`
       DocsOnly      bool     `toml:"docs_only"`
   }
   ```

2. **Repository Specification**
   - Format: `owner/repo` (e.g., `facebook/react`)
   - Support for multiple repositories
   - Public and private repositories (with token)

### Phase 5: Content Collection

1. **Documentation Files**
   - README.md (always)
   - docs/ directory (recursively)
   - CONTRIBUTING.md
   - CODE_OF_CONDUCT.md
   - LICENSE (optional)
   - All .md files in root

2. **Issue Collection** (optional)
   - Open issues
   - Closed issues (last 100)
   - Issue comments
   - Labels and metadata

### Phase 6: Image Handling

1. **Image References**
   - Extract image URLs from Markdown
   - Download images from repository
   - Store in `data/images/`
   - Update references in Document

### Phase 7: Testing

1. **Unit Tests** (`internal/sources/github/github_test.go`)
   - API client functionality
   - File to Document conversion
   - Markdown processing
   - Image extraction

2. **Integration Tests** (`test/integration/github_flow_test.go`)
   - Full collection workflow
   - Mock GitHub API responses
   - Storage integration

## GitHub Data Model

### File Structure
```go
type File struct {
    Path    string
    Content string
    SHA     string
    Size    int
    Type    string
}

type Repository struct {
    Owner       string
    Name        string
    Description string
    URL         string
    Stars       int
    Language    string
}

type Issue struct {
    Number   int
    Title    string
    Body     string
    State    string
    Labels   []string
    Created  time.Time
    Comments []Comment
}
```

### Document Conversion
```go
// Convert File → Document
Document{
    ID:        fmt.Sprintf("github-%s-%s-%s", owner, repo, sanitizePath(file.Path)),
    Source:    "github",
    Title:     extractTitle(file.Path),
    ContentMD: file.Content,
    Chunks:    chunkContent(file.Content),
    Images:    extractImages(file.Content),
    Metadata: {
        "repository": fmt.Sprintf("%s/%s", owner, repo),
        "path":       file.Path,
        "url":        buildFileURL(owner, repo, file.Path),
        "type":       "documentation",
    },
}

// Convert Issue → Document
Document{
    ID:        fmt.Sprintf("github-%s-%s-issue-%d", owner, repo, issue.Number),
    Source:    "github",
    Title:     fmt.Sprintf("#%d %s", issue.Number, issue.Title),
    ContentMD: formatIssue(issue),
    Chunks:    chunkContent(contentMD),
    Metadata: {
        "repository": fmt.Sprintf("%s/%s", owner, repo),
        "issueNumber": issue.Number,
        "state":      issue.State,
        "labels":     issue.Labels,
        "url":        buildIssueURL(owner, repo, issue.Number),
        "type":       "issue",
    },
}
```

## Code Structure

### API Client Pattern
```go
type APIClient struct {
    token   string
    client  *http.Client
    logger  *arbor.Logger
    baseURL string
}

func NewAPIClient(token string, logger *arbor.Logger) *APIClient

func (c *APIClient) makeRequest(method, endpoint string) (*http.Response, error) {
    req, _ := http.NewRequest(method, c.baseURL+endpoint, nil)

    // Add authorization header
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
    req.Header.Set("Accept", "application/vnd.github.v3+json")

    return c.client.Do(req)
}

func (c *APIClient) GetREADME(owner, repo string) (string, error) {
    endpoint := fmt.Sprintf("/repos/%s/%s/readme", owner, repo)
    resp := c.makeRequest("GET", endpoint)

    // Decode base64 content
    var data struct {
        Content string `json:"content"`
    }
    json.NewDecoder(resp.Body).Decode(&data)

    content, _ := base64.StdEncoding.DecodeString(data.Content)
    return string(content), nil
}
```

### Source Implementation Pattern
```go
type GitHubSource struct {
    apiClient  *APIClient
    processor  *Processor
    config     *GitHubConfig
    logger     *arbor.Logger
}

func NewGitHubSource(config *GitHubConfig, logger *arbor.Logger) *GitHubSource

func (g *GitHubSource) Collect(ctx context.Context) ([]*models.Document, error) {
    var documents []*models.Document

    for _, repoSpec := range g.config.Repos {
        parts := strings.Split(repoSpec, "/")
        owner, repo := parts[0], parts[1]

        g.logger.Info("Collecting from GitHub repository", "repo", repoSpec)

        // Get README
        readme := g.apiClient.GetREADME(owner, repo)
        doc := g.processor.FileToDocument(&File{Path: "README.md", Content: readme})
        documents = append(documents, doc)

        // Get docs/
        docs := g.apiClient.GetDocs(owner, repo)
        for _, file := range docs {
            content := g.apiClient.GetFileContent(owner, repo, file.Path)
            doc := g.processor.FileToDocument(&File{Path: file.Path, Content: content})
            documents = append(documents, doc)
        }

        // Get issues (optional)
        if g.config.IncludeIssues {
            issues := g.apiClient.GetIssues(owner, repo)
            for _, issue := range issues {
                doc := g.processor.IssueToDocument(issue)
                documents = append(documents, doc)
            }
        }
    }

    return documents, nil
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

### Implement GitHub Collector
```
/quaero-github C:\development\quaero
```

### Add Multiple Repositories
```
/quaero-github C:\development\quaero --repos=facebook/react,microsoft/typescript
```

## Validation

After implementation, verifies:
- ✓ API client authenticates with GitHub
- ✓ README files retrieved
- ✓ Documentation collected from docs/
- ✓ Markdown files processed
- ✓ Issues collected (if enabled)
- ✓ Source interface implemented
- ✓ Images extracted and downloaded
- ✓ Content chunking functional
- ✓ Metadata properly extracted
- ✓ Unit tests passing
- ✓ Integration tests passing

## Output

Provides detailed report:
- Files created/modified
- API client implementation status
- Document conversion pipeline
- Image handling implementation
- Tests created
- Sample documents generated

---

**Agent**: quaero-github

**Prompt**: Implement the GitHub data source collector for Quaero at {{args.[0]}}.

## Implementation Tasks

1. **Build API Client** (`internal/sources/github/client.go`)
   - GitHub API v3 client with token auth
   - Methods: GetRepository, GetREADME, GetDocs, GetIssues, GetFileContent
   - Rate limiting, pagination, error handling
   - Base64 content decoding

2. **Implement Processor** (`internal/sources/github/processor.go`)
   - File → Document conversion
   - Issue → Document conversion
   - Markdown processing and cleaning
   - Code block extraction
   - Content chunking for RAG
   - Metadata extraction (repo, path, URL, type)

3. **Create Source Implementation** (`internal/sources/github/github.go`)
   - Implement models.Source interface
   - Collection orchestration (API + Processor)
   - Progress tracking and logging
   - Error handling and recovery

4. **Add Configuration**
   - GitHubConfig struct
   - Repository list (owner/repo format)
   - Include/exclude issues
   - Docs-only mode

5. **Implement Image Handling**
   - Extract image URLs from Markdown
   - Download images from repository
   - Save to data/images/
   - Update references in Document

6. **Create Tests**
   - Unit tests for API, processor
   - Integration test for full collection flow
   - Mock GitHub API responses
   - Test fixtures with sample files

## Code Quality Standards

- Implements models.Source interface
- Token-based authentication
- Structured logging (arbor)
- Comprehensive error handling
- Rate limit awareness
- Proper resource cleanup (HTTP clients)
- 80%+ test coverage

## Success Criteria

✓ API client retrieves GitHub content
✓ README and docs collected
✓ Files converted to Document model
✓ Issues collected (if enabled)
✓ Images extracted and downloaded
✓ Markdown processing accurate
✓ Source interface fully implemented
✓ All tests passing
✓ Can collect from configured repos
