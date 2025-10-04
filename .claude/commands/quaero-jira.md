# quaero-jira

Implements the Jira data source collector for Quaero, extracting issues, comments, and attachments from Jira.

## Usage

```
/quaero-jira <quaero-project-path>
```

## Arguments

- `quaero-project-path` (required): Path to Quaero monorepo

## What it does

### Phase 1: API Client Implementation

1. **API Client** (`internal/sources/jira/client.go`)
   - REST API client for Jira
   - Authentication using credentials from AuthManager
   - Cookie and token injection
   - JQL query support
   - Pagination handling
   - Rate limiting and retry logic

2. **API Methods**
   - `GetProjects() ([]*Project, error)` - List accessible projects
   - `GetIssuesInProject(projectKey string) ([]*Issue, error)` - Get issues
   - `GetIssue(issueKey string) (*Issue, error)` - Get single issue with details
   - `GetComments(issueKey string) ([]*Comment, error)` - Get issue comments
   - `GetAttachments(issueKey string) ([]*Attachment, error)` - Get attachments
   - `SearchJQL(jql string) ([]*Issue, error)` - Search with JQL
   - `DownloadAttachment(url string) ([]byte, error)` - Download file

### Phase 2: Document Processor

1. **Processor** (`internal/sources/jira/processor.go`)
   - Converts Jira issues to Document model
   - Formats issue content as Markdown
   - Extracts and processes attachments
   - Content chunking for RAG
   - Metadata extraction

2. **Processing Methods**
   - `IssueToDocument(issue *Issue) (*models.Document, error)` - Convert issue
   - `FormatIssue(issue *Issue) (string, error)` - Format as Markdown
   - `ExtractAttachments(issue *Issue) ([]*models.Image, error)` - Process attachments
   - `ChunkContent(content string) ([]models.Chunk, error)` - Create chunks
   - `ExtractMetadata(issue *Issue) (map[string]interface{}, error)` - Metadata

### Phase 3: Source Implementation

1. **Source Interface** (`internal/sources/jira/jira.go`)
   - Implements `models.Source` interface
   - Orchestrates API client and processor
   - Handles collection workflow
   - Progress tracking and logging

2. **Interface Methods**
   ```go
   func (j *JiraSource) Name() string
   func (j *JiraSource) Collect(ctx context.Context) ([]*models.Document, error)
   func (j *JiraSource) SupportsImages() bool
   ```

3. **Collection Workflow**
   - Retrieve auth from AuthManager
   - Get list of configured projects
   - For each project:
     - Get all issues via API
     - Process issue details
     - Extract comments and format
     - Download and process attachments
     - Convert to Document model
   - Return all documents

### Phase 4: Configuration

1. **Config Structure**
   ```go
   type JiraConfig struct {
       Enabled      bool     `toml:"enabled"`
       BaseURL      string   `toml:"base_url"`
       Projects     []string `toml:"projects"`
       IssueTypes   []string `toml:"issue_types"`
       Statuses     []string `toml:"statuses"`
       MaxIssues    int      `toml:"max_issues"`
       IncludeComments bool  `toml:"include_comments"`
   }
   ```

2. **Filtering**
   - Collect from specific projects only
   - Filter by issue type (Bug, Story, Task, etc.)
   - Filter by status (Open, In Progress, Done)
   - Custom JQL queries

### Phase 5: Issue Formatting

1. **Markdown Conversion**
   - Issue summary as title
   - Description converted to Markdown
   - Field formatting (Status, Assignee, Priority, etc.)
   - Comments section
   - Links to related issues

2. **Example Format**
   ```markdown
   # [PROJECT-123] Issue Title

   **Type:** Story
   **Status:** In Progress
   **Priority:** High
   **Assignee:** John Doe
   **Reporter:** Jane Smith
   **Created:** 2025-10-01
   **Updated:** 2025-10-04

   ## Description

   [Issue description in Markdown]

   ## Comments

   ### John Doe - 2025-10-02
   [Comment text]

   ### Jane Smith - 2025-10-03
   [Reply text]

   ## Related Issues
   - Blocks: PROJECT-122
   - Related to: PROJECT-124
   ```

### Phase 6: Attachment Handling

1. **Attachment Processing**
   - Download image attachments
   - Store in `data/images/`
   - Track references in Document
   - Extract metadata (filename, size, type)

2. **Supported Types**
   - Images (PNG, JPG, GIF)
   - Documents (referenced but not processed)
   - Diagrams and screenshots

### Phase 7: Testing

1. **Unit Tests** (`internal/sources/jira/jira_test.go`)
   - API client functionality
   - Issue to Document conversion
   - Markdown formatting
   - Attachment handling

2. **Integration Tests** (`test/integration/jira_flow_test.go`)
   - Full collection workflow
   - Auth integration
   - Storage integration
   - Mock Jira API responses

## Jira Data Model

### Issue Structure
```go
type Issue struct {
    Key         string
    Summary     string
    Description string
    IssueType   string
    Status      string
    Priority    string
    Assignee    string
    Reporter    string
    Created     time.Time
    Updated     time.Time
    Labels      []string
    Components  []string
    Comments    []Comment
    Attachments []Attachment
    Links       []IssueLink
}

type Comment struct {
    Author  string
    Body    string
    Created time.Time
}

type Attachment struct {
    ID       string
    Filename string
    MimeType string
    Size     int64
    URL      string
}
```

### Document Conversion
```go
// Convert Issue → Document
Document{
    ID:        fmt.Sprintf("jira-%s", issue.Key),
    Source:    "jira",
    Title:     fmt.Sprintf("[%s] %s", issue.Key, issue.Summary),
    ContentMD: formatIssue(issue),
    Chunks:    chunkContent(contentMD),
    Images:    extractAttachments(issue),
    Metadata: {
        "issueKey":   issue.Key,
        "project":    extractProject(issue.Key),
        "issueType":  issue.IssueType,
        "status":     issue.Status,
        "priority":   issue.Priority,
        "assignee":   issue.Assignee,
        "labels":     issue.Labels,
        "url":        buildIssueURL(issue),
    },
}
```

## Code Structure

### API Client Pattern
```go
type APIClient struct {
    baseURL string
    auth    *auth.AuthCredentials
    client  *http.Client
    logger  *arbor.Logger
}

func NewAPIClient(baseURL string, auth *auth.AuthCredentials, logger *arbor.Logger) *APIClient

func (c *APIClient) makeRequest(method, endpoint string) (*http.Response, error) {
    req, _ := http.NewRequest(method, c.baseURL+endpoint, nil)

    // Inject cookies from auth
    for _, cookie := range c.auth.Cookies {
        req.AddCookie(cookie)
    }

    // Inject tokens
    if c.auth.Tokens.AtlToken != "" {
        req.Header.Set("X-Atlassian-Token", c.auth.Tokens.AtlToken)
    }

    return c.client.Do(req)
}

func (c *APIClient) SearchJQL(jql string) ([]*Issue, error) {
    endpoint := fmt.Sprintf("/rest/api/2/search?jql=%s", url.QueryEscape(jql))
    // Handle pagination with maxResults and startAt
}
```

### Source Implementation Pattern
```go
type JiraSource struct {
    apiClient  *APIClient
    processor  *Processor
    config     *JiraConfig
    logger     *arbor.Logger
}

func NewJiraSource(auth *auth.AuthCredentials, config *JiraConfig, logger *arbor.Logger) *JiraSource

func (j *JiraSource) Collect(ctx context.Context) ([]*models.Document, error) {
    var documents []*models.Document

    // Get projects
    projects := j.config.Projects

    for _, projectKey := range projects {
        j.logger.Info("Collecting from Jira project", "project", projectKey)

        // Get issues in project
        issues := j.apiClient.GetIssuesInProject(projectKey)

        for _, issue := range issues {
            // Get full issue details (comments, attachments)
            fullIssue := j.apiClient.GetIssue(issue.Key)

            // Convert to document
            doc := j.processor.IssueToDocument(fullIssue)
            documents = append(documents, doc)
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

### Implement Jira Collector
```
/quaero-jira C:\development\quaero
```

### Add Custom Filtering
```
/quaero-jira C:\development\quaero --projects=DATA,ENG --types=Story,Bug
```

## Validation

After implementation, verifies:
- ✓ API client authenticates with Jira
- ✓ Issues retrieved from projects
- ✓ Processor converts to Document model
- ✓ Source interface implemented
- ✓ Comments included and formatted
- ✓ Attachments downloaded and stored
- ✓ Markdown conversion accurate
- ✓ Content chunking functional
- ✓ Metadata properly extracted
- ✓ Unit tests passing
- ✓ Integration tests passing

## Output

Provides detailed report:
- Files created/modified
- API client implementation status
- Document conversion pipeline
- Attachment handling implementation
- Tests created
- Sample documents generated

---

**Agent**: quaero-jira

**Prompt**: Implement the Jira data source collector for Quaero at {{args.[0]}}.

## Implementation Tasks

1. **Build API Client** (`internal/sources/jira/client.go`)
   - REST API client with auth injection
   - Methods: GetProjects, GetIssuesInProject, GetIssue, GetComments, SearchJQL
   - Pagination, rate limiting, error handling
   - Cookie and token injection from AuthManager

2. **Implement Processor** (`internal/sources/jira/processor.go`)
   - Issue → Document conversion
   - Format issue as Markdown (title, fields, description, comments)
   - Attachment extraction and storage
   - Content chunking for RAG
   - Metadata extraction (project, type, status, assignee, labels)

3. **Create Source Implementation** (`internal/sources/jira/jira.go`)
   - Implement models.Source interface
   - Collection orchestration (API + Processor)
   - Progress tracking and logging
   - Error handling and recovery

4. **Add Configuration**
   - JiraConfig struct
   - Project filtering
   - Issue type and status filtering
   - Custom JQL support
   - Max issues limit

5. **Implement Attachment Handling**
   - Download image attachments
   - Save to data/images/
   - Track references in Document
   - Extract metadata

6. **Create Tests**
   - Unit tests for API, processor
   - Integration test for full collection flow
   - Mock Jira API responses
   - Test fixtures with sample issues

## Code Quality Standards

- Implements models.Source interface
- Uses auth.AuthCredentials from AuthManager
- Structured logging (arbor)
- Comprehensive error handling
- Thread-safe operations
- Proper resource cleanup (HTTP clients)
- 80%+ test coverage

## Success Criteria

✓ API client retrieves Jira issues
✓ Issues converted to Document model
✓ Comments included and formatted
✓ Attachments extracted and stored
✓ Markdown conversion accurate
✓ Source interface fully implemented
✓ All tests passing
✓ Can collect from configured projects
