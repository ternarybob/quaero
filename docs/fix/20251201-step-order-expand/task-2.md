# Task 2: Fetch Children on Expand Click
Depends: 1 | Critical: no | Model: sonnet

## Do
1. Modify `toggleParentExpand()` in `pages/queue.html`
2. When expanding, check if children are already in `allJobs`
3. If not, call `fetchChildrenForParent()` before rendering
4. Show loading indicator while fetching

## Accept
- [ ] Clicking expand fetches children if not already loaded
- [ ] Child job rows appear after expand
- [ ] UI handles loading state gracefully
