# Plan: Add API Tests for Search Endpoint

## Steps

1. **Create test file with basic search tests**
   - Skill: @test-writer
   - Files: `test/api/search_test.go` (new)
   - User decision: no
   - Description: Create TestSearchBasic with 5 subtests (EmptyDatabase, SingleDocument, MultipleDocuments, NoResults, EmptyQuery), reuse document helpers from documents_test.go

2. **Implement pagination tests**
   - Skill: @test-writer
   - Files: `test/api/search_test.go`
   - User decision: no
   - Description: Create TestSearchPagination with 5 subtests (DefaultPagination, CustomLimit, CustomOffset, LimitAndOffset, SecondPage) to verify pagination parameters

3. **Implement limit clamping tests**
   - Skill: @test-writer
   - Files: `test/api/search_test.go`
   - User decision: no
   - Description: Create TestSearchLimitClamping with 6 subtests to verify limit/offset validation and clamping (max 100, default 50, negative handling)

4. **Implement response structure tests**
   - Skill: @test-writer
   - Files: `test/api/search_test.go`
   - User decision: no
   - Description: Create TestSearchResponseStructure with 4 subtests to verify complete response format and field presence

5. **Implement brief truncation tests**
   - Skill: @test-writer
   - Files: `test/api/search_test.go`
   - User decision: no
   - Description: Create TestSearchBriefTruncation with 5 subtests to verify 200-char truncation logic with varying content lengths

6. **Implement error case tests**
   - Skill: @test-writer
   - Files: `test/api/search_test.go`
   - User decision: no
   - Description: Create TestSearchErrorCases with MethodNotAllowed subtest, add documentation comment about FTS5 disabled scenario limitation

7. **Run full test suite and verify**
   - Skill: @test-writer
   - Files: `test/api/search_test.go`
   - User decision: no
   - Description: Execute `go test -v -run TestSearch` to verify compilation and test execution, document results

## Success Criteria

- All 6 test functions implemented with comprehensive subtests (~26 total subtests)
- Tests reuse document helper functions from documents_test.go
- Tests compile cleanly without errors
- Clean state management with cleanup before/after ensures test isolation
- Response structure validated (results, count, query, limit, offset)
- Pagination, limit clamping, and brief truncation thoroughly tested
- Code follows Go testing conventions and project patterns
- FTS5 disabled scenario documented as limitation (requires config modification)
