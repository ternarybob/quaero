# quaero-auth

Implements and manages the authentication system for Quaero, handling auth flow from browser extension to collectors.

## Usage

```
/quaero-auth <quaero-project-path>
```

## Arguments

- `quaero-project-path` (required): Path to Quaero monorepo

## What it does

### Phase 1: Auth Manager Implementation

1. **Auth Data Structures** (`internal/auth/types.go`)
   - `ExtensionAuthData` - Data received from extension
   - `AuthCredentials` - Stored credentials
   - `Cookie` - Cookie representation
   - `Tokens` - Token collection (atlToken, cloudId, etc.)

2. **Auth Manager** (`internal/auth/manager.go`)
   - `Manager` struct with credential storage
   - `StoreAuth(source string, data *ExtensionAuthData)` - Store credentials
   - `GetAuth(source string) *AuthCredentials` - Retrieve credentials
   - `RefreshAuth(source string)` - Handle refresh
   - `IsValid(source string) bool` - Check validity
   - Secure credential storage
   - Thread-safe operations

3. **Auth Store** (`internal/auth/store.go`)
   - Persistent storage of credentials
   - Encryption at rest
   - Credential rotation support
   - Expiry tracking

### Phase 2: HTTP Handler Implementation

1. **Auth Handler** (`internal/auth/handler.go`)
   - HTTP endpoint handler for `/api/auth`
   - Receives ExtensionAuthData from browser extension
   - Validates incoming auth data
   - Stores credentials via AuthManager
   - Triggers collection on successful auth
   - Returns appropriate HTTP responses

2. **Request Processing**
   - JSON deserialization of extension payload
   - Cookie extraction and validation
   - Token parsing and storage
   - baseURL validation
   - User-Agent capture

3. **Response Handling**
   - Success confirmation to extension
   - Error responses with details
   - CORS headers for extension origin
   - Logging auth events

### Phase 3: Auth Integration

1. **Source Integration**
   - Provides auth to Confluence collector
   - Provides auth to Jira collector
   - Provides auth to any authenticated source
   - Handles auth refresh across sources

2. **HTTP Client Integration**
   - Cookie injection for requests
   - Token header injection
   - User-Agent propagation
   - Auth error handling and refresh

### Phase 4: Extension Communication

1. **Extension Protocol**
   - Defines JSON structure for auth data
   - Endpoint specification (/api/auth)
   - Response format specification
   - Error handling protocol

2. **Auth Refresh Flow**
   - Handles periodic refresh from extension (every 30 min)
   - Updates stored credentials
   - Notifies collectors of refresh
   - Manages credential versioning

### Phase 5: Security Implementation

1. **Credential Security**
   - Encryption of stored credentials
   - Secure memory handling
   - Credential rotation
   - Access control

2. **Validation**
   - Origin validation
   - Payload validation
   - Credential expiry checking
   - Token verification

### Phase 6: Testing

1. **Unit Tests** (`internal/auth/manager_test.go`, `handler_test.go`)
   - Auth storage and retrieval
   - Credential validation
   - HTTP handler behavior
   - Security measures

2. **Integration Tests** (`test/integration/auth_flow_test.go`)
   - Extension → Server flow
   - Auth refresh flow
   - Collector auth usage
   - Error scenarios

## Authentication Flow

```
1. User logs into Jira/Confluence (handles 2FA, SSO)
   ↓
2. Extension extracts complete auth state:
   • All cookies (.atlassian.net)
   • localStorage tokens
   • sessionStorage tokens
   • cloudId, atl_token
   • User agent
   ↓
3. Extension POSTs to http://localhost:8080/api/auth
   {
     "cookies": [...],
     "tokens": {...},
     "baseUrl": "https://company.atlassian.net"
   }
   ↓
4. AuthManager stores credentials securely
   ↓
5. Collectors retrieve auth via GetAuth(source)
   ↓
6. Extension refreshes every 30 minutes
```

## Code Structure

### Manager Pattern
```go
type Manager struct {
    store      *Store
    logger     *arbor.Logger
    mu         sync.RWMutex
    credentials map[string]*AuthCredentials
}

func NewManager(store *Store, logger *arbor.Logger) *Manager

func (m *Manager) StoreAuth(source string, data *ExtensionAuthData) error
func (m *Manager) GetAuth(source string) *AuthCredentials
func (m *Manager) RefreshAuth(source string, data *ExtensionAuthData) error
func (m *Manager) IsValid(source string) bool
```

### Handler Pattern
```go
type Handler struct {
    manager    *Manager
    orchestrator *collector.Orchestrator
    logger     *arbor.Logger
}

func NewHandler(manager *Manager, orchestrator *collector.Orchestrator, logger *arbor.Logger) *Handler

func (h *Handler) HandleAuth(w http.ResponseWriter, r *http.Request)
func (h *Handler) validateRequest(r *http.Request) error
func (h *Handler) parseAuthData(r *http.Request) (*ExtensionAuthData, error)
func (h *Handler) triggerCollection(source string)
```

### Usage in Collectors
```go
// Confluence collector
type APIClient struct {
    auth    *auth.AuthCredentials
    baseURL string
}

func (c *APIClient) makeRequest(endpoint string) (*http.Response, error) {
    req, _ := http.NewRequest("GET", c.baseURL+endpoint, nil)

    // Add cookies from extension
    for _, cookie := range c.auth.Cookies {
        req.AddCookie(cookie)
    }

    // Add tokens
    if c.auth.Tokens.AtlToken != "" {
        req.Header.Set("X-Atlassian-Token", c.auth.Tokens.AtlToken)
    }

    return http.DefaultClient.Do(req)
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

### Implement Auth System
```
/quaero-auth C:\development\quaero
```

### Extend Auth for New Source
```
/quaero-auth C:\development\quaero --source=slack
```

## Validation

After implementation, verifies:
- ✓ AuthManager implemented with thread-safe operations
- ✓ Auth Handler receives and processes extension data
- ✓ Credentials stored securely
- ✓ Auth refresh mechanism working
- ✓ Collectors can retrieve credentials
- ✓ HTTP endpoint functional (/api/auth)
- ✓ Unit tests passing
- ✓ Integration tests passing
- ✓ Security measures in place

## Output

Provides detailed report:
- Files created/modified
- Auth flow components implemented
- Security features enabled
- Tests created
- Integration points ready
- Extension communication protocol defined

---

**Agent**: quaero-auth

**Prompt**: Implement the authentication management system for Quaero at {{args.[0]}}.

## Implementation Tasks

1. **Define Auth Data Structures** (`internal/auth/types.go`)
   - ExtensionAuthData (cookies, tokens, baseUrl, userAgent)
   - AuthCredentials (processed credentials for storage)
   - Cookie, Tokens structs
   - Support for Atlassian-specific auth (cloudId, atl_token)

2. **Implement Auth Manager** (`internal/auth/manager.go`)
   - Thread-safe credential storage (sync.RWMutex)
   - StoreAuth, GetAuth, RefreshAuth, IsValid methods
   - Secure credential handling
   - Expiry tracking and validation
   - Integration with Store for persistence

3. **Create Auth Store** (`internal/auth/store.go`)
   - Persistent storage layer
   - Encryption at rest
   - Credential rotation
   - File-based or secure storage backend

4. **Build HTTP Handler** (`internal/auth/handler.go`)
   - `/api/auth` POST endpoint
   - Receive ExtensionAuthData JSON
   - Validate and store credentials
   - Trigger collection orchestrator
   - Error handling and logging
   - CORS support for extension

5. **Integration Points**
   - Provide credentials to Confluence collector
   - Provide credentials to Jira collector
   - HTTP client auth injection helpers
   - Refresh notification system

6. **Security Implementation**
   - Credential encryption
   - Secure memory handling
   - Origin validation
   - Access control

7. **Testing**
   - Unit tests for Manager and Handler
   - Integration test for auth flow
   - Mock extension requests
   - Test auth refresh scenarios

## Code Quality Standards

- Thread-safe operations (sync.RWMutex)
- Comprehensive error handling
- Secure credential storage
- Structured logging (arbor)
- Interface-based design
- Dependency injection
- 80%+ test coverage

## Success Criteria

✓ Extension can POST auth to /api/auth
✓ Credentials stored securely
✓ Collectors retrieve auth successfully
✓ Auth refresh works every 30 min
✓ Thread-safe concurrent access
✓ All tests passing
✓ Security measures validated
