I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current Implementation Analysis

**Child Jobs Display (lines 262-302 in `pages/queue.html`):**
- Flat list rendering with no visual hierarchy
- Each child shows: job type badge, status badge, URL, depth number
- Pagination via "Load More" button (25 items per page)
- Child data structure includes: `id`, `url`, `depth`, `status`, `timestamp`, `job_type`

**Alpine.js State Management (lines 1278-1577):**
- `expandedParents: Set` - Tracks which parent jobs are expanded
- `childJobsList: Map<parentId, childMeta[]>` - Stores child job metadata
- `childJobsVisibleCount: Map<parentId, number>` - Pagination state per parent
- `childJobsPageSize: 25` - Items per page
- Functions: `loadChildJobs()`, `getVisibleChildJobs()`, `loadMoreChildJobs()`, `toggleParentExpansion()`, `handleChildSpawned()`, `handleChildJobStatus()`

**Existing CSS Infrastructure (`quaero.css` lines 817-1071):**
- `.child-jobs-list-container`, `.child-jobs-list-header`, `.child-jobs-list`
- `.child-job-item` with flexbox layout
- `.child-job-url`, `.child-job-depth` styling
- Scrollbar customization for child lists
- Basic tree-view classes (`.job-card-parent`, `.job-card-child`)

**Job Type Utilities (`common.js` lines 239-274):**
- `getJobTypeBadgeClass()` - Maps job types to badge colors
- `getJobTypeIcon()` - Maps job types to Font Awesome icons
- `getJobTypeDisplayName()` - Human-readable names
- Exposed globally via `window` object

**WebSocket Integration:**
- `handleChildSpawned()` adds new children to the list in real-time
- `handleChildJobStatus()` updates child status when jobs complete/fail
- `updateJobInList()` handles parent job updates

## GitHub Actions Tree View Patterns (from web search)

**Key Design Elements:**
1. **Collapsible sections** - Jobs → steps as nested tree with expand/collapse
2. **Progressive disclosure** - Default collapsed, fast local expand/collapse
3. **Visual hierarchy** - Indentation and connecting lines show parent-child relationships
4. **Status indicators** - Icons and colors for pending/running/completed/failed states
5. **Inline metadata** - Timestamps, durations, and context shown inline
6. **Keyboard navigation** - Arrow keys for traversal, Enter to expand/collapse
7. **Annotations surfaced** - Errors/warnings highlighted above log sections
8. **Streaming updates** - Real-time status changes without full refresh

## Design Decisions

**Tree Structure Approach:**
- Use **flat DOM with CSS indentation** (not nested divs) for better performance
- Calculate indentation based on `depth` property: `padding-left: calc(depth * 1.5rem)`
- Maintain existing pagination to handle 200+ children efficiently

**Collapsible Nodes:**
- Add `collapsedDepths: Map<parentId, Set<depth>>` to track collapsed depth levels
- Only depth 0 children are collapsible (they may have depth 1+ children)
- Use chevron icons (fa-chevron-right/down) for expand/collapse affordance

**Status Visualization:**
- Replace status badges with **icon + text** layout similar to GitHub
- Use Font Awesome icons: `fa-circle` (pending), `fa-spinner` (running), `fa-check-circle` (completed), `fa-times-circle` (failed)
- Color-code icons using existing CSS variables: `--color-success`, `--color-danger`, `--color-primary`, `--text-secondary`

**Accessibility:**
- Add ARIA attributes: `role="tree"`, `role="treeitem"`, `aria-expanded`, `aria-level`
- Keyboard navigation: Arrow keys (↑↓) for traversal, (←→) for collapse/expand, Enter for toggle
- Focus management with `tabindex` and visual focus indicators

**Integration with Existing Features:**
- Preserve "Load More" pagination (works at tree level, not per depth)
- Maintain WebSocket real-time updates for status changes
- Keep existing `expandedParents` for parent job expansion
- No backend changes required - all data already available

### Approach

**Three-Phase Transformation:**

**Phase 1 - CSS Tree Styling:** Add GitHub Actions-inspired tree styles with depth-based indentation, connecting lines, status icons, and collapsible node indicators. Extend existing `.child-job-item` classes with tree-specific variants.

**Phase 2 - HTML Template Redesign:** Transform flat list into tree structure with proper ARIA attributes, status icons replacing badges, depth-based indentation, and collapsible node controls. Maintain pagination structure.

**Phase 3 - Alpine.js Tree Logic:** Add tree expansion state management, keyboard navigation handlers, depth-based filtering for collapsed nodes, and update WebSocket handlers to work with tree structure. Preserve existing pagination and real-time update mechanisms.

### Reasoning

I explored the queue management UI implementation by reading `pages/queue.html` (child jobs rendering at lines 262-302, Alpine.js component at lines 1278-1577), examined the CSS infrastructure in `pages/static/quaero.css` (lines 817-1071), reviewed job type utilities in `pages/static/common.js` (lines 239-274), and researched GitHub Actions tree view patterns via web search. I identified the current flat list structure, pagination mechanism, WebSocket integration points, and existing CSS classes that can be extended for tree visualization.

## Mermaid Diagram

sequenceDiagram
    participant User
    participant UI as Queue UI (Alpine.js)
    participant State as Tree State (Maps/Sets)
    participant DOM as DOM Tree Items
    
    Note over User,DOM: Initial Load - Parent Job Expanded
    
    User->>UI: Click expand parent job
    UI->>UI: toggleParentExpansion(parentId)
    UI->>State: expandedParents.add(parentId)
    UI->>UI: loadChildJobs(parentId)
    UI-->>DOM: Render tree items (depth 0, 1, 2...)
    DOM-->>User: Display tree with indentation
    
    Note over User,DOM: Tree Interaction - Collapse Depth Level
    
    User->>DOM: Click chevron on depth 0 item
    DOM->>UI: toggleDepthCollapse(parentId, 0)
    UI->>State: collapsedDepths.get(parentId).add(0)
    UI->>UI: renderJobs()
    UI->>DOM: Apply .tree-item-collapsed to depth 1+ items
    DOM-->>User: Hide depth 1+ items (display: none)
    
    Note over User,DOM: Keyboard Navigation
    
    User->>DOM: Press ArrowDown key
    DOM->>UI: handleTreeKeydown(event, parentId, child)
    UI->>UI: getVisibleTreeItems(parentId)
    UI->>State: Filter by isChildCollapsed()
    UI->>DOM: focusTreeItem(nextChildId)
    DOM-->>User: Focus moves to next visible item
    
    Note over User,DOM: WebSocket Real-time Update
    
    Note over UI: Child job status changes: pending → running
    UI->>UI: handleChildJobStatus(jobId, 'running')
    UI->>State: Update child.status in childJobsList
    UI->>UI: renderJobs()
    UI->>DOM: Update icon class to .status-running
    DOM-->>User: Icon changes to spinner (fa-pulse)
    
    Note over User,DOM: Load More Pagination
    
    User->>DOM: Click "Load More" button
    DOM->>UI: loadMoreChildJobs(parentId)
    UI->>UI: Increase childJobsVisibleCount
    UI->>UI: renderJobs()
    UI->>DOM: Render additional 25 tree items
    DOM-->>User: Display more children in tree
    
    Note over User,DOM: Accessibility - Screen Reader
    
    User->>DOM: Tab to tree item (keyboard focus)
    DOM->>UI: Focus event on tree item
    UI->>DOM: Apply :focus styles (outline)
    DOM-->>User: Visual focus indicator
    User->>DOM: Press Enter to expand
    DOM->>UI: handleTreeKeydown('Enter')
    UI->>State: Toggle collapsed state
    UI->>DOM: Update aria-expanded attribute
    DOM-->>User: Screen reader announces "Expanded"

## Proposed File Changes

### pages\static\quaero.css(MODIFY)

**Add GitHub Actions-style tree view CSS (after line 1071):**

**1. Tree Container Styles:**
- Add `.child-jobs-tree` class to replace `.child-jobs-list`
- Remove max-height and scrolling (pagination handles overflow)
- Add `role="tree"` styling with proper spacing

**2. Tree Item Base Styles:**
- Extend `.child-job-item` with tree-specific layout
- Add depth-based indentation: `padding-left: calc(var(--depth) * 1.5rem)`
- Use CSS custom property `--depth` set via inline style
- Add hover state with subtle background color change
- Add focus state with outline for keyboard navigation

**3. Tree Connector Lines (GitHub-style):**
- Add `.tree-connector` pseudo-element for vertical lines
- Use `::before` for vertical line connecting siblings
- Use `::after` for horizontal line to item
- Color: `var(--border-color)` with 1px solid border
- Position absolutely relative to tree item

**4. Collapsible Node Styles:**
- Add `.tree-node-toggle` button class
- Inline button before item content
- Chevron icon rotation: 0deg (collapsed) → 90deg (expanded)
- Transition: `transform 0.2s ease`
- Size: 1rem × 1rem, centered alignment

**5. Status Icon Styles:**
- Add `.tree-status-icon` class for status indicators
- Icon size: 0.875rem (14px)
- Margin-right: 0.5rem
- Color mapping:
  - `.status-pending`: `var(--text-secondary)` with `fa-circle`
  - `.status-running`: `var(--color-primary)` with `fa-spinner fa-pulse`
  - `.status-completed`: `var(--color-success)` with `fa-check-circle`
  - `.status-failed`: `var(--color-danger)` with `fa-times-circle`
  - `.status-cancelled`: `var(--text-secondary)` with `fa-ban`

**6. Tree Item Content Layout:**
- Add `.tree-item-content` wrapper class
- Flexbox layout: `display: flex; align-items: center; gap: 0.5rem;`
- Allow text overflow with ellipsis for long URLs
- Add `.tree-item-url` class (replaces `.child-job-url`)
- Add `.tree-item-meta` class for depth/timestamp info

**7. Collapsed State Styles:**
- Add `.tree-item-collapsed` class for hidden items
- Use `display: none` (not visibility) for performance
- Add fade-in animation when expanding: `@keyframes treeItemFadeIn`

**8. Keyboard Navigation Styles:**
- Add `.tree-item:focus` with visible outline
- Add `.tree-item:focus-visible` for keyboard-only focus
- Outline: `2px solid var(--color-primary)` with `outline-offset: 2px`
- Ensure focus is visible but not intrusive

**9. Load More Button in Tree Context:**
- Update `.load-more-container` to work with tree layout
- Center align with proper spacing
- Maintain existing button styles

**10. Responsive Adjustments:**
- Reduce indentation on mobile: `calc(var(--depth) * 1rem)` at max-width 768px
- Hide connector lines on very small screens (< 480px)
- Adjust font sizes for better readability

**11. Accessibility Enhancements:**
- Add high contrast mode support with `@media (prefers-contrast: high)`
- Ensure color contrast ratios meet WCAG AA standards
- Add focus indicators that work in both light and dark modes

**Example CSS structure:**
```css
/* Tree container */
.child-jobs-tree {
  padding: 0.5rem;
  position: relative;
}

/* Tree item with depth-based indentation */
.tree-item {
  display: flex;
  align-items: center;
  padding: 0.4rem 0.5rem;
  padding-left: calc(var(--depth, 0) * 1.5rem + 0.5rem);
  margin-bottom: 0.25rem;
  border-radius: var(--border-radius);
  transition: background-color 0.15s ease;
  position: relative;
}

.tree-item:hover {
  background-color: rgba(0, 0, 0, 0.03);
}

.tree-item:focus {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}

/* Status icons */
.tree-status-icon {
  font-size: 0.875rem;
  margin-right: 0.5rem;
  flex-shrink: 0;
}

.tree-status-icon.status-pending { color: var(--text-secondary); }
.tree-status-icon.status-running { color: var(--color-primary); }
.tree-status-icon.status-completed { color: var(--color-success); }
.tree-status-icon.status-failed { color: var(--color-danger); }
```

**Note:** Remove or deprecate old flat list styles (`.child-job-item` specific rules) that conflict with tree layout.

### pages\queue.html(MODIFY)

References: 

- pages\static\quaero.css(MODIFY)

**Transform child jobs flat list into GitHub Actions-style tree view (lines 262-302):**

**1. Update Container Structure:**
- Change `.child-jobs-list-container` to include `role="tree"` attribute
- Update header text from "Spawned URLs" to "Spawned Jobs" (more accurate)
- Keep existing header with count display
- Change `.child-jobs-list` to `.child-jobs-tree` class

**2. Redesign Tree Item Template (replace lines 269-291):**

**New structure for each child:**
```html
<div class="tree-item" 
     :style="'--depth: ' + child.depth"
     :class="{ 'tree-item-collapsed': isChildCollapsed(item.job.id, child) }"
     role="treeitem"
     :aria-level="child.depth + 1"
     :aria-expanded="hasVisibleChildren(child) ? !isDepthCollapsed(item.job.id, child.depth) : undefined"
     tabindex="0"
     @keydown="handleTreeKeydown($event, item.job.id, child)">
  
  <!-- Collapsible toggle (only for depth 0 items that might have children) -->
  <button v-if="child.depth === 0 && mightHaveChildren(item.job.id, child.depth)" 
          class="tree-node-toggle"
          @click.stop="toggleDepthCollapse(item.job.id, child.depth)"
          :aria-label="isDepthCollapsed(item.job.id, child.depth) ? 'Expand' : 'Collapse'">
    <i class="fas fa-chevron-right" 
       :class="{ 'rotated': !isDepthCollapsed(item.job.id, child.depth) }"></i>
  </button>
  
  <!-- Status icon (replaces status badge) -->
  <i class="tree-status-icon" 
     :class="[
       'status-' + child.status,
       getStatusIcon(child.status)
     ]"
     :title="getStatusDisplayText(child.status)"></i>
  
  <!-- Job type icon (keep existing) -->
  <i v-if="child.job_type" 
     class="fas tree-job-type-icon" 
     :class="window.getJobTypeIcon(child.job_type)"
     :title="window.getJobTypeDisplayName(child.job_type)"
     style="font-size: 0.75rem; color: var(--text-secondary); margin-right: 0.5rem;"></i>
  
  <!-- URL (main content) -->
  <span class="tree-item-url" :title="child.url">{{ child.url }}</span>
  
  <!-- Metadata (depth, timestamp) -->
  <span class="tree-item-meta">
    <span class="tree-item-depth" style="font-size: 0.7rem; color: var(--text-secondary);">
      D{{ child.depth }}
    </span>
  </span>
</div>
```

**3. Remove Old Badge Elements:**
- Remove job type badge `<span class="label label-sm">` (lines 272-278)
- Remove status badge `<span class="label label-sm">` (lines 280-287)
- Keep URL and depth but restructure as shown above

**4. Update Load More Button (lines 294-300):**
- Keep existing structure and functionality
- Update button text to "Load More Jobs" instead of "Load More"
- Ensure button works with tree layout (no changes to logic needed)

**5. Add ARIA Attributes:**
- Container: `role="tree"` and `aria-label="Spawned child jobs"`
- Each item: `role="treeitem"`, `aria-level`, `aria-expanded` (if collapsible)
- Toggle buttons: `aria-label` for screen readers

**6. Add Keyboard Navigation Handler:**
- Add `@keydown` handler to each tree item
- Handler function: `handleTreeKeydown(event, parentId, child)`
- Support arrow keys, Enter, Space for navigation and expansion

**7. Update Conditional Rendering:**
- Change `x-if="item.isExpanded && childJobsList.has(...)"` to just `x-if="item.isExpanded && ..."`
- Tree items handle their own collapsed state via `isChildCollapsed()` function
- This allows partial tree expansion (some depths collapsed, others expanded)

**8. Preserve Existing Features:**
- Keep `x-for="child in getVisibleChildJobs(item.job.id)"` for pagination
- Keep `:key="child.id"` for Vue reactivity
- Keep `@click.stop` on toggle buttons to prevent event bubbling
- Maintain existing header count display

**Example of key changes:**
- **Before:** Flat list with badges, no hierarchy, no collapse
- **After:** Tree structure with icons, depth indentation, collapsible nodes, keyboard navigation

**Accessibility improvements:**
- Proper ARIA tree roles and attributes
- Keyboard navigation support (arrow keys, Enter, Space)
- Focus management with visible indicators
- Screen reader announcements for expand/collapse actions
**Add tree expansion state and keyboard navigation to Alpine.js `jobList` component (lines 1278-1577):**

**1. Add New State Variables (after line 1287):**
- `collapsedDepths: new Map()` - Map<parentId, Set<depth>> to track which depth levels are collapsed per parent
- `focusedTreeItem: null` - Track currently focused tree item for keyboard navigation
- `treeItemRefs: new Map()` - Map<childId, HTMLElement> for focus management

**2. Add Tree Collapse Management Functions (after `loadMoreChildJobs()` around line 1550):**

**Function: `toggleDepthCollapse(parentId, depth)`**
- Check if depth is in `collapsedDepths.get(parentId)` Set
- If collapsed, remove from Set (expand)
- If expanded, add to Set (collapse)
- Initialize Set if not exists: `this.collapsedDepths.set(parentId, new Set())`
- Call `this.renderJobs()` to update UI
- Announce to screen readers: "Depth {depth} {expanded/collapsed}"

**Function: `isDepthCollapsed(parentId, depth)`**
- Return `this.collapsedDepths.get(parentId)?.has(depth) || false`
- Used in template to determine if depth level is collapsed

**Function: `isChildCollapsed(parentId, child)`**
- If child.depth === 0, always visible (return false)
- Check if any parent depth (0 to child.depth-1) is collapsed
- Return true if any parent depth is collapsed, false otherwise
- Logic: `for (let d = 0; d < child.depth; d++) { if (isDepthCollapsed(parentId, d)) return true; }`

**Function: `mightHaveChildren(parentId, depth)`**
- Check if any children in `childJobsList.get(parentId)` have depth > current depth
- Return true if found, false otherwise
- Used to show/hide collapse toggle button

**Function: `hasVisibleChildren(child)`**
- Check if child has any direct children (depth + 1) in the list
- Return boolean for `aria-expanded` attribute

**3. Add Status Icon Mapping Functions (after tree collapse functions):**

**Function: `getStatusIcon(status)`**
- Map status to Font Awesome icon class:
  - 'pending': 'fa-circle'
  - 'running': 'fa-spinner fa-pulse'
  - 'completed': 'fa-check-circle'
  - 'failed': 'fa-times-circle'
  - 'cancelled': 'fa-ban'
  - default: 'fa-question-circle'
- Return icon class string

**Function: `getStatusDisplayText(status)`**
- Map status to human-readable text:
  - 'pending': 'Pending'
  - 'running': 'Running'
  - 'completed': 'Completed'
  - 'failed': 'Failed'
  - 'cancelled': 'Cancelled'
- Return display text for tooltips and screen readers

**4. Add Keyboard Navigation Handler (after status functions):**

**Function: `handleTreeKeydown(event, parentId, child)`**
- Handle arrow keys for navigation:
  - **ArrowDown**: Move focus to next visible tree item
  - **ArrowUp**: Move focus to previous visible tree item
  - **ArrowRight**: Expand collapsed node (if collapsible)
  - **ArrowLeft**: Collapse expanded node (if collapsible)
  - **Enter/Space**: Toggle expand/collapse (if collapsible)
  - **Home**: Focus first tree item
  - **End**: Focus last tree item
- Prevent default browser behavior for handled keys
- Update `focusedTreeItem` state
- Call `focusTreeItem(childId)` to move DOM focus

**Function: `focusTreeItem(childId)`**
- Get element from `treeItemRefs` Map
- Call `element.focus()` if found
- Update `focusedTreeItem` state

**Function: `getVisibleTreeItems(parentId)`**
- Get all children from `getVisibleChildJobs(parentId)`
- Filter out collapsed items using `isChildCollapsed()`
- Return array of visible children for navigation

**5. Update Existing Functions:**

**Update `toggleParentExpansion()` (line 1552):**
- When collapsing parent, clear collapsed depths: `this.collapsedDepths.delete(parentId)`
- Reset focus state: `this.focusedTreeItem = null`
- Preserve existing expand/collapse logic

**Update `handleDeleteCleanup()` (line 1374):**
- When deleting parent, also clear: `this.collapsedDepths.delete(jobId)`
- Clear tree item refs: `this.treeItemRefs.delete(childId)` for all children

**Update `handleChildSpawned()` (line 1315):**
- No changes needed - new children appear at depth 0 by default (visible)
- Existing logic handles adding to `childJobsList`

**Update `handleChildJobStatus()` (line 1351):**
- No changes needed - status updates work with tree structure
- Icon changes are handled by template reactivity

**6. Add Cleanup in `init()` (line 1298):**
- Add event listener cleanup on component destroy
- Clear Maps and Sets to prevent memory leaks

**7. Performance Optimizations:**
- Debounce `renderJobs()` calls during rapid collapse/expand
- Use `requestAnimationFrame` for smooth animations
- Limit tree depth calculations to visible items only

**8. Accessibility Announcements:**
- Add ARIA live region for status changes
- Announce expand/collapse actions to screen readers
- Use `aria-label` for dynamic content updates

**Example implementation snippet:**
```javascript
toggleDepthCollapse(parentId, depth) {
  if (!this.collapsedDepths.has(parentId)) {
    this.collapsedDepths.set(parentId, new Set());
  }
  const collapsed = this.collapsedDepths.get(parentId);
  if (collapsed.has(depth)) {
    collapsed.delete(depth);
    this.announceToScreenReader(`Depth ${depth} expanded`);
  } else {
    collapsed.add(depth);
    this.announceToScreenReader(`Depth ${depth} collapsed`);
  }
  this.renderJobs();
},

isChildCollapsed(parentId, child) {
  if (child.depth === 0) return false;
  for (let d = 0; d < child.depth; d++) {
    if (this.isDepthCollapsed(parentId, d)) return true;
  }
  return false;
},

handleTreeKeydown(event, parentId, child) {
  const visibleItems = this.getVisibleTreeItems(parentId);
  const currentIndex = visibleItems.findIndex(c => c.id === child.id);
  
  switch(event.key) {
    case 'ArrowDown':
      event.preventDefault();
      if (currentIndex < visibleItems.length - 1) {
        this.focusTreeItem(visibleItems[currentIndex + 1].id);
      }
      break;
    case 'ArrowUp':
      event.preventDefault();
      if (currentIndex > 0) {
        this.focusTreeItem(visibleItems[currentIndex - 1].id);
      }
      break;
    case 'ArrowRight':
      if (this.mightHaveChildren(parentId, child.depth) && this.isDepthCollapsed(parentId, child.depth)) {
        event.preventDefault();
        this.toggleDepthCollapse(parentId, child.depth);
      }
      break;
    case 'ArrowLeft':
      if (this.mightHaveChildren(parentId, child.depth) && !this.isDepthCollapsed(parentId, child.depth)) {
        event.preventDefault();
        this.toggleDepthCollapse(parentId, child.depth);
      }
      break;
    case 'Enter':
    case ' ':
      if (this.mightHaveChildren(parentId, child.depth)) {
        event.preventDefault();
        this.toggleDepthCollapse(parentId, child.depth);
      }
      break;
  }
}
```

**Integration with existing features:**
- Pagination continues to work at the parent level (Load More button)
- WebSocket updates (`handleChildSpawned`, `handleChildJobStatus`) work seamlessly
- Parent expansion (`toggleParentExpansion`) remains unchanged
- Real-time status updates reflect in tree icons automatically