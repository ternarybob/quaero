# Task 3: Add Test for Child Job Document Counts in UI
Depends: 2 | Critical: no | Model: sonnet

## Context
The UI should show document counts for each child job.
Currently:
- Parent job shows document count (but it's redundant and showing wrong value)
- Child jobs need to display their own document counts

## Do
1. Add test to verify child jobs display their document counts
2. Expand the parent to show child job rows
3. Check each child job row has a document count displayed
4. Verify the document counts are correct (not N/A, not 0)
5. Places child should show ~20 docs, Agent child should show ~4-20 docs (depending on processing)

## Accept
- [ ] Test verifies child job rows have document counts
- [ ] Test checks document count is not "N/A"
- [ ] Test checks document count > 0 for completed children
- [ ] Test should FAIL if child document counts are not displayed
