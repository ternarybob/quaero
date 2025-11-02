I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The task requires adding post-job configuration UI to the job definitions management interface. This is the final phase of the post-jobs feature implementation.

**Current State:**
- Backend model has `PostJobs []string` field with marshal/unmarshal methods
- Database schema includes `post_jobs TEXT` column
- JobExecutor triggers post-jobs after completion via callback
- UI has no post-jobs configuration or display

**Key UI Patterns Identified:**

1. **Multi-Select Pattern (Sources):**
   - Lines 395-408 in `jobs.html` show a multi-select dropdown for sources
   - Uses `x-model="currentJobDefinition.sources"` with `multiple` attribute
   - Displays hint text and warning when no options available
   - Template loop renders available options

2. **Alpine.js Component Structure:**
   - `resetCurrentJobDefinition()` (lines 582-596) initializes default values
   - `saveJobDefinition()` (lines 698-727) handles create/update operations
   - `editJobDefinition()` (lines 682-696) loads existing job for editing
   - `formatSourcesList()` (lines 798-801) formats array display for cards

3. **Card Display Pattern:**
   - Lines 308-361 show job definition cards with metadata
   - Lines 326-330 display sources count using `formatSourcesList()`
   - Metadata uses icon + text pattern with flexbox layout

**Design Decisions:**

1. **Post-Jobs Multi-Select Location:** Place after the "Sources" field and before "Schedule" field in the modal (logical workflow: define what to run, then what to run after)

2. **Available Options:** Populate with all job definitions EXCEPT the current one being edited (prevent circular dependencies and self-references)

3. **Display Format:** Show count in card metadata (e.g., "2 post-jobs") with tooltip/hover showing job names

4. **Validation:** No client-side validation needed - backend handles enabled/valid checks during execution

5. **Empty State:** Show hint text when no job definitions available (similar to sources pattern)

### Approach

Add post-jobs configuration UI to the job definitions modal by introducing a multi-select field that allows users to select other job definitions to run after completion. Update the Alpine.js component to handle the post_jobs array throughout the lifecycle (initialization, save, edit), and enhance the job definition cards to display post-job information similar to how sources are currently displayed.

### Reasoning

I examined the repository structure, read `pages/jobs.html` and `pages/static/common.js` to understand the existing UI patterns for job definitions management. I identified that the modal already has a multi-select pattern for sources (lines 395-408 in jobs.html), and the Alpine.js component `jobDefinitionsManagement` (lines 524-802 in common.js) handles CRUD operations. The post_jobs field has been added to the backend model and storage layer in previous phases, so this phase focuses solely on the UI layer.

## Mermaid Diagram

sequenceDiagram
    participant User
    participant Modal as Job Definition Modal
    participant Alpine as jobDefinitionsManagement
    participant API as Backend API
    participant Card as Job Definition Card

    Note over User,Card: Creating/Editing Job with Post-Jobs

    User->>Modal: Click "Add Job Definition"
    Modal->>Alpine: openCreateModal()
    Alpine->>Alpine: resetCurrentJobDefinition()<br/>(post_jobs: [])
    Alpine->>Alpine: Compute availablePostJobs<br/>(filter out current job)
    Modal-->>User: Show modal with empty post_jobs

    User->>Modal: Select post-jobs from dropdown<br/>(e.g., "Corpus Summary", "DB Maintenance")
    Modal->>Alpine: x-model updates post_jobs array

    User->>Modal: Click "Save"
    Modal->>Alpine: saveJobDefinition()
    Alpine->>API: POST /api/job-definitions<br/>{..., post_jobs: ["id1", "id2"]}
    API-->>Alpine: Success
    Alpine->>Alpine: loadJobDefinitions()
    API-->>Alpine: Job definitions with post_jobs

    Alpine->>Card: Render job definition cards
    Card->>Alpine: formatPostJobsList(post_jobs)
    Alpine-->>Card: "2 post-jobs"
    Card->>Alpine: getPostJobsTooltip(post_jobs)
    Alpine-->>Card: "Post-jobs:\nCorpus Summary\nDB Maintenance"
    Card-->>User: Display "🔗 2 post-jobs" with tooltip

    Note over User,Card: Editing Existing Job

    User->>Card: Click "Edit" button
    Card->>Alpine: editJobDefinition(jobDef)
    Alpine->>Alpine: Deep clone jobDef<br/>(includes post_jobs)
    Alpine->>Alpine: Defensive check:<br/>post_jobs = post_jobs || []
    Alpine->>Alpine: Compute availablePostJobs<br/>(exclude current job)
    Modal-->>User: Show modal with selected post-jobs

    User->>Modal: Modify post-jobs selection
    User->>Modal: Click "Save"
    Modal->>Alpine: saveJobDefinition()
    Alpine->>API: PUT /api/job-definitions/{id}<br/>{..., post_jobs: ["id1", "id3"]}
    API-->>Alpine: Success
    Alpine->>Alpine: loadJobDefinitions()
    Card-->>User: Updated display

## Proposed File Changes

### pages\jobs.html(MODIFY)

References: 

- pages\static\common.js(MODIFY)

**Add Post-Jobs Multi-Select Field to Modal (after line 408, before Schedule field):**

Insert a new form group for post-jobs configuration:
- Use `<div class="form-group">` wrapper
- Add label: `<label class="form-label">Post-Jobs (Optional)</label>`
- Create multi-select dropdown:
  - `<select class="form-select" x-model="currentJobDefinition.post_jobs" multiple size="5">`
  - Use `x-for` template to iterate through `availablePostJobs` (computed property)
  - Each option: `<option :value="jobDef.id" x-text="jobDef.name"></option>`
  - Close select tag
- Add hint text: `<p class="form-input-hint">Select job definitions to execute after this job completes successfully. Jobs run independently (no parent/child relationship). Hold Ctrl/Cmd to select multiple.</p>`
- Add empty state warning (using `x-if` template):
  - `<template x-if="availablePostJobs.length === 0">`
  - `<p class="form-input-hint text-warning">No other job definitions available. Create additional jobs first.</p>`
  - Close template

This follows the exact same pattern as the Sources multi-select field (lines 395-408).

**Add Post-Jobs Display to Job Definition Cards (after line 336, before Status):**

Insert a new metadata item in the card's metadata section:
- Use `<div>` wrapper (consistent with other metadata items)
- Add icon: `<i class="fas fa-link"></i>` (represents job chaining/linking)
- Add text span: `<span x-text="formatPostJobsList(jobDef.post_jobs)"></span>`
- Close div

This displays the post-jobs count/names inline with other metadata like sources, schedule, and status.

**Optional Enhancement - Add Tooltip/Title Attribute:**

For better UX, add a title attribute to the post-jobs metadata div:
- `:title="getPostJobsTooltip(jobDef.post_jobs)"`
- This shows job names on hover (implemented in common.js)

This enhancement is optional but recommended for discoverability.

### pages\static\common.js(MODIFY)

References: 

- pages\jobs.html(MODIFY)
- internal\models\job_definition.go

**Update resetCurrentJobDefinition Method (line 582-596):**

Add `post_jobs` field initialization:
- After line 594 (`config: {}`), add: `post_jobs: []`
- This ensures new job definitions start with an empty post-jobs array
- Matches the backend model's default value

**Add availablePostJobs Computed Property (after line 596, before generateID method):**

Create a getter that filters job definitions for the post-jobs dropdown:
- Method signature: `get availablePostJobs() { ... }`
- Logic:
  - Return `this.jobDefinitions.filter(jobDef => jobDef.id !== this.currentJobDefinition.id)`
  - This excludes the current job being edited (prevents self-reference)
  - Returns all other job definitions as valid post-job options
- This computed property is reactive and updates when jobDefinitions or currentJobDefinition changes

**Update saveJobDefinition Method (lines 698-727):**

Ensure `post_jobs` field is included in the saved payload:
- No changes needed - line 705 already does deep clone: `JSON.parse(JSON.stringify(this.currentJobDefinition))`
- The `post_jobs` array is automatically included in the cloned object
- Backend API expects `post_jobs` field and will persist it

Verify that the method doesn't strip out the `post_jobs` field during serialization.

**Update editJobDefinition Method (lines 682-696):**

Ensure `post_jobs` field is loaded when editing:
- No changes needed - line 684 already does deep clone: `JSON.parse(JSON.stringify(jobDef))`
- The `post_jobs` array from the backend is automatically included
- If `post_jobs` is missing (old job definitions), JavaScript will treat it as undefined

Add defensive initialization after line 684:
- Check if `this.currentJobDefinition.post_jobs` is undefined or null
- If so, initialize to empty array: `if (!this.currentJobDefinition.post_jobs) { this.currentJobDefinition.post_jobs = []; }`
- This handles backward compatibility with job definitions created before post-jobs feature

**Add formatPostJobsList Method (after line 801, before closing of jobDefinitionsManagement component):**

Create a helper method to format post-jobs display for cards:
- Method signature: `formatPostJobsList(postJobs) { ... }`
- Logic:
  - If `!postJobs || postJobs.length === 0`, return `'None'`
  - Otherwise, return `${postJobs.length} post-job${postJobs.length !== 1 ? 's' : ''}`
  - Example outputs: "None", "1 post-job", "3 post-jobs"
- This matches the pattern of `formatSourcesList()` method (lines 798-801)

**Add getPostJobsTooltip Method (optional, after formatPostJobsList):**

Create a helper method to generate tooltip text showing job names:
- Method signature: `getPostJobsTooltip(postJobIds) { ... }`
- Logic:
  - If `!postJobIds || postJobIds.length === 0`, return `'No post-jobs configured'`
  - Map each post-job ID to its name by looking up in `this.jobDefinitions`
  - Use `this.jobDefinitions.find(jd => jd.id === postJobId)?.name || postJobId`
  - Join names with newline: `names.join('\n')`
  - Prefix with "Post-jobs:\n" for clarity
  - Example output: "Post-jobs:\nCorpus Summary Generation\nDatabase Maintenance"
- This provides better UX by showing job names on hover instead of just IDs

**Update loadJobDefinitions Method (lines 541-567):**

No changes needed - method already loads all job definitions including the `post_jobs` field from the backend API. The API response includes `post_jobs` array for each job definition (added in previous phase).

**Backward Compatibility Consideration:**

Add a defensive check in the `init()` method (line 534-539) or in `loadJobDefinitions()` to ensure all loaded job definitions have a `post_jobs` field:
- After line 559 (where jobDefinitions array is assigned), add:
  - `this.jobDefinitions = this.jobDefinitions.map(jd => ({ ...jd, post_jobs: jd.post_jobs || [] }))`
  - This ensures old job definitions without `post_jobs` field get an empty array
  - Prevents undefined errors in the UI