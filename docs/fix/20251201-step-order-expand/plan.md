# Plan: Fix Step Order and Child Job Expand
Type: fix | Workdir: ./docs/fix/20251201-step-order-expand/

## Tasks
| # | Desc | Depends | Critical | Model |
|---|------|---------|----------|-------|
| 1 | Sort steps by dependencies in ToJobDefinition | - | no | sonnet |
| 2 | Fetch children on expand click if not loaded | 1 | no | sonnet |
| 3 | Run tests and verify | 2 | no | sonnet |

## Order
[1] → [2] → [3]

## Analysis

### Problem 1: Step Order
The `ToJobDefinition()` function in `internal/jobs/service.go` iterates over `f.Step` map which is unordered in Go. Steps need to be sorted by their `depends` field to ensure execution order respects dependencies.

**Solution**: After parsing all steps from the map, sort them using topological sort based on the `depends` field.

### Problem 2: Child Jobs Don't Expand
When user clicks expand button, `toggleParentExpand()` only toggles state and calls `renderJobs()`. But if children aren't in `allJobs` yet, the filter `job => job.parent_id === parentJob.id` returns empty array.

**Solution**: Modify `toggleParentExpand()` to call `fetchChildrenForParent()` when expanding if children aren't already loaded.

### Problem 3: Test Verification
Run `TestNearbyRestaurantsKeywordsMultiStep` to verify fixes work.
