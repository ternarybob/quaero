# quaero-confluence

Implements the Confluence data source collector for Quaero, extracting pages, attachments, and images from Confluence.

## Usage

```
/quaero-confluence <quaero-project-path>
```

## Arguments

- `quaero-project-path` (required): Path to Quaero monorepo

## What it does

### Phase 1: API Client Implementation

1. **API Client** (`internal/sources/confluence/api.go`)
   - REST API client for Confluence
   - Authentication using credentials from AuthManager
   - Cookie and token injection
   - Pagination handling
   - Rate limiting and retry logic
   - Error handling for API responses

2. **API Methods**
   - `GetSpaces() ([]*Space, error)` - List all accessible spaces
   - `GetPagesInSpace(spaceKey string) ([]*Page, error)` - Get pages in space
   - `GetPage(pageId string) (*Page, error)` - Get single page with content
   - `GetPageChildren(pageId string) ([]*Page, error)` - Get child pages
   - `GetAttachments(pageId string) ([]*Attachment, error)` - Get attachments
   - `DownloadAttachment(url string) ([]byte, error)` - Download file

### Phase 2: Browser Scraper Implementation

1. **Scraper** (`internal/sources/confluence/scraper.go`)
   - Browser automation using rod
   - Handles JavaScript-rendered content
   - Authenticates using cookies from AuthManager
   - Screenshot capture for pages
   - Dynamic content extraction

2. **Scraping Methods**
   - `ScrapePageContent(pageUrl string) (*PageContent, error)` - Full page content
   - `CaptureScreenshot(pageUrl string) ([]byte, error)` - Page screenshot
   - `ExtractDiagrams(pageUrl string) ([]*Image, error)` - Extract embedded diagrams
   - `GetRenderedHTML(pageUrl string) (string, error)` - Get rendered HTML

### Phase 3: Document Processor

1. **Processor** (`internal/sources/confluence/processor.go`)
   - Converts Confluence pages to Document model
   - HTML to Markdown conversion
   - Image extraction and storage
   - Content chunking for RAG
   - Metadata extraction

2. **Processing Methods**
   - `PageToDocument(page *Page) (*models.Document, error)` - Convert page
   - `ExtractImages(content string) ([]*models.Image, error)` - Extract images
   - `ConvertToMarkdown(html string) (string, error)` - HTML→MD conversion
   - `ChunkContent(content string) ([]models.Chunk, error)` - Create chunks
   - `ExtractMetadata(page *Page) (map[string]interface{}, error)` - Metadata

### Phase 4: Source Implementation

1. **Source Interface** (`internal/sources/confluence/confluence.go`)
   - Implements `models.Source` interface
   - Orchestrates API, scraper, and processor
   - Handles collection workflow
   - Progress tracking and logging

2. **Interface Methods**
   ```go
   func (c *ConfluenceSource) Name() string
   func (c *ConfluenceSource) Collect(ctx context.Context) ([]*models.Document, error)
   func (c *ConfluenceSource) SupportsImages() bool
   ```

3. **Collection Workflow**
   - Retrieve auth from AuthManager
   - Get list of configured spaces
   - For each space:
     - Get all pages via API
     - Scrape content with rod
     - Extract images and attachments
     - Process to Document model
     - Handle child pages recursively
   - Return all documents

### Phase 5: Configuration

1. **Config Structure**
   ```go
   type ConfluenceConfig struct {
       Enabled    bool     `toml:"enabled"`
       BaseURL    string   `toml:"base_url"`
       Spaces     []string `toml:"spaces"`
       MaxPages   int      `toml:"max_pages"`
       IncludeAttachments bool `toml:"include_attachments"`
   }
   ```

2. **Space Filtering**
   - Collect from specific spaces only
   - Pattern matching for space keys
   - Include/exclude lists

### Phase 6: Image Handling

1. **Image Extraction**
   - Inline images from content
   - Attached images
   - Embedded diagrams (draw.io, Gliffy)
   - Screenshots

2. **Image Storage**
   - Save to `data/images/` with unique IDs
   - Track image references in Document
   - Generate thumbnails
   - OCR for diagram text extraction

### Phase 7: Testing

1. **Unit Tests** (`internal/sources/confluence/confluence_test.go`)
   - API client functionality
   - Document conversion
   - Markdown conversion
   - Image extraction

2. **Integration Tests** (`test/integration/confluence_flow_test.go`)
   - Full collection workflow
   - Auth integration
   - Storage integration
   - Mock Confluence responses

## Confluence Data Model

### Page Structure
```go
type Page struct {
    ID          string
    Title       string
    SpaceKey    string
    Content     string  // HTML content
    Version     int
    Author      string
    Created     time.Time
    Updated     time.Time
    ParentID    string
    Ancestors   []string
    Labels      []string
    Attachments []Attachment
}
```

### Document Conversion
```go
// Convert Page → Document
Document{
    ID:        fmt.Sprintf("confluence-%s", page.ID),
    Source:    "confluence",
    Title:     page.Title,
    ContentMD: convertToMarkdown(page.Content),
    Chunks:    chunkContent(contentMD),
    Images:    extractImages(page),
    Metadata: {
        "spaceKey": page.SpaceKey,
        "pageId":   page.ID,
        "author":   page.Author,
        "labels":   page.Labels,
        "url":      buildPageURL(page),
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
```

### Scraper Pattern
```go
type Scraper struct {
    browser *rod.Browser
    auth    *auth.AuthCredentials
    logger  *arbor.Logger
}

func NewScraper(auth *auth.AuthCredentials, logger *arbor.Logger) *Scraper

func (s *Scraper) ScrapePageContent(pageURL string) (*PageContent, error) {
    page := s.browser.MustPage(pageURL)

    // Inject cookies for authentication
    for _, cookie := range s.auth.Cookies {
        page.MustSetCookies(cookie)
    }

    page.MustWaitLoad()
    content := page.MustElement("#content").MustHTML()
    return &PageContent{HTML: content}, nil
}
```

### Source Implementation Pattern
```go
type ConfluenceSource struct {
    apiClient  *APIClient
    scraper    *Scraper
    processor  *Processor
    config     *ConfluenceConfig
    logger     *arbor.Logger
}

func NewConfluenceSource(auth *auth.AuthCredentials, config *ConfluenceConfig, logger *arbor.Logger) *ConfluenceSource

func (c *ConfluenceSource) Collect(ctx context.Context) ([]*models.Document, error) {
    var documents []*models.Document

    // Get spaces
    spaces := c.apiClient.GetSpaces()

    for _, space := range spaces {
        // Get pages in space
        pages := c.apiClient.GetPagesInSpace(space.Key)

        for _, page := range pages {
            // Scrape full content
            content := c.scraper.ScrapePageContent(page.URL)

            // Convert to document
            doc := c.processor.PageToDocument(page, content)
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

### Implement Confluence Collector
```
/quaero-confluence C:\development\quaero
```

### Add Custom Space Logic
```
/quaero-confluence C:\development\quaero --spaces=TEAM,DOCS,ENG
```

## Validation

After implementation, verifies:
- ✓ API client authenticates with Confluence
- ✓ Scraper captures full page content
- ✓ Processor converts to Document model
- ✓ Source interface implemented
- ✓ Images extracted and stored
- ✓ Markdown conversion working
- ✓ Content chunking functional
- ✓ Metadata properly extracted
- ✓ Unit tests passing
- ✓ Integration tests passing

## Output

Provides detailed report:
- Files created/modified
- API client implementation status
- Scraper functionality
- Document conversion pipeline
- Image handling implementation
- Tests created
- Sample documents generated

---

**Agent**: quaero-confluence

**Prompt**: Implement the Confluence data source collector for Quaero at {{args.[0]}}.

## Implementation Tasks

1. **Build API Client** (`internal/sources/confluence/api.go`)
   - REST API client with auth injection
   - Methods: GetSpaces, GetPagesInSpace, GetPage, GetAttachments
   - Pagination, rate limiting, error handling
   - Cookie and token injection from AuthManager

2. **Create Browser Scraper** (`internal/sources/confluence/scraper.go`)
   - rod-based scraper for JS-rendered content
   - Cookie injection for authentication
   - Screenshot capture
   - Diagram extraction
   - Dynamic content handling

3. **Implement Processor** (`internal/sources/confluence/processor.go`)
   - Page → Document conversion
   - HTML → Markdown conversion
   - Image extraction and storage
   - Content chunking for RAG
   - Metadata extraction (space, author, labels, URL)

4. **Create Source Implementation** (`internal/sources/confluence/confluence.go`)
   - Implement models.Source interface
   - Collection orchestration (API + Scraper + Processor)
   - Progress tracking and logging
   - Error handling and recovery

5. **Add Configuration**
   - ConfluenceConfig struct
   - Space filtering
   - Include/exclude patterns
   - Max pages limit

6. **Implement Image Handling**
   - Extract inline images
   - Download attachments
   - Extract diagrams (draw.io, Gliffy)
   - Save to data/images/
   - OCR for diagram text

7. **Create Tests**
   - Unit tests for API, scraper, processor
   - Integration test for full collection flow
   - Mock Confluence API responses
   - Test fixtures with sample pages

## Code Quality Standards

- Implements models.Source interface
- Uses auth.AuthCredentials from AuthManager
- Structured logging (arbor)
- Comprehensive error handling
- Thread-safe operations
- Proper resource cleanup (browser, HTTP clients)
- 80%+ test coverage

## Success Criteria

✓ API client retrieves Confluence pages
✓ Scraper captures full rendered content
✓ Pages converted to Document model
✓ Images extracted and stored
✓ Markdown conversion accurate
✓ Source interface fully implemented
✓ All tests passing
✓ Can collect from configured spaces
