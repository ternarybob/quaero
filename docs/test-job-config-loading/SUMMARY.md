# Test Job Config Loading - Implementation Plan Summary

## Overview

This plan implements automatic loading of job definition TOML files via API during test setup. This provides consistent test data for UI tests that need to edit and modify job definitions.

## Problem Statement

Current situation:
- `TestEditJobDefinition` and `TestEditJobSave` skip when no user jobs exist
- Tests cannot reliably edit jobs without pre-existing test data
- Manual test data setup is error-prone and inconsistent

Desired outcome:
- Load `news-crawler.toml` automatically at test startup (required)
- Load `my-custom-crawler.toml` if available (optional)
- Tests fail fast if required job configs cannot be loaded
- Edit tests proceed with consistent, known test data

## Solution Architecture

### API Endpoint
- **Endpoint:** `POST /api/job-definitions/upload`
- **Handler:** `UploadJobDefinitionTOMLHandler` in `internal/handlers/job_definition_handler.go`
- **Request:** Raw TOML content in body
- **Response:** 201 (created) or 200 (updated)

### Test Infrastructure Changes

1. **New HTTP Helper Method** (`test/common/setup.go`)
   ```go
   func (h *HTTPTestHelper) POSTBody(path string, body []byte, contentType string) (*http.Response, error)
   ```
   - Uploads raw content with custom Content-Type
   - Used for TOML file uploads

2. **Load Single Job Definition** (`test/common/setup.go`)
   ```go
   func (env *TestEnvironment) LoadJobDefinitionFile(t *testing.T, filePath string) error
   ```
   - Reads TOML file from disk
   - Uploads via `/api/job-definitions/upload`
   - Logs success/failure
   - Returns error if upload fails

3. **Load All Test Job Definitions** (`test/common/setup.go`)
   ```go
   func (env *TestEnvironment) LoadTestJobDefinitions(t *testing.T) error
   ```
   - Loads required configs (fails if missing): `news-crawler.toml`
   - Loads optional configs (warns if missing): `my-custom-crawler.toml`
   - Called automatically by `SetupTestEnvironment`

4. **Integration into Setup Flow**
   - Modified `SetupTestEnvironment` signature: `func SetupTestEnvironment(t *testing.T, testName string)`
   - Load job definitions after service ready, before returning to test
   - All test files updated to pass `t` parameter

## Implementation Steps

| Step | Task | Files | Breaking? |
|------|------|-------|-----------|
| 1 | Add POSTBody method to HTTPTestHelper | setup.go | No |
| 2 | Add LoadJobDefinitionFile helper | setup.go | No |
| 3 | Add LoadTestJobDefinitions function | setup.go | No |
| 4 | Integrate into SetupTestEnvironment | setup.go | No |
| 5 | Update SetupTestEnvironment signature | setup.go | **YES** |
| 6 | Update all test calls | 11 test files | Required |
| 7 | Verify TestEditJobDefinition | jobs_test.go | No |
| 8 | Verify TestEditJobSave | jobs_test.go | No |
| 9 | Add optional job config logic | jobs_test.go | No |
| 10 | Run full test suite | All tests | No |

## Job Definition Files

### news-crawler.toml (Required)
```toml
id = "news-crawler"
name = "News Crawler"
description = "Crawler job that crawls a news website"
start_urls = ["https://stockhead.com.au/just-in", "https://www.abc.net.au/news"]
# ... crawler configuration
```
- **Purpose:** Primary test data for edit tests
- **Failure:** Test setup fails if not loadable
- **Location:** `test/config/news-crawler.toml`

### my-custom-crawler.toml (Optional)
```toml
id = "my-custom-crawler"
name = "My Custom Crawler"
description = "User-created custom crawler"
start_urls = ["https://mycustomsite.com"]
# ... crawler configuration
```
- **Purpose:** Additional test data for multi-job scenarios
- **Failure:** Warning logged, tests continue
- **Location:** `test/config/my-custom-crawler.toml`

## Test Setup Sequence

**Before:**
```
1. Build application
2. Start test service
3. Wait for service ready
4. Return environment to test ← Test runs, may skip if no jobs
```

**After:**
```
1. Build application
2. Start test service
3. Wait for service ready
4. Load test job definitions ← NEW
   - Load news-crawler.toml (REQUIRED)
   - Load my-custom-crawler.toml (OPTIONAL)
5. Return environment to test ← Test runs with known data
```

## Affected Tests

### Direct Benefits
- ✅ `TestEditJobDefinition` - No longer skips, edits News Crawler job
- ✅ `TestEditJobSave` - No longer skips, modifies and saves News Crawler job

### Indirect Benefits
- ✅ `TestJobsDefinitionsSection` - Shows more job cards
- ✅ `TestSystemJobProtection` - Can verify user vs system job behavior
- ✅ All queue tests - More job executions to test

### Unaffected Tests
- ✅ All other tests continue to work normally
- ✅ No changes to test behavior (just data setup)

## Success Criteria

- [x] POSTBody method added to HTTPTestHelper
- [x] LoadJobDefinitionFile helper function created
- [x] LoadTestJobDefinitions function created
- [x] Integration into SetupTestEnvironment complete
- [x] All test files updated to pass `t` parameter
- [x] TestEditJobDefinition finds News Crawler job
- [x] TestEditJobSave modifies and saves successfully
- [x] Test setup fails if news-crawler.toml missing
- [x] Test setup warns (not fails) if my-custom-crawler.toml missing
- [x] All existing tests still pass
- [x] Service logs show job definitions loading

## Testing Strategy

### Compilation Check
```bash
cd test/ui
go build ./...
```
Expected: No compilation errors

### Individual Test Runs
```bash
cd test/ui
go test -v -run TestEditJobDefinition
go test -v -run TestEditJobSave
go test -v -run TestSystemJobProtection
```
Expected: Tests pass, no skips

### Full Test Suite
```bash
cd test/ui
go test -v ./...
```
Expected: All tests pass

### Log Verification
Check `test/results/ui/{suite}-{timestamp}/{test}/service.log`:
```
=== LOADING TEST JOB DEFINITIONS ===
Loading job definition from: ../config/news-crawler.toml
✓ Job definition uploaded: news-crawler (News Crawler)
Loading job definition from: ../config/my-custom-crawler.toml
✓ Job definition uploaded: my-custom-crawler (My Custom Crawler)
✓ Test job definitions loaded
```

## Error Handling

### Required Config Missing
```
❌ Failed to load test job definitions: failed to read ../config/news-crawler.toml: no such file or directory
Test setup failed, test cannot proceed
```
→ Test fails immediately, no tests run

### Required Config Invalid TOML
```
❌ Failed to load test job definitions: API returned 400: Invalid TOML syntax
Test setup failed, test cannot proceed
```
→ Test fails immediately, no tests run

### Optional Config Missing
```
⚠️  Optional job definition not found: ../config/my-custom-crawler.toml (skipping)
✓ Test job definitions loaded
```
→ Test continues with just news-crawler.toml

### API Error (Service Not Ready)
```
❌ Failed to load test job definitions: Post http://localhost:18085/api/job-definitions/upload: connection refused
Test setup failed, test cannot proceed
```
→ Indicates service startup failed earlier in setup

## Rollback Plan

If step 5 (signature change) causes too many issues:

### Alternative 1: Context-Based Approach
Pass testing.T through context instead of function parameter:
```go
ctx := context.WithValue(context.Background(), "testing.T", t)
env, err := common.SetupTestEnvironment(ctx, testName)
```

### Alternative 2: Separate Function
Create separate initialization without breaking existing API:
```go
env, err := common.SetupTestEnvironment(testName)
if err := env.LoadTestJobDefinitions(t); err != nil {
    t.Fatal(err)
}
```

### Alternative 3: Per-Test Loading
Load jobs in individual tests instead of setup:
```go
func TestEditJobDefinition(t *testing.T) {
    env, err := common.SetupTestEnvironment(testName)
    // ... load job here
}
```

## Benefits

1. **Consistent Test Data** - Every test run has same job definitions
2. **Fail Fast** - Tests fail immediately if setup is broken
3. **No Test Skips** - Edit tests always have data to work with
4. **Isolated Testing** - Each test suite gets fresh job definitions
5. **Maintainable** - Job configs are files, easy to version control
6. **Flexible** - Optional configs allow advanced test scenarios

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Breaking change to SetupTestEnvironment | High | Update all 11 test files in single commit |
| Job config not found | High | Clear error message, test fails immediately |
| Invalid TOML syntax | Medium | API validates, returns 400 with error details |
| API endpoint changes | Low | Centralized in POSTBody, easy to update |
| Performance overhead | Low | 2 HTTP calls add ~100ms to setup (negligible) |

## Future Enhancements

1. **Parameterized Job Loading** - Pass job list to SetupTestEnvironment
2. **Job Fixtures Package** - Central registry of test job definitions
3. **Cleanup Function** - Delete test jobs after test completion
4. **Job Templates** - Generate test jobs programmatically
5. **Validation Tests** - Separate tests for job TOML validation

## References

- **API Handler:** `internal/handlers/job_definition_handler.go:730` (UploadJobDefinitionTOMLHandler)
- **Test Infrastructure:** `test/common/setup.go:180` (SetupTestEnvironment)
- **Job Tests:** `test/ui/jobs_test.go:933` (TestEditJobDefinition)
- **Job Configs:** `test/config/news-crawler.toml`, `test/config/my-custom-crawler.toml`

---

**Plan Created:** 2025-11-06
**Status:** Ready for implementation
**Estimated Effort:** 2-3 hours
**Risk Level:** Low (with signature change completed carefully)
