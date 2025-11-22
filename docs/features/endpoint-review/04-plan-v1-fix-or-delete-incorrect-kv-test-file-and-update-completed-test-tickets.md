I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The codebase contains a test file `test/api/kv_case_insensitive_test.go` that violates the established testing pattern defined in `test/api/health_check_test.go`. The problematic file directly instantiates SQLite databases and storage layers, bypassing the standard `common.SetupTestEnvironment()` infrastructure that uses Badger storage and HTTP-based testing via `HTTPTestHelper`.

**Key Issues Identified:**
1. **Direct SQLite Usage**: The file uses `sqlite.NewSQLiteDB()` and `sqlite.NewKVStorage()` directly, creating test databases with `setupTestDB()` helper
2. **Pattern Violation**: Does not follow the template pattern of using `SetupTestEnvironment()` with Badger config and `HTTPTestHelper` for HTTP requests
3. **Redundant Coverage**: All test cases in this file (case-insensitivity, upsert behavior, duplicate validation, API endpoint testing) are already covered in `test/api/settings_system_test.go` using the correct pattern (lines 81-517)
4. **SQLite Dependency**: Since SQLite is being removed in a subsequent phase, this file will break anyway
5. **No References**: Grep search confirmed no other files import or reference this test file

**Coverage Verification:**
`settings_system_test.go` already includes comprehensive KV tests following the correct pattern:
- KV CRUD lifecycle (lines 81-190)
- Case-insensitivity (lines 192-270)
- Upsert behavior (lines 272-327)
- Duplicate key validation (lines 329-372)
- Value masking (lines 374-445)
- Validation errors (lines 447-517)

All tests use `SetupTestEnvironment()` with Badger config and `HTTPTestHelper` for HTTP requests, matching the `health_check_test.go` template exactly.


### Approach

**Single-Step Solution: Delete the Incorrect Test File**

The approach is straightforward—delete `test/api/kv_case_insensitive_test.go` entirely since it:
1. Violates the established test pattern (direct DB access vs HTTP-based testing)
2. Uses SQLite directly (being removed in subsequent phase)
3. Provides no unique coverage (all scenarios already tested in `settings_system_test.go`)
4. Has no dependencies or references from other files

**Why This Works:**
- Zero impact on test coverage (all scenarios covered in `settings_system_test.go`)
- Aligns with the project's migration to Badger-only storage
- Enforces consistent test patterns across the codebase
- Prevents confusion for future developers about which pattern to follow

**Verification Steps:**
1. Delete the file
2. Run `go test ./test/api/...` to ensure all remaining tests pass
3. Verify test coverage remains comprehensive via test output

**Trade-offs:**
- None—this is purely removing redundant, incorrect code
- No functionality is lost since `settings_system_test.go` provides superior coverage using the correct pattern


### Reasoning

Started by listing the repository structure to understand the project layout. Read the three key files mentioned by the user: `kv_case_insensitive_test.go` (the problematic file), `health_check_test.go` (the template), and `setup.go` (the test infrastructure). Searched for any references to the problematic file across the codebase using grep—found none. Searched for and read `settings_system_test.go` to verify test coverage overlap, confirming all KV test scenarios are already covered using the correct pattern. Listed the test/api directory to see all test files and read `main_test.go` to understand the test suite structure.


## Proposed File Changes

### test\api\kv_case_insensitive_test.go(DELETE)

References: 

- test\api\health_check_test.go
- test\api\settings_system_test.go
- test\common\setup.go

Delete this entire file as it violates the established test pattern and is redundant.

**Reasons for Deletion:**
1. **Pattern Violation**: Uses direct SQLite database instantiation via `setupTestDB()` and `sqlite.NewSQLiteDB()`, bypassing the standard `common.SetupTestEnvironment()` infrastructure
2. **Wrong Storage Backend**: Uses SQLite directly when the project is migrating to Badger-only storage (SQLite removal is planned in subsequent phase)
3. **Incorrect Test Approach**: Tests storage layer directly instead of testing via HTTP endpoints using `HTTPTestHelper` as demonstrated in `test/api/health_check_test.go`
4. **Redundant Coverage**: All test scenarios in this file are already comprehensively covered in `test/api/settings_system_test.go` (lines 81-517) using the correct pattern:
   - Case-insensitive key handling (lines 192-270)
   - Upsert behavior (lines 272-327)
   - Duplicate key validation (lines 329-372)
   - API endpoint testing (lines 81-190)
   - Value masking (lines 374-445)
5. **No Dependencies**: Grep search confirmed no other files reference or import this test file

**Correct Pattern (from health_check_test.go):**
```go
env, err := common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")
require.NoError(t, err)
defer env.Cleanup()

helper := env.NewHTTPTestHelper(t)
resp, err := helper.GET("/api/endpoint")
helper.AssertStatusCode(resp, http.StatusOK)
```

All KV functionality is properly tested in `settings_system_test.go` following this pattern.