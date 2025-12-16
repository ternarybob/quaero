# ARCHITECT ANALYSIS: UI Test File Consolidation

## Task
1. Revise `test/ui/job_core_test.go` to focus on job page functions, avoiding overlap with `index_test.go`
2. Combine `index_test.go` and `main_test.go` into `index_test.go`, removing duplicates

## Current State Analysis

### Files Involved

| File | Purpose | Lines | Tests |
|------|---------|-------|-------|
| `main_test.go` | TestMain setup, service connectivity, cleanup | 119 | 0 (setup only) |
| `index_test.go` | Index page tests (favicon, navbar, SSE, footer) | 267 | 1 (TestIndex) |
| `job_core_test.go` | Basic page loads, navigation, jobs/queue checks | 151 | 4 tests |

### Overlap Analysis

#### `main_test.go` vs `index_test.go`
- `main_test.go` contains ONLY infrastructure (TestMain, verifyServiceConnectivity, cleanup)
- `index_test.go` contains ONLY TestIndex test
- **NO overlap** - these serve different purposes

#### `job_core_test.go` vs `index_test.go`

| Feature | index_test.go | job_core_test.go | Overlap? |
|---------|--------------|------------------|----------|
| Page load check | ✓ (index only) | ✓ (all pages) | YES - both test page loads |
| Navbar verification | ✓ (link text) | - | NO |
| SSE connection | ✓ | - | NO |
| Service logs | ✓ | - | NO |
| Footer version | ✓ | - | NO |
| Jobs page content | - | ✓ | NO |
| Queue page content | - | ✓ | NO |
| Navigation between pages | - | ✓ | NO |

### Duplication Found

1. **`TestPagesLoad` in job_core_test.go** (lines 14-48) tests page loads for:
   - Home page - DUPLICATES index_test.go's navigation test
   - Jobs, Queue, Documents, Settings - NOT duplicated

2. **Browser setup pattern** - Both files create browser contexts independently
   - `main_test.go` uses its own browser setup in `verifyServiceConnectivity`
   - `index_test.go` uses its own browser setup
   - `job_core_test.go` uses `UITestContext` framework

---

## Recommendation

### Action 1: Combine `main_test.go` INTO `index_test.go`

**Rationale:**
- `main_test.go` contains `TestMain()` which is required infrastructure for the package
- `TestMain()` must stay in the `ui` package - it can live in `index_test.go`
- `index_test.go` is logically the "entry point" test file
- Moving `TestMain` + helpers into `index_test.go` consolidates setup with the first test

**Changes:**
1. Move `TestMain()`, `cleanupAllResources()`, `verifyServiceConnectivity()` to `index_test.go`
2. Add necessary imports (`io`, `net/http`, `github.com/ternarybob/quaero/test`)
3. Delete `main_test.go`

### Action 2: Revise `job_core_test.go` - Remove Home Page Overlap

**Rationale:**
- `TestPagesLoad` tests all pages including Home
- Index page load is already tested in `TestIndex`
- `job_core_test.go` should focus on Jobs/Queue functionality

**Changes:**
1. Remove "Home" from the pages list in `TestPagesLoad`
2. Rename test to `TestJobRelatedPagesLoad` for clarity
3. Keep `TestJobsPageShowsJobs`, `TestQueuePageShowsQueue`, `TestNavigationBetweenPages` as-is

---

## Implementation Order

1. **Step 1**: Modify `index_test.go` - Add TestMain and related functions
2. **Step 2**: Delete `main_test.go`
3. **Step 3**: Modify `job_core_test.go` - Remove Home page test, rename function
4. **Step 4**: Run build and tests to verify

## Risk Assessment

- **LOW RISK**: These are test-only changes
- No production code affected
- Tests remain functionally equivalent after consolidation
