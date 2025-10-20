I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

## Current State Analysis

**Backend Infrastructure (Complete)**:
- `SourceConfig.Filters` field exists as `map[string]interface{}` in `internal/models/source.go` (line 36)
- Storage layer properly serializes/deserializes filters as JSON in `internal/storage/sqlite/source_storage.go` (lines 37-40, 230-232)
- URL derivation logic in `internal/common/url_utils.go` fully supports filters:
  - Jira: `filters["projects"]` (array) or `filters["project"]` (string) → generates `/browse/{projectKey}` URLs
  - Confluence: `filters["spaces"]` (array) or `filters["space"]` (string) → generates `/spaces/{spaceKey}` URLs
  - GitHub: `filters["org"]` or `filters["user"]` (string) → generates org/user repo URLs
- Job helper `StartCrawlJob()` calls `common.DeriveSeedURLs(source, ...)` which reads filters automatically

**Frontend Gaps**:
- Modal form in `pages/sources.html` (lines 129-214) has NO filter input fields
- Alpine.js component `sourceManagement` in `pages/static/common.js` already initializes `filters: {}` (line 364)
- Source list table (lines 71-124) does NOT display filters column
- No UI feedback showing which filters are active on a source

**Key Insight**: The backend is fully functional and waiting for UI input. This is purely a frontend enhancement task with optional backend validation improvements.

### Approach

## Implementation Strategy

**Phase 1: Add Dynamic Filter Input Fields to Modal**
Add source-type-aware filter inputs that show/hide based on selected source type using Alpine.js `x-show` directives.

**Phase 2: Update Alpine.js Component**
Ensure filter data structure is properly initialized and converted between UI format (comma-separated strings) and backend format (arrays/objects).

**Phase 3: Add Filter Display to Source List**
Show active filters in the table with visual badges for easy identification.

**Phase 4: Add Backend Validation (Optional)**
Validate filter format and provide helpful error messages for invalid configurations.

**Design Decision**: Use comma-separated input fields for array filters (e.g., "PROJ1, PROJ2, PROJ3") rather than complex multi-input UI to keep the interface simple and maintainable.

### Reasoning

Explored the repository structure, read source management UI (`pages/sources.html`), handler (`internal/handlers/sources_handler.go`), model (`internal/models/source.go`), URL utilities (`internal/common/url_utils.go`), job helper (`internal/services/jobs/job_helper.go`), source service (`internal/services/sources/service.go`), storage layer (`internal/storage/sqlite/source_storage.go`), and frontend JavaScript (`pages/static/common.js`). Analyzed the complete data flow from UI input through storage to URL derivation, confirming that backend infrastructure is complete and only UI enhancements are needed.

## Mermaid Diagram

sequenceDiagram
    participant User
    participant UI as sources.html
    participant Alpine as sourceManagement (Alpine.js)
    participant Handler as sources_handler.go
    participant Model as SourceConfig.Validate()
    participant Storage as source_storage.go
    participant URLUtils as url_utils.DeriveSeedURLs()
    participant Crawler as Job Execution

    Note over User,Crawler: Filter Configuration Flow

    User->>UI: Opens "Add Source" modal
    UI->>Alpine: Initializes currentSource with filters: {}
    User->>UI: Selects source type (e.g., "jira")
    UI->>UI: Shows Jira filter inputs (x-show directive)
    
    User->>UI: Enters "PROJ1, PROJ2, PROJ3" in Projects field
    UI->>Alpine: Binds to currentSource.filters.projects (string)
    
    User->>UI: Clicks "Save"
    Alpine->>Alpine: parseFilterInput() converts "PROJ1, PROJ2, PROJ3" to ["PROJ1", "PROJ2", "PROJ3"]
    Alpine->>Handler: POST /api/sources with filters: {projects: [...]}
    
    Handler->>Handler: validateSourceFilters() checks format
    Handler->>Model: source.Validate() validates structure
    Model->>Model: Checks filter types and mutual exclusivity
    
    alt Validation Passes
        Handler->>Storage: SaveSource() with filters
        Storage->>Storage: json.Marshal(filters) to JSON string
        Storage->>Storage: INSERT/UPDATE sources table
        Handler->>Alpine: 201 Created response
        Alpine->>UI: Shows success notification
        Alpine->>Alpine: Reloads source list
        UI->>User: Displays source with filter badge "Projects: 3"
    else Validation Fails
        Handler->>Alpine: 400 Bad Request with error message
        Alpine->>UI: Shows error notification
        UI->>User: Displays validation error
    end

    Note over User,Crawler: Filter Usage During Crawl

    User->>UI: Triggers crawl job for source
    Crawler->>Storage: Loads SourceConfig with filters
    Storage->>Storage: json.Unmarshal(filters) from JSON
    Storage->>Crawler: Returns SourceConfig with filters: {projects: [...]}
    
    Crawler->>URLUtils: DeriveSeedURLs(source, useHTMLSeeds, logger)
    URLUtils->>URLUtils: Checks source.Type == "jira"
    URLUtils->>URLUtils: Reads source.Filters["projects"]
    URLUtils->>URLUtils: Generates URLs: ["/browse/PROJ1", "/browse/PROJ2", "/browse/PROJ3"]
    URLUtils->>Crawler: Returns seed URLs array
    
    Crawler->>Crawler: Starts crawl with filtered seed URLs
    Note over Crawler: Only crawls specified projects, not all projects

## Proposed File Changes

### pages\sources.html(MODIFY)

References: 

- pages\static\common.js(MODIFY)

## Section 1: Add Filter Input Fields to Modal Form (after line 173, before crawl_config section)

**Add Dynamic Filter Section**:
1. Insert new form group with label "Filters" and help text explaining filter purpose
2. Add Jira-specific filter inputs (visible when `currentSource.type === 'jira'`):
   - Text input for "Projects" with placeholder "PROJ1, PROJ2, PROJ3" (comma-separated)
   - Help text: "Enter Jira project keys to crawl (comma-separated). Leave empty to crawl all projects."
   - Bind to `currentSource.filters.projects` using `x-model`
3. Add Confluence-specific filter inputs (visible when `currentSource.type === 'confluence'`):
   - Text input for "Spaces" with placeholder "SPACE1, SPACE2, SPACE3" (comma-separated)
   - Help text: "Enter Confluence space keys to crawl (comma-separated). Leave empty to crawl all spaces."
   - Bind to `currentSource.filters.spaces` using `x-model`
4. Add GitHub-specific filter inputs (visible when `currentSource.type === 'github'`):
   - Text input for "Organization" with placeholder "myorg"
   - Text input for "User" with placeholder "username"
   - Help text: "Enter either organization OR user (not both). Leave empty to require manual configuration."
   - Bind to `currentSource.filters.org` and `currentSource.filters.user` using `x-model`
5. Use Alpine.js `x-show` directive to conditionally display filter inputs based on `currentSource.type`

**Implementation Notes**:
- Place filter section BEFORE crawl_config section (line 182) for logical flow
- Use consistent styling with existing form groups (class="form-group")
- Add `style="margin-bottom: 0.5rem;"` for spacing consistency
- Ensure tab order flows naturally through form fields

## Section 2: Add Filters Column to Source List Table (modify lines 71-124)

**Update Table Header** (after line 77, before STATUS column):
1. Add new `<th>FILTERS</th>` column header between BASE URL and AUTHENTICATION columns
2. This provides visibility into which sources have filters configured

**Update Table Body** (in template x-for loop, after base_url cell):
1. Add new `<td>` cell to display filters
2. Use Alpine.js template logic to render filter badges:
   - For Jira: Show badge with "Projects: {count}" if `source.filters.projects` exists
   - For Confluence: Show badge with "Spaces: {count}" if `source.filters.spaces` exists
   - For GitHub: Show badge with "Org: {name}" or "User: {name}" if configured
   - If no filters: Show "All" or "-" to indicate no filtering
3. Use `<span class="label label-secondary">` for filter badges to match existing UI patterns
4. Handle both array and string filter formats (projects can be array or single string)

**Update Empty State** (line 86):
1. Change colspan from "7" to "8" to account for new FILTERS column

**Visual Design**:
- Use compact badge display to avoid table width issues
- Truncate long filter lists with "... +N more" pattern if needed
- Ensure responsive design doesn't break on smaller screens

## Section 3: Update Table Column Count for Empty State

**Fix Empty Row Colspan** (line 86):
1. Update `colspan="7"` to `colspan="8"` to match new column count (NAME, TYPE, BASE URL, FILTERS, AUTHENTICATION, STATUS, CREATED, ACTIONS)
2. This ensures the "No sources configured" message spans the full table width

**Verification**:
- Count all `<th>` elements in header to confirm total columns
- Ensure all data rows have matching `<td>` count
- Test empty state display with no sources configured

### pages\static\common.js(MODIFY)

References: 

- pages\sources.html(MODIFY)
- internal\models\source.go(MODIFY)

## Section 1: Enhance resetCurrentSource() Method (lines 350-366)

**Update Filter Initialization**:
1. Current implementation already initializes `filters: {}` (line 364) - this is correct
2. No changes needed to initialization logic
3. Empty object allows dynamic filter properties to be added by UI bindings

**Verification Only**:
- Confirm `filters: {}` is present in `resetCurrentSource()` method
- This empty object will be populated by Alpine.js `x-model` bindings from UI inputs
- Backend expects `map[string]interface{}` which maps perfectly to JavaScript object

## Section 2: Add Filter Parsing Helper Methods (new methods after line 476)

**Create `parseFilterInput()` Helper Method**:
1. Method signature: `parseFilterInput(input, filterType)`
2. Purpose: Convert comma-separated string input to array format for backend
3. Logic:
   - If input is empty/null, return undefined (removes filter from object)
   - Trim whitespace from input string
   - Split by comma: `input.split(',').map(s => s.trim()).filter(s => s.length > 0)`
   - Return array of trimmed, non-empty strings
   - For single-value filters (GitHub org/user), return string directly
4. Call this method in `saveSource()` before sending to backend

**Create `formatFilterDisplay()` Helper Method**:
1. Method signature: `formatFilterDisplay(filters, sourceType)`
2. Purpose: Generate human-readable filter summary for table display
3. Logic:
   - Check source type and extract relevant filter keys
   - For Jira: Check `filters.projects` (array) or `filters.project` (string)
   - For Confluence: Check `filters.spaces` (array) or `filters.space` (string)
   - For GitHub: Check `filters.org` or `filters.user`
   - Return formatted string like "Projects: 3" or "Org: myorg"
   - Return empty string if no filters configured
4. Use this method in template bindings for filter column display

## Section 3: Update saveSource() Method (lines 404-431)

**Add Filter Processing Before Save**:
1. Before sending request (after line 409, before fetch call), process filter inputs
2. Create a copy of `currentSource` to avoid mutating reactive data during processing
3. For Jira sources:
   - If `currentSource.filters.projects` is a string, parse it to array using `parseFilterInput()`
   - Store result back to filters object
4. For Confluence sources:
   - If `currentSource.filters.spaces` is a string, parse it to array using `parseFilterInput()`
   - Store result back to filters object
5. For GitHub sources:
   - Ensure `org` and `user` are strings (not arrays)
   - Remove empty string values to avoid sending invalid filters
6. Clean up empty filter object: if `filters` is empty object `{}`, consider removing it or leaving as-is (backend handles both)

**Error Handling**:
- Add try-catch around filter parsing to handle malformed input
- Show user-friendly error message if filter parsing fails
- Validate that GitHub doesn't have both org AND user set (mutually exclusive)

## Section 4: Update editSource() Method (lines 368-384)

**Add Filter Format Conversion for Editing**:
1. After deep cloning source (line 370), convert filter arrays to comma-separated strings for UI display
2. For Jira sources:
   - If `currentSource.filters.projects` is an array, join with ", " to create string
   - This allows user to see and edit existing filters in text input
3. For Confluence sources:
   - If `currentSource.filters.spaces` is an array, join with ", " to create string
4. For GitHub sources:
   - No conversion needed (org/user are already strings)
5. This ensures round-trip editing works correctly: array → string (for editing) → array (for saving)

**Implementation Note**:
- Use `Array.isArray()` to check if filter value is array before joining
- Handle both legacy single-string filters and new array filters
- Preserve backward compatibility with existing data formats

### internal\handlers\sources_handler.go(MODIFY)

References: 

- internal\models\source.go(MODIFY)
- internal\common\url_utils.go

## Section 1: Add Filter Validation Helper Function (new function after line 175)

**Create `validateSourceFilters()` Function**:
1. Function signature: `func validateSourceFilters(source *models.SourceConfig) error`
2. Purpose: Validate filter format and content based on source type
3. Validation logic for Jira sources:
   - Check if `filters["projects"]` exists and is either `[]string`, `[]interface{}`, or `string`
   - If array, validate each project key is non-empty string
   - If single string in `filters["project"]`, validate it's non-empty
   - Project keys should match pattern: uppercase letters, numbers, underscore (optional validation)
4. Validation logic for Confluence sources:
   - Check if `filters["spaces"]` exists and is either `[]string`, `[]interface{}`, or `string`
   - If array, validate each space key is non-empty string
   - If single string in `filters["space"]`, validate it's non-empty
   - Space keys should be non-empty alphanumeric strings
5. Validation logic for GitHub sources:
   - Check if `filters["org"]` and `filters["user"]` are mutually exclusive (not both set)
   - Validate org/user are non-empty strings if present
   - Return error if both org and user are specified
6. Return `nil` if validation passes, descriptive error if validation fails

**Error Messages**:
- Use clear, actionable error messages: "Jira filters must contain 'projects' (array) or 'project' (string)"
- Include examples in error messages: "Example: {\"projects\": [\"PROJ1\", \"PROJ2\"]}"
- Help users understand correct filter format for each source type

## Section 2: Integrate Validation in CreateSourceHandler (after line 88)

**Add Filter Validation Call**:
1. After decoding request body (line 87) and before calling `source.Validate()` (line 89)
2. Call `validateSourceFilters(&source)` to check filter format
3. If validation fails, log error with filter details and return HTTP 400 Bad Request
4. Include validation error message in response body for client debugging
5. Use existing error handling pattern: `WriteError(w, http.StatusBadRequest, err.Error())`

**Logging**:
- Log filter validation attempts with source type and filter content
- Use `h.logger.Debug()` for successful validation
- Use `h.logger.Warn()` for validation failures with filter details
- Include source ID in log context for traceability

## Section 3: Integrate Validation in UpdateSourceHandler (after line 120)

**Add Filter Validation Call**:
1. After decoding request body (line 120) and setting ID (line 123)
2. Call `validateSourceFilters(&source)` before calling `source.Validate()` (line 125)
3. Use same error handling pattern as CreateSourceHandler
4. Log validation results with source ID for audit trail

**Consistency**:
- Ensure validation logic is identical between Create and Update handlers
- Both handlers should reject invalid filters with same error messages
- Consider extracting common validation logic to reduce duplication

## Section 4: Add Filter Logging for Debugging (in CreateSourceHandler and UpdateSourceHandler)

**Enhanced Logging**:
1. After successful validation, log filter configuration for debugging
2. Use structured logging with `h.logger.Debug()` to log:
   - Source ID
   - Source type
   - Filter keys present (e.g., "projects", "spaces", "org")
   - Filter value count (e.g., "3 projects", "2 spaces")
   - Do NOT log actual filter values in production (may contain sensitive data)
3. This helps diagnose filter-related issues in production without exposing sensitive information

**Example Log Entry**:
```
DEBUG [SourcesHandler] Filter validation passed: source_id=abc-123, source_type=jira, filter_keys=[projects], project_count=3
```

**Security Note**:
- Avoid logging actual project/space names in production logs
- Log only metadata (counts, keys) to prevent information disclosure
- Use debug level for detailed filter logging to avoid log spam

### internal\models\source.go(MODIFY)

References: 

- internal\common\url_utils.go
- internal\handlers\sources_handler.go(MODIFY)

## Section 1: Add Filter Validation to Validate() Method (after line 76, before return nil)

**Add Basic Filter Structure Validation**:
1. Check if `Filters` field is not nil (it can be empty map, but shouldn't be nil)
2. If `Filters` is nil, initialize it to empty map: `s.Filters = make(map[string]interface{})`
3. Validate that filter values are of expected types:
   - String values should be non-empty if present
   - Array values should contain at least one element if present
   - Reject unsupported filter keys for each source type
4. This provides basic type safety before handler-level validation

**Type-Specific Filter Validation**:
1. For Jira sources (`s.Type == SourceTypeJira`):
   - Accept filter keys: "projects" (array or []interface{}), "project" (string)
   - Warn if both "projects" and "project" are set (ambiguous)
   - Validate array elements are strings if "projects" is array
2. For Confluence sources (`s.Type == SourceTypeConfluence`):
   - Accept filter keys: "spaces" (array or []interface{}), "space" (string)
   - Warn if both "spaces" and "space" are set (ambiguous)
   - Validate array elements are strings if "spaces" is array
3. For GitHub sources (`s.Type == SourceTypeGithub`):
   - Accept filter keys: "org" (string), "user" (string)
   - Return error if both "org" and "user" are set (mutually exclusive)
   - Validate values are non-empty strings

**Error Messages**:
- Return descriptive errors: `fmt.Errorf("invalid filter for %s source: %s", s.Type, details)`
- Include guidance on correct filter format in error message
- Reference `internal/common/url_utils.go` documentation for filter examples

**Backward Compatibility**:
- Allow empty `Filters` map (no filters = crawl all)
- Support both array and single-string formats for projects/spaces
- Don't break existing sources that have no filters configured

## Section 2: Add Filter Documentation Comments (before Filters field, line 36)

**Add Comprehensive Field Documentation**:
1. Add multi-line comment explaining filter purpose and format
2. Document filter keys for each source type:
   - Jira: `{"projects": ["PROJ1", "PROJ2"]}` or `{"project": "PROJ1"}`
   - Confluence: `{"spaces": ["SPACE1", "SPACE2"]}` or `{"space": "SPACE1"}`
   - GitHub: `{"org": "myorg"}` or `{"user": "username"}` (mutually exclusive)
3. Explain that empty filters map means "crawl all" (no filtering)
4. Reference `DeriveSeedURLs()` in `internal/common/url_utils.go` for implementation details

**Example Documentation**:
```go
// Filters contains source-specific filtering criteria for crawling.
// Supported filters by source type:
//   - Jira: {"projects": ["PROJ1", "PROJ2"]} or {"project": "PROJ1"}
//   - Confluence: {"spaces": ["SPACE1", "SPACE2"]} or {"space": "SPACE1"}
//   - GitHub: {"org": "myorg"} or {"user": "username"} (mutually exclusive)
// Empty map means no filtering (crawl all accessible content).
// See DeriveSeedURLs() in internal/common/url_utils.go for filter usage.
Filters map[string]interface{} `json:"filters"`
```

**Benefits**:
- Developers can understand filter format without reading multiple files
- IDE tooltips show filter documentation when hovering over field
- Reduces confusion about filter structure and usage