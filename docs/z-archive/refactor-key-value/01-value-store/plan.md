# Plan: Value Store Infrastructure

## Steps

1. **Create database schema and interface definitions**
   - Skill: @code-architect
   - Files: `internal/storage/sqlite/schema.go`, `internal/interfaces/kv_storage.go`
   - User decision: no

2. **Implement SQLite storage layer**
   - Skill: @go-coder
   - Files: `internal/storage/sqlite/kv_storage.go`
   - User decision: no

3. **Create service layer**
   - Skill: @go-coder
   - Files: `internal/services/kv/service.go`
   - User decision: no

4. **Wire storage into manager**
   - Skill: @go-coder
   - Files: `internal/interfaces/storage.go`, `internal/storage/sqlite/manager.go`
   - User decision: no

5. **Wire service into app initialization**
   - Skill: @go-coder
   - Files: `internal/app/app.go`
   - User decision: no

6. **Create comprehensive unit tests**
   - Skill: @test-writer
   - Files: `internal/storage/sqlite/kv_storage_test.go`
   - User decision: no

## Success Criteria

- All files compile cleanly without errors
- Unit tests pass with good coverage
- Code follows existing patterns (DocumentStorage, AuthStorage)
- Storage manager properly wires KeyValueStorage
- App initializes KVService successfully
- No breaking changes to existing functionality
- Schema creates key_value_store table with proper indexes
- Mutex prevents SQLITE_BUSY errors on concurrent writes
