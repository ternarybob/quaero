# go-deduplicate

Deep-scans Go codebase to find duplicate and similar functions, then consolidates them into single implementations in the correct architectural location.

## Usage

```
/go-deduplicate [path]
```

## Arguments

- `path` (optional): Path to Go project to deduplicate (defaults to current directory)

## What it does

This command performs intelligent code deduplication by:

1. **Deep Analysis Phase**
   - Scans all `.go` files recursively
   - Extracts ALL function signatures and implementations
   - Compares function bodies for similarity (not just names)
   - Detects code duplication patterns across files
   - Identifies functions that should be in services vs common
   - Maps function usage across codebase

2. **Similarity Detection**
   - **Exact Duplicates**: Identical function names and implementations
   - **Near Duplicates**: Same logic, different variable names (>80% similar)
   - **Functional Duplicates**: Same purpose, different implementations
   - **Copy-Paste Code**: Repeated logic blocks within different functions
   - **Redundant Helpers**: Multiple functions doing the same thing

3. **Classification & Decision**
   - Determines if duplicate belongs in `internal/common/` (stateless) or `internal/services/` (stateful)
   - Identifies which implementation is "best" (most robust, best error handling)
   - Plans consolidation strategy
   - Identifies all call sites that need updating

4. **Consolidation Phase**
   - **Merge Exact Duplicates**: Keep best implementation, remove others
   - **Consolidate Similar Functions**: Create single canonical function
   - **Extract Common Logic**: Pull repeated code into `internal/common/` utilities
   - **Update Call Sites**: Replace all references to point to canonical implementation
   - **Remove Dead Code**: Delete unused/orphaned functions

5. **Validation Phase**
   - Verify all call sites updated correctly
   - Ensure no broken imports
   - Run `go build` to confirm compilation
   - Generate deduplication report

## Deduplication Strategy

### Exact Duplicates
```go
// BEFORE: Duplicate in multiple files

// File: internal/services/jira_service.go
func (s *JiraService) FormatDate(t time.Time) string {
    return t.Format("2006-01-02")
}

// File: internal/services/confluence_service.go
func (c *ConfluenceService) FormatDate(t time.Time) string {
    return t.Format("2006-01-02")
}

// AFTER: Single implementation in common/

// File: internal/common/date.go
func FormatDate(t time.Time) string {
    return t.Format("2006-01-02")
}

// Updated call sites:
// s.FormatDate(now) → common.FormatDate(now)
// c.FormatDate(now) → common.FormatDate(now)
```

### Near Duplicates (80%+ Similar)
```go
// BEFORE: Similar implementations

// File: internal/services/jira_service.go
func (s *JiraService) GetPageCount(spaceKey string) (int, error) {
    resp, err := s.client.Get(s.baseURL + "/spaces/" + spaceKey)
    if err != nil {
        return 0, err
    }
    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    return int(result["size"].(float64)), nil
}

// File: internal/services/confluence_service.go
func (c *ConfluenceService) CountPages(key string) (int, error) {
    response, err := c.httpClient.Get(c.url + "/spaces/" + key)
    if err != nil {
        return 0, err
    }
    var data map[string]interface{}
    json.NewDecoder(response.Body).Decode(&data)
    return int(data["size"].(float64)), nil
}

// AFTER: Single robust implementation

// File: internal/services/confluence_service.go (if service-specific)
// OR internal/common/api.go (if reusable)
func GetSpacePageCount(client *http.Client, baseURL, spaceKey string) (int, error) {
    url := fmt.Sprintf("%s/spaces/%s", baseURL, spaceKey)
    resp, err := client.Get(url)
    if err != nil {
        return 0, fmt.Errorf("failed to fetch space: %w", err)
    }
    defer resp.Body.Close()

    var result map[string]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return 0, fmt.Errorf("failed to decode response: %w", err)
    }

    size, ok := result["size"].(float64)
    if !ok {
        return 0, fmt.Errorf("invalid size in response")
    }

    return int(size), nil
}
```

### Functional Duplicates (Same Purpose)
```go
// BEFORE: Different implementations, same purpose

// File: internal/services/user_service.go
func (s *UserService) IsValidEmail(email string) bool {
    return strings.Contains(email, "@")
}

// File: internal/services/auth_service.go
func (a *AuthService) ValidateEmailFormat(addr string) error {
    if !strings.Contains(addr, "@") {
        return errors.New("invalid email")
    }
    return nil
}

// File: internal/handlers/user_handler.go
func checkEmail(e string) bool {
    parts := strings.Split(e, "@")
    return len(parts) == 2
}

// AFTER: Single canonical implementation

// File: internal/common/validation.go
func ValidateEmail(email string) error {
    if email == "" {
        return errors.New("email cannot be empty")
    }
    parts := strings.Split(email, "@")
    if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
        return errors.New("invalid email format")
    }
    return nil
}

// All call sites updated to use common.ValidateEmail
```

### Repeated Logic Blocks
```go
// BEFORE: Copy-pasted error handling

func (s *Service) Operation1() error {
    result, err := s.doSomething()
    if err != nil {
        s.logger.Error("Operation failed", "error", err)
        return fmt.Errorf("operation1 failed: %w", err)
    }
    // use result...
}

func (s *Service) Operation2() error {
    result, err := s.doSomethingElse()
    if err != nil {
        s.logger.Error("Operation failed", "error", err)
        return fmt.Errorf("operation2 failed: %w", err)
    }
    // use result...
}

// AFTER: Extract common pattern

func (s *Service) handleError(operation string, err error) error {
    s.logger.Error("Operation failed", "operation", operation, "error", err)
    return fmt.Errorf("%s failed: %w", operation, err)
}

func (s *Service) Operation1() error {
    result, err := s.doSomething()
    if err != nil {
        return s.handleError("operation1", err)
    }
    // use result...
}

func (s *Service) Operation2() error {
    result, err := s.doSomethingElse()
    if err != nil {
        return s.handleError("operation2", err)
    }
    // use result...
}
```

## Detection Algorithms

### 1. Signature Matching
Finds functions with identical or very similar signatures:
```go
func FormatDate(t time.Time) string
func FormatTime(t time.Time) string  // Similar!
```

### 2. Body Similarity Analysis
Compares function bodies using AST (Abstract Syntax Tree):
- Ignores variable names
- Compares control flow structure
- Matches function call patterns
- Calculates similarity percentage (80%+ = near duplicate)

### 3. Semantic Analysis
Understands what functions do:
- Email validation functions
- Date formatting functions
- HTTP request wrappers
- Error handling patterns

### 4. Call Site Analysis
Maps where functions are used:
- Used in single file → candidate for private helper
- Used across services → candidate for `internal/common/`
- Service-specific state → keep in service with receiver

## Architectural Placement Rules

### Move to `internal/common/` if:
- ✅ Stateless (no struct receiver needed)
- ✅ Pure function (same inputs → same outputs)
- ✅ Used across multiple services
- ✅ No dependencies on service state (db, logger, config)
- Examples: validation, formatting, parsing, calculation

### Keep in `internal/services/` if:
- ✅ Needs service state (db, logger, http client)
- ✅ Service-specific business logic
- ✅ Uses receiver methods
- ✅ Only used within one service
- Examples: database operations, API calls, business workflows

### Extract to separate service if:
- ✅ Complex logic shared across services
- ✅ Needs its own state
- ✅ Represents a distinct domain concern
- Examples: email service, cache service, metrics service

## Examples

### Example 1: Simple Deduplication
```bash
/go-deduplicate C:\development\aktis\aktis-parser

# Output:
# 📊 Deduplication Analysis
#
# Found 8 duplicate/similar function groups:
#   - FormatDate: 2 exact duplicates
#   - GetPageCount: 2 near duplicates (85% similar)
#   - ValidateInput: 3 functional duplicates
#   - handleHTTPError: repeated in 5 places
#
# Consolidation Plan:
#   ✓ Merge 2 FormatDate → internal/common/date.go
#   ✓ Merge 2 GetPageCount → internal/services/api_helpers.go
#   ✓ Create ValidateInput → internal/common/validation.go
#   ✓ Extract handleHTTPError → internal/common/http.go
#
# Will update 47 call sites across 12 files
```

### Example 2: Current Directory
```bash
/go-deduplicate

# Scans current working directory
```

## Output Format

### Analysis Report
```markdown
# Code Deduplication Report

## Project: aktis-parser
## Scan Date: 2025-10-03 14:30:00

---

## Summary

**Files Scanned:** 45 Go files
**Functions Analyzed:** 287 functions
**Duplicates Found:** 8 groups (23 total duplicate functions)
**Estimated Lines Removed:** 456 lines
**Call Sites to Update:** 47 locations

---

## Duplicate Groups

### Group 1: FormatDate ⚠️ EXACT DUPLICATES

**Signature:** `func FormatDate(t time.Time) string`

**Instances:**
1. internal/services/jira_service.go:125 (receiver method)
2. internal/services/confluence_service.go:89 (receiver method)

**Similarity:** 100% (identical)

**Recommendation:**
- ✅ Extract to `internal/common/date.go` (stateless utility)
- Remove receiver methods
- Update 12 call sites

**Canonical Implementation:**
```go
// internal/common/date.go
func FormatDate(t time.Time) string {
    return t.Format("2006-01-02")
}
```

---

### Group 2: GetPageCount ⚠️ NEAR DUPLICATES

**Signatures:**
- `func (s *JiraService) GetPageCount(key string) (int, error)`
- `func (c *ConfluenceService) CountPages(key string) (int, error)`

**Instances:**
1. internal/services/jira_service.go:234
2. internal/services/confluence_service.go:178

**Similarity:** 85% (same logic, different var names)

**Recommendation:**
- ✅ Create unified function in `internal/common/api.go`
- Pass http.Client as parameter
- Update 8 call sites

**Canonical Implementation:**
```go
// internal/common/api.go
func GetSpacePageCount(client *http.Client, baseURL, spaceKey string) (int, error) {
    // Merged best practices from both implementations
}
```

---

### Group 3: ValidateInput ⚠️ FUNCTIONAL DUPLICATES

**Purpose:** Input validation for strings

**Instances:**
1. internal/handlers/jira_handler.go:45 - `validateProjectKey`
2. internal/handlers/confluence_handler.go:67 - `checkSpaceKey`
3. internal/services/scraper_service.go:123 - `isValidKey`

**Similarity:** 75% (same purpose, different implementations)

**Recommendation:**
- ✅ Create single validation function in `internal/common/validation.go`
- More robust than any individual implementation
- Update 15 call sites

**Canonical Implementation:**
```go
// internal/common/validation.go
func ValidateKey(key string) error {
    if key == "" {
        return errors.New("key cannot be empty")
    }
    if len(key) > 255 {
        return errors.New("key too long")
    }
    if !regexp.MustCompile(`^[A-Z0-9_-]+$`).MatchString(key) {
        return errors.New("invalid key format")
    }
    return nil
}
```

---

## Consolidation Summary

### Files to Create
- `internal/common/date.go` (date formatting utilities)
- `internal/common/api.go` (API helper functions)
- `internal/common/validation.go` (input validation)

### Files to Modify
- internal/services/jira_service.go (remove 3 duplicate functions)
- internal/services/confluence_service.go (remove 2 duplicate functions)
- internal/handlers/jira_handler.go (update imports, remove 1 function)
- internal/handlers/confluence_handler.go (update imports, remove 1 function)
- [12 more files with call site updates]

### Lines Saved
- Removed: 456 lines of duplicate code
- Added: 87 lines of canonical implementations
- **Net Reduction:** 369 lines (18.5% of analyzed code)

---

## Validation Results

✅ All call sites updated successfully
✅ Imports corrected in all files
✅ `go build` - Success (no compile errors)
✅ Function index updated
✅ Zero duplicate functions remaining

---

## Next Steps

1. Review consolidated functions in `internal/common/`
2. Run full test suite: `go test ./...`
3. Commit changes: `git commit -m "refactor: consolidate duplicate functions"`
4. Delete `.backup` files if satisfied
```

## Safety Features

- ✅ Creates `.backup` files before modifications
- ✅ Preserves business logic exactly
- ✅ Updates all call sites automatically
- ✅ Validates compilation after changes
- ✅ Provides detailed change report
- ✅ Rollback instructions included

## When to Use

Use `/go-deduplicate` when:
- ✅ Multiple developers created similar functions
- ✅ Copy-paste coding is suspected
- ✅ Want to reduce codebase size
- ✅ Need to enforce DRY principle
- ✅ Consolidating after merge/refactor
- ✅ Preparing for code review
- ✅ Cleaning up legacy code

## Integration with Other Commands

```bash
# Complete cleanup workflow:
/go-enforce-compliance    # Fix code quality violations
/go-deduplicate          # Remove duplicate code
# Result: Clean, compliant, DRY codebase
```

## Advanced Features

### Similarity Threshold Control
The agent uses intelligent thresholds:
- **100%**: Exact duplicates (always merge)
- **80-99%**: Near duplicates (merge with review)
- **60-79%**: Similar functions (suggest consolidation)
- **<60%**: Different implementations (report only)

### Whitelist Patterns
Some "duplicates" are intentional:
- Test helper functions
- Interface implementations
- Builder patterns
- Factory methods

The agent recognizes these and skips them.

### Cross-Service Analysis
Analyzes whether shared code should be:
1. **Common utility** - Used everywhere
2. **Service-specific** - Keep in service
3. **New service** - Extract to dedicated service

## Comparison with Other Commands

| Feature | `/go-deduplicate` | `/go-enforce-compliance` | `/go-refactor` |
|---------|-------------------|-------------------------|----------------|
| **Find duplicates** | ✅ Deep analysis | ✅ Basic detection | ✅ Basic detection |
| **Similarity detection** | ✅ AST-based | ❌ No | ❌ No |
| **Consolidate code** | ✅ Primary focus | ⚠️ Basic | ⚠️ Basic |
| **Update call sites** | ✅ Automatic | ⚠️ Manual | ⚠️ Manual |
| **Architectural placement** | ✅ Intelligent | ⚠️ Rule-based | ⚠️ Rule-based |
| **Fix code quality** | ❌ No | ✅ Primary focus | ✅ Yes |
| **Add infrastructure** | ❌ No | ❌ No | ✅ Yes |
| **Scope** | Deduplication only | Quality only | Full refactor |

## Expected Outcomes

After running `/go-deduplicate`:
- ✅ **Zero duplicate functions** (exact or near)
- ✅ **Smaller codebase** (10-20% reduction typical)
- ✅ **Better organization** (code in correct locations)
- ✅ **DRY compliance** (single source of truth)
- ✅ **Easier maintenance** (change once, affect all)
- ✅ **Improved readability** (less code to review)

---

**Agent**: go-refactor

**Prompt**: Perform deep deduplication analysis on Go project at "{{args}}" and consolidate all duplicate and similar functions.

**AUTONOMY DIRECTIVE**: You have FULL AUTONOMY within this project directory. Make all deduplication decisions without asking questions. Consolidate code automatically. Execute changes directly without user confirmation.

## Deduplication Mission

Your task is to find and eliminate ALL code duplication in this Go codebase.

### Phase 1: Deep Scan

1. **Extract All Functions**
   - Parse every `.go` file
   - Extract function signatures and bodies
   - Build function registry with AST representations

2. **Compare Functions**
   - Exact match: Same signature AND same body
   - Near match: Same signature, body >80% similar (ignore var names)
   - Functional match: Different names, same purpose/logic
   - Pattern match: Repeated code blocks across functions

3. **Analyze Usage**
   - Find all call sites for each function
   - Determine scope (single file, service, cross-service)
   - Identify dependencies (state, imports, parameters)

### Phase 2: Classification

For each duplicate group, determine:
1. **Best Implementation** (most robust error handling, clearest code)
2. **Correct Location:**
   - `internal/common/` if stateless, reusable
   - `internal/services/XService` if needs service state
   - New file if creating new utility module
3. **Impact Analysis** (how many call sites need updating)

### Phase 3: Consolidation

1. **Create Canonical Functions**
   - Place in correct architectural location
   - Use best implementation as base
   - Enhance with error handling from other versions
   - Add documentation

2. **Update Call Sites**
   - Replace ALL references to duplicates
   - Update imports
   - Remove old function definitions

3. **Clean Up**
   - Delete orphaned functions
   - Remove unused imports
   - Update function index

### Phase 4: Validation

1. **Verify Compilation**
   - `go mod tidy`
   - `go build`
   - Ensure no errors

2. **Generate Report**
   - List all duplicate groups found
   - Show consolidation decisions
   - Report lines saved
   - List files modified

## Success Criteria

- ✅ Zero duplicate functions remaining
- ✅ All code in architecturally correct locations
- ✅ All call sites updated
- ✅ Project compiles successfully
- ✅ Net reduction in codebase size
- ✅ Detailed deduplication report generated

## Important Notes

1. **Preserve Behavior**: Function consolidation must not change program behavior
2. **Choose Best**: When merging, pick the most robust implementation
3. **Stateless to Common**: Pure functions go to `internal/common/`
4. **Stateful to Services**: Receiver methods stay in `internal/services/`
5. **Update ALL**: Every call site must be updated, no exceptions
