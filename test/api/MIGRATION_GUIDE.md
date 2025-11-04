# Migration Guide: Converting API Tests to Self-Contained Setup

## Overview

The new self-contained test setup provides:
- No external dependencies (no imports from `github.com/ternarybob/quaero/test`)
- Automatic service lifecycle management
- Results collected in `test/api/results/` with parent/child structure
- HTTP helper methods built into the TestEnvironment

## Migration Steps

### Before (Old Pattern - External Dependencies)
```go
package api

import (
    "testing"
    "github.com/ternarybob/quaero/test"  // ❌ External dependency
)

func TestOldPattern(t *testing.T) {
    h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

    resp, err := h.GET("/api/sources")
    // ... test code
}
```

### After (New Pattern - Self-Contained)
```go
package api

import (
    "testing"
    // No external test package import needed! ✅
)

func TestNewPattern(t *testing.T) {
    // Setup environment (builds & starts service)
    env, err := SetupTestEnvironment("TestNewPattern")
    if err != nil {
        t.Fatalf("Setup failed: %v", err)
    }
    defer env.Cleanup()

    // Create HTTP helper from environment
    h := env.NewHTTPTestHelper(t)

    resp, err := h.GET("/api/sources")
    // ... test code
}
```

## Complete Migration Example

### Before: sources_api_test.go (excerpt)
```go
package api

import (
    "net/http"
    "testing"
    "github.com/ternarybob/quaero/test"  // ❌
)

func TestListSources(t *testing.T) {
    h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

    resp, err := h.GET("/api/sources")
    if err != nil {
        t.Fatalf("Failed: %v", err)
    }

    h.AssertStatusCode(resp, http.StatusOK)
}
```

### After: sources_api_test.go (migrated)
```go
package api

import (
    "net/http"
    "testing"
    // No test package import! ✅
)

func TestListSources(t *testing.T) {
    env, err := SetupTestEnvironment("TestListSources")
    if err != nil {
        t.Fatalf("Setup failed: %v", err)
    }
    defer env.Cleanup()

    h := env.NewHTTPTestHelper(t)

    resp, err := h.GET("/api/sources")
    if err != nil {
        t.Fatalf("Failed: %v", err)
    }

    h.AssertStatusCode(resp, http.StatusOK)
}
```

## Changes Required

### 1. Remove External Import
```diff
- import "github.com/ternarybob/quaero/test"
```

### 2. Add Setup/Cleanup
```diff
  func TestMyAPI(t *testing.T) {
+     env, err := SetupTestEnvironment("TestMyAPI")
+     if err != nil {
+         t.Fatalf("Setup failed: %v", err)
+     }
+     defer env.Cleanup()
```

### 3. Update HTTP Helper Creation
```diff
-     h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())
+     h := env.NewHTTPTestHelper(t)
```

## Migration Checklist

For each test file:

- [ ] Remove `"github.com/ternarybob/quaero/test"` import
- [ ] Add `SetupTestEnvironment()` call at start of test
- [ ] Add `defer env.Cleanup()` after setup
- [ ] Change `test.NewHTTPTestHelper(...)` to `env.NewHTTPTestHelper(t)`
- [ ] Change `test.MustGetTestServerURL()` to `env.GetBaseURL()`
- [ ] Remove any `LoadTestConfig()` calls (use SetupTestEnvironment instead)
- [ ] Test the migrated function: `go test -v -run TestMyFunc`

## Files To Migrate

All test files currently importing the external `test` package:

1. `auth_api_test.go`
2. `chat_api_test.go`
3. `config_api_test.go`
4. `job_api_test.go`
5. `job_completion_test.go` - **Special case**: uses `test.NewMockServer`
6. `job_definition_execution_test.go`
7. `job_deletion_test.go`
8. `job_logs_aggregated_test.go`
9. `job_rerun_test.go`
10. `markdown_storage_test.go`
11. `search_api_test.go`
12. `sources_api_test.go`

## Special Cases

### Tests Using LoadTestConfig (In-Memory Tests)
If a test uses `LoadTestConfig(t)` from test_fixtures.go for in-memory unit tests:

```go
// Old:
config, cleanup := LoadTestConfig(t)

// New:
config, cleanup := CreateInMemoryTestConfig(t)
```

Note: `LoadTestConfig` was renamed to `CreateInMemoryTestConfig` to avoid conflict with setup.go.

### Tests Using Mock Server
`job_completion_test.go` uses `test.NewMockServer()` which is not available in the self-contained setup. Options:

1. **Add mock server to setup.go** (recommended for commonly needed test infrastructure)
2. **Create httptest.Server inline** in the test
3. **Skip migration** if test is deprecated (see note in test file)

Example inline mock server:
```go
mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("<html>Test Content</html>"))
}))
defer mockServer.Close()
```

## Benefits After Migration

- ✅ **Self-contained**: No dependencies outside test/api
- ✅ **Isolated**: Each test gets its own service instance and results directory
- ✅ **Debuggable**: All logs saved to `test/api/results/{Suite}-{datetime}/{TestName}/`
- ✅ **Consistent**: Same setup pattern across all tests
- ✅ **Maintainable**: Changes to test infrastructure only affect test/api

## Testing Your Migration

After migrating a test:

```powershell
cd test/api
go test -v -run TestYourMigratedTest
```

Check the results directory:
```powershell
ls test/api/results/
```

You should see:
```
TestYourTest-20250104-150405/
├── TestYourMigratedTest/
    ├── service.log    # Service output
    └── test.log       # Test execution log
```

## Need Help?

See `example_api_test.go` for complete working examples of:
- Simple GET request test
- CREATE with resource cleanup
- Response parsing and assertions
- Proper use of SetupTestEnvironment

## Next Steps

1. Start with simple tests (e.g., `TestListSources`)
2. Migrate one file at a time
3. Run tests after each migration
4. Check `test/api/results/` for debugging
5. Update this guide with any edge cases you encounter
