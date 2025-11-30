# Plan: Fix Dual Steps UI - Child Jobs Display

## Classification
- Type: fix
- Workdir: ./docs/fix/20251201-dual-steps-ui/

## Analysis

### Problem
The current UI (screenshot 1) shows:
- Parent job with "Progress: 4 pending, 10 running, 6 completed" as text on same line
- Children are NOT listed as separate rows

### Desired State (screenshot 2 - reference)
- Parent shows overall status and document count
- Children listed on SEPARATE lines with their own:
  - Status badge (Completed, Running, Pending, Failed)
  - Progress info
  - Document counts

### Current Implementation
- `pages/queue.html:2010-2086` - `renderJobs()` method renders parent jobs and steps
- Steps are rendered but NOT child jobs
- The UI shows "Progress: X pending, Y running, Z completed" inline text instead of child rows

### Dependencies
- Alpine.js data binding in queue.html
- Queue Manager (`internal/queue/manager.go`)
- Job Definition model (`internal/models/job_definition.go`)

### Approach
1. Modify `renderJobs()` in queue.html to include child jobs as separate rows
2. Add child job data loading when parent is expanded
3. Add `document_filter_tags` field to JobStep config validation

### Risks
- Performance: Loading all children at once could be slow for large jobs
- UI complexity: Need to distinguish between steps and child jobs visually

## Groups
| Task | Desc | Depends | Critical | Complexity | Model |
|------|------|---------|----------|------------|-------|
| 1 | Add child job rows to UI template | none | no | medium | sonnet |
| 2 | Update renderJobs() to include children | 1 | no | medium | sonnet |
| 3 | Add document_filter_tags to job definition model | none | no | low | sonnet |
| 4 | Validate and test changes | 1,2,3 | no | low | sonnet |

## Order
Sequential: [1] -> [2] -> [3] -> [4]
