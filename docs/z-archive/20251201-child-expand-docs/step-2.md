# Step 2: Debug and Fix Child Expand Functionality
Model: sonnet | Status: ✅

## Done
- Fixed Alpine.js reactivity issue in `toggleParentExpand()` function
- Changed from direct property assignment to object spread/reassignment pattern

## Root Cause
Alpine.js doesn't reliably detect changes to nested properties in objects. When `this.expandedParents[parentId] = true` was set, Alpine's reactivity system didn't detect the change because only a new property was added to an existing object reference.

## Fix Applied
Changed from:
```javascript
this.expandedParents[parentId] = !this.expandedParents[parentId];
```

To:
```javascript
const newState = !this.expandedParents[parentId];
this.expandedParents = { ...this.expandedParents, [parentId]: newState };
```

This creates a new object reference, which Alpine.js correctly detects as a change.

## Files Changed
- `pages/queue.html` - Modified `toggleParentExpand()` function (lines 1920-1927)
  - Now uses object spread to reassign the entire `expandedParents` object
  - Triggers proper Alpine.js reactivity for re-rendering

## Verify
Build: pending | Tests: pending
