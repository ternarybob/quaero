# Plan: Add API Tests for Documents Endpoints

## Steps

1. **Create helper functions for document test setup and cleanup**
   - Skill: @test-writer
   - Files: `test/api/documents_test.go` (new)
   - User decision: no
   - Description: Implement helper functions following patterns from `test/api/auth_test.go`: createTestDocument(), createTestDocumentWithMetadata(), createAndSaveTestDocument(), deleteTestDocument(), cleanupAllDocuments()

2. **Implement GET /api/documents (List) tests with pagination and filtering**
   - Skill: @test-writer
   - Files: `test/api/documents_test.go`
   - User decision: no
   - Description: Create TestDocumentsList with 11 subtests covering empty list, single/multiple documents, pagination (limit/offset), filtering (source_type, tags, dates), ordering, and combined filters

3. **Implement POST /api/documents (Create) tests**
   - Skill: @test-writer
   - Files: `test/api/documents_test.go`
   - User decision: no
   - Description: Create TestDocumentsCreate with 9 subtests covering success scenarios (basic, with metadata, with tags), error cases (invalid JSON, missing/empty required fields), and duplicate ID handling

4. **Implement GET /api/documents/{id} (Get) tests**
   - Skill: @test-writer
   - Files: `test/api/documents_test.go`
   - User decision: no
   - Description: Create TestDocumentsGet with 4 subtests covering success (basic and complex metadata), not found, and empty ID scenarios

5. **Implement DELETE /api/documents/{id} (Delete) tests**
   - Skill: @test-writer
   - Files: `test/api/documents_test.go`
   - User decision: no
   - Description: Create TestDocumentsDelete with 4 subtests covering success, not found (500 error), empty ID, and multiple deletes scenarios

6. **Implement POST /api/documents/{id}/reprocess (Reprocess) tests**
   - Skill: @test-writer
   - Files: `test/api/documents_test.go`
   - User decision: no
   - Description: Create TestDocumentsReprocess with 3 subtests covering success (no-op endpoint), not found (400 error), and empty ID

7. **Implement GET /api/documents/stats (Stats) tests**
   - Skill: @test-writer
   - Files: `test/api/documents_test.go`
   - User decision: no
   - Description: Create TestDocumentsStats with 4 subtests covering empty database, single document, multiple source types breakdown, and stats fields verification

8. **Implement GET /api/documents/tags (Tags) tests**
   - Skill: @test-writer
   - Files: `test/api/documents_test.go`
   - User decision: no
   - Description: Create TestDocumentsTags with 5 subtests covering empty database, single tag, multiple tags, duplicate tags deduplication, and no tags scenarios

9. **Implement DELETE /api/documents/clear-all (DeleteAll) tests**
   - Skill: @test-writer
   - Files: `test/api/documents_test.go`
   - User decision: no
   - Description: Create TestDocumentsClearAll with 3 subtests covering success with count, empty database, and verification of complete deletion

10. **Implement comprehensive document lifecycle test**
    - Skill: @test-writer
    - Files: `test/api/documents_test.go`
    - User decision: no
    - Description: Create TestDocumentLifecycle covering end-to-end flow: create → get → list → stats → tags → reprocess → delete → verify deletion

11. **Run full test suite and verify all tests compile and execute**
    - Skill: @test-writer
    - Files: `test/api/documents_test.go`
    - User decision: no
    - Description: Execute `go test -v -run TestDocuments` and verify compilation, document test results, identify any issues

## Success Criteria

- All 11 test functions implemented with comprehensive subtests (45+ total subtests)
- Helper functions follow patterns from `test/api/auth_test.go`
- All 8 document endpoints covered with positive and negative test cases
- Pagination, filtering, and ordering thoroughly tested
- Error cases validated (invalid JSON, missing fields, not found)
- Tests compile cleanly without errors
- Clean state management with cleanup before/after ensures test isolation
- Code follows Go testing conventions and project patterns
