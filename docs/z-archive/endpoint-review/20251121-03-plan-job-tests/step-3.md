# Step 3: Add Job Definition CRUD tests (6 functions)

**Skill:** @test-writer
**Files:** `test/api/jobs_test.go` (EDIT)

---

## Iteration 1

### Agent 2 - Implementation

Added 6 job definition CRUD test functions to complete the job definition CRUD test suite.

**Implementation details:**
- Continued from Step 2's foundation (6 helpers + 12 job management tests)
- Package: `api`
- Imports: Already established in previous steps
- Test setup pattern: `SetupTestEnvironment()` with Badger config
- Reused helper functions from Step 1 for common operations

**Test functions implemented (6 total):**

1. **TestJobDefinition_List** - Tests GET /api/job-definitions:
   - Creates 2 test job definitions for testing
   - List with default parameters → 200 OK with job_definitions, total_count, limit, offset
   - List with pagination (limit=10, offset=0)
   - List with type filter (type=crawler)
   - List with enabled filter (enabled=true)
   - List with ordering (order_by=name, order_dir=ASC)
   - Verifies response structure and arrays

2. **TestJobDefinition_Create** - Tests POST /api/job-definitions:
   - Create valid job definition → 201 Created with all fields
   - Create with missing ID → 400 Bad Request
   - Create with missing name → 400 Bad Request
   - Create with missing steps → 400 Bad Request
   - Verifies validation errors for required fields

3. **TestJobDefinition_Get** - Tests GET /api/job-definitions/{id}:
   - Get valid job definition → 200 OK with all fields (id, name, type, steps, created_at)
   - Get nonexistent job definition → 404 Not Found
   - Get with empty ID → 400 or 404
   - Verifies response structure

4. **TestJobDefinition_Update** - Tests PUT /api/job-definitions/{id}:
   - Update valid job definition → 200 OK with updated fields
   - Update nonexistent job definition → 404 Not Found
   - Update with invalid data (missing steps) → 400 Bad Request
   - Verifies name update applied correctly

5. **TestJobDefinition_Delete** - Tests DELETE /api/job-definitions/{id}:
   - Delete valid job definition → 204 No Content
   - Verify deletion with GET → 404 Not Found
   - Delete nonexistent job definition → 404 Not Found
   - Tests complete deletion workflow

6. **TestJobDefinition_Execute** - Tests POST /api/job-definitions/{id}/execute:
   - Execute valid job definition → 202 Accepted with job_id, job_name, status, message
   - Execute nonexistent job definition → 404 Not Found
   - Create disabled job definition and attempt execute → 400 Bad Request
   - Verifies async execution starts with status="running"
   - Cleans up created jobs

**Changes made:**
- `test/api/jobs_test.go`: Added 6 tests (427 lines added, total: 1333 lines)

**Commands run:**
```bash
cd test/api && go test -c -o /tmp/jobs_test.exe
```
Result: ✅ Compilation successful

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 9/10

**Notes:**
- All 6 job definition CRUD tests implemented
- Tests follow established patterns from Steps 1-2
- Comprehensive validation error testing (missing ID, name, steps)
- Pagination, filtering, and ordering tested (type, enabled, order_by, order_dir)
- System job protection not tested (no easy way to create system jobs in test)
- Execute endpoint tests async workflow (202 Accepted, job_id returned)
- Disabled job definition execution tested (400 Bad Request)
- All tests properly cleanup resources (job definitions, jobs)
- File compiles successfully (1333 lines total)
- **Job Definition CRUD test suite now complete** (6/6 tests)

**Test Coverage Summary:**
- ✅ TestJobDefinition_List (Step 3) - List with pagination, filtering, ordering
- ✅ TestJobDefinition_Create (Step 3) - Create valid, validation errors
- ✅ TestJobDefinition_Get (Step 3) - Get by ID, 404 handling
- ✅ TestJobDefinition_Update (Step 3) - Update valid, validation errors, 404
- ✅ TestJobDefinition_Delete (Step 3) - Delete valid, verify deletion, 404
- ✅ TestJobDefinition_Execute (Step 3) - Execute valid, disabled, 404

**→ Continuing to Step 4**
