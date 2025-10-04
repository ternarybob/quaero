# go-enforce-compliance

Scans and refactors existing Go code to enforce compliance with code quality standards and clean architecture patterns.

## Usage

```
/go-enforce-compliance [path]
```

## Arguments

- `path` (optional): Path to Go project to enforce compliance on (defaults to current directory)

## What it does

This command performs automated compliance enforcement on existing Go codebases by:

1. **Analysis Phase**
   - Scans all `.go` files in the project
   - Identifies violations of Go structure standards
   - Detects duplicate functions across codebase
   - Finds unused/redundant functions
   - Locates forbidden patterns (fmt.Println, log.Println, ignored errors)
   - Checks function length violations (>80 lines)
   - Validates directory structure rules (common/ vs services/)
   - Identifies missing required libraries (arbor, banner)

2. **Violation Detection**
   - **Logging Violations**: Usage of fmt.Println/log.Println instead of arbor logger
   - **Error Handling**: Ignored errors (`_ = ...`)
   - **Function Length**: Functions exceeding 80 lines
   - **Duplicate Functions**: Multiple implementations of same function
   - **Directory Rules**: Receiver methods in `internal/common/`
   - **Missing Libraries**: Projects not using required ternarybob libraries
   - **Startup Sequence**: Incorrect order in main.go

3. **Refactoring Phase**
   - **Replace Logging**: Convert fmt.Println/log.Println to arbor logger calls
   - **Remove Duplicates**: Consolidate duplicate functions into single implementations
   - **Fix Error Handling**: Properly handle all errors
   - **Split Long Functions**: Break functions >80 lines into smaller, focused functions
   - **Restructure Directories**: Move code to comply with services/ vs common/ rules
   - **Add Required Libraries**: Import and configure arbor, banner, go-toml
   - **Fix Startup Sequence**: Reorder main.go to follow standard pattern
   - **Remove Dead Code**: Delete unused/unreferenced functions

4. **Validation Phase**
   - Re-run compliance checks on refactored code
   - Verify all violations resolved
   - Ensure code still compiles (`go build`)
   - Generate compliance report

## Compliance Standards

### Required Libraries
- `github.com/ternarybob/arbor` - All logging (no fmt.Println/log.Println)
- `github.com/ternarybob/banner` - Startup banners
- `github.com/pelletier/go-toml/v2` - TOML configuration

### Startup Sequence (main.go)
1. Configuration loading (`common.LoadFromFile`)
2. Logger initialization (`common.InitLogger`)
3. Banner display (`common.PrintBanner`)
4. Version management (`common.GetVersion`)
5. Service initialization
6. Handler initialization
7. Information logging

### Directory Structure Rules

**`internal/common/` - Stateless Utilities**
- ✅ Pure functions without receivers
- ✅ No state (no struct fields)
- ❌ NO receiver methods
- Examples: `LoadFromFile()`, `InitLogger()`, `ValidateEmail()`

**`internal/services/` - Stateful Services**
- ✅ Receiver methods on service structs
- ✅ State in struct fields (db, logger, config)
- ❌ NO standalone public functions
- Examples: `(s *UserService) CreateUser()`, `(s *JiraService) ScrapeProjects()`

### Code Quality Rules
- **Function Length**: Max 80 lines (ideal: 20-40)
- **No Duplicate Functions**: One implementation per function name
- **Error Handling**: No ignored errors (`_ = ...`)
- **Logging**: Use arbor logger, not fmt/log packages
- **Single Responsibility**: One purpose per function
- **DRY Principle**: Extract common code to utilities

### Forbidden Patterns
- `fmt.Println()` - Use `logger.Info()` instead
- `log.Println()` - Use `logger.Info()` instead
- `_ = functionCall()` - Handle errors properly
- Functions >80 lines - Split into smaller functions
- Duplicate function names - Consolidate into one

## Examples

```bash
# Enforce compliance on current project
/go-enforce-compliance

# Enforce compliance on specific project
/go-enforce-compliance C:\development\aktis\aktis-parser

# Enforce compliance on relative path
/go-enforce-compliance ./my-service
```

## Output

Provides detailed compliance report:

### Analysis Summary
```
📊 Compliance Analysis Report

Project: aktis-parser
Files Scanned: 45 Go files

Violations Found:
  ❌ Logging violations: 12 instances
  ❌ Long functions: 8 functions (>80 lines)
  ❌ Duplicate functions: 3 duplicates
  ⚠️  Ignored errors: 5 instances
  ❌ Directory rule violations: 2 receiver methods in common/
```

### Refactoring Summary
```
✨ Compliance Refactoring Complete

Changes Applied:
  ✅ Replaced 12 fmt.Println with logger.Info
  ✅ Split 8 long functions into 24 focused functions
  ✅ Consolidated 3 duplicate functions
  ✅ Fixed 5 error handling issues
  ✅ Moved 2 functions from common/ to services/
  ✅ Added arbor logger integration
  ✅ Updated startup sequence in main.go

Files Modified: 18
Functions Refactored: 32
Lines Changed: 456
```

### Validation Results
```
✓ All compliance checks passed
✓ Code compiles successfully
✓ No violations remaining
✓ 100% compliant with Go standards
```

## Safety

- Creates `.backup` files before modifications
- Preserves business logic and functionality
- Only refactors structure and patterns
- Validates with `go build` before completion
- Provides rollback instructions if issues occur

## When to Use

Use this command when:
- Existing Go project has code quality violations
- Hooks weren't active during initial development
- Legacy code needs standardization
- Pre-commit compliance enforcement failed
- Migrating old code to new standards

## Integration with Hooks

This command works alongside the pre-write/edit hooks:
- **Hooks**: Prevent new violations from being written
- **This Command**: Fixes existing violations in legacy code

Together they ensure:
- No new violations introduced (hooks)
- No existing violations remain (this command)
- Complete codebase compliance

---

**Agent**: go-refactor

**Prompt**: Analyze the Go project at "{{args}}" and enforce compliance with Go clean architecture standards and code quality rules.

## Compliance Enforcement Mission

Scan the entire codebase and systematically fix ALL violations of:
- Logging standards (use arbor, not fmt/log)
- Function length limits (max 80 lines)
- Duplicate function detection and consolidation
- Error handling (no ignored errors)
- Directory structure rules (common/ vs services/)
- Required libraries (arbor, banner, go-toml)
- Startup sequence order

## Approach

### Phase 1: Deep Analysis

1. **Scan All Go Files**
   - Search for all `.go` files recursively
   - Parse each file for compliance violations
   - Generate comprehensive violation map

2. **Violation Categorization**
   - Logging: fmt.Println, log.Println usage
   - Function Length: Count lines per function, flag >80 lines
   - Duplicates: Compare function signatures across all files
   - Error Handling: Find `_ = ` patterns
   - Directory Rules: Check receiver methods in common/
   - Libraries: Verify arbor/banner imports and usage
   - Startup: Validate main.go sequence

3. **Impact Assessment**
   - Count total violations by category
   - Identify high-priority fixes
   - Plan refactoring strategy

### Phase 2: Systematic Refactoring

**Priority Order:**
1. Add missing libraries (arbor, banner)
2. Replace logging violations (fmt → arbor)
3. Fix error handling (handle all errors)
4. Consolidate duplicate functions
5. Split long functions (>80 lines)
6. Move code to correct directories
7. Fix startup sequence

**For Each Violation:**
1. Identify exact file and line numbers
2. Plan fix that preserves functionality
3. Create backup of original file
4. Apply refactoring
5. Verify syntax with `go fmt`

### Phase 3: Validation

1. **Compliance Re-Check**
   - Re-scan all files
   - Verify zero violations remain
   - Confirm all standards met

2. **Build Validation**
   - Run `go mod tidy`
   - Run `go build`
   - Ensure no compile errors

3. **Reporting**
   - Generate before/after comparison
   - List all changes made
   - Provide compliance certificate

## Specific Refactoring Rules

### Logging Replacement Pattern
```go
// BEFORE (violation)
fmt.Println("Scraping projects")
log.Printf("Error: %v", err)

// AFTER (compliant)
logger.Info("Scraping projects")
logger.Error("Operation failed", "error", err)
```

### Function Splitting Pattern
```go
// BEFORE (violation - 120 lines)
func (s *Service) ProcessAll() error {
    // 120 lines of code...
}

// AFTER (compliant - split into focused functions)
func (s *Service) ProcessAll() error {
    if err := s.validateInput(); err != nil {
        return err
    }
    if err := s.fetchData(); err != nil {
        return err
    }
    return s.saveResults()
}

func (s *Service) validateInput() error {
    // 20 lines
}

func (s *Service) fetchData() error {
    // 30 lines
}

func (s *Service) saveResults() error {
    // 25 lines
}
```

### Duplicate Consolidation Pattern
```go
// BEFORE (violation - duplicates)
// File: services/jira.go
func GetPageCount(key string) int { ... }

// File: services/confluence.go
func GetPageCount(key string) int { ... }

// AFTER (compliant - consolidated)
// File: internal/common/pagination.go
func GetPageCount(client *http.Client, apiURL, key string) (int, error) {
    // Single implementation
}
```

### Error Handling Fix Pattern
```go
// BEFORE (violation)
_ = db.Close()
_ = file.Write(data)

// AFTER (compliant)
if err := db.Close(); err != nil {
    logger.Error("Failed to close database", "error", err)
}
if err := file.Write(data); err != nil {
    return fmt.Errorf("write failed: %w", err)
}
```

### Directory Structure Fix Pattern
```go
// BEFORE (violation - receiver in common/)
// File: internal/common/config.go
func (c *Config) Load() error {
    // Receiver method in common/ is forbidden
}

// AFTER (compliant - moved to services/)
// File: internal/services/config_service.go
type ConfigService struct {
    config *common.Config
}

func (s *ConfigService) Load() error {
    // Receiver method in services/ is correct
}

// File: internal/common/config.go
func LoadFromFile(path string) (*Config, error) {
    // Pure function in common/ is correct
}
```

## Expected Output Format

```markdown
# Compliance Enforcement Report

## Project: [name]
## Date: [timestamp]

---

## Phase 1: Analysis

### Files Scanned
- Total Go files: [count]
- Files with violations: [count]

### Violations Detected

#### Logging Violations (Priority: HIGH)
- [ ] [file:line] - fmt.Println usage
- [ ] [file:line] - log.Printf usage
Total: [count]

#### Function Length Violations (Priority: MEDIUM)
- [ ] [file:line] - [function_name] ([line_count] lines)
Total: [count]

#### Duplicate Functions (Priority: HIGH)
- [ ] [function_name] found in:
  - [file1:line1]
  - [file2:line2]
Total: [count]

#### Error Handling Violations (Priority: MEDIUM)
- [ ] [file:line] - Ignored error
Total: [count]

#### Directory Structure Violations (Priority: HIGH)
- [ ] [file:line] - Receiver method in common/
Total: [count]

#### Missing Libraries (Priority: CRITICAL)
- [ ] arbor logger not imported
- [ ] banner not imported
Total: [count]

**Total Violations: [count]**

---

## Phase 2: Refactoring

### Changes Applied

#### Logging Fixes
- ✅ Replaced [count] fmt.Println with logger.Info
- ✅ Replaced [count] log.Printf with logger.Error
- ✅ Added arbor logger initialization

#### Function Refactoring
- ✅ Split [function_name] ([old_lines] → [new_lines] lines)
- ✅ Extracted [helper_function] to internal/common/

#### Duplicate Consolidation
- ✅ Consolidated [function_name] from [file1] and [file2] into [new_location]

#### Error Handling Fixes
- ✅ Fixed [count] ignored errors

#### Directory Restructuring
- ✅ Moved [function_name] from common/ to services/

#### Library Integration
- ✅ Added arbor logger to go.mod
- ✅ Integrated banner display in main.go

### Files Modified
- [file1] - [change_summary]
- [file2] - [change_summary]

Total: [count] files

---

## Phase 3: Validation

### Compliance Re-Check
✓ Logging violations: 0
✓ Function length violations: 0
✓ Duplicate functions: 0
✓ Error handling violations: 0
✓ Directory violations: 0
✓ Required libraries: All present

**100% Compliant**

### Build Validation
✓ `go mod tidy` - Success
✓ `go build` - Success
✓ No compile errors
✓ All tests pass

---

## Summary

**Before:**
- Total violations: [count]
- Compliance score: [percentage]%

**After:**
- Total violations: 0
- Compliance score: 100%

**Impact:**
- Files modified: [count]
- Functions refactored: [count]
- Lines changed: [count]
- Backups created: [count]

✅ **Project is now fully compliant with Go clean architecture standards**

---

## Next Steps

1. Review `.backup` files to verify changes
2. Run full test suite: `go test ./...`
3. Commit changes: `git add . && git commit -m "refactor: enforce Go compliance standards"`
4. Delete `.backup` files if satisfied: `rm **/*.backup`

## Rollback Instructions

If issues occur:
```bash
# Restore from backups
for file in $(find . -name "*.backup"); do
    mv "$file" "${file%.backup}"
done
```
```

## Important Notes

1. **Preserves Business Logic**: Only refactors structure, doesn't change functionality
2. **Creates Backups**: All modified files backed up as `.backup`
3. **Validates Compilation**: Ensures code still builds after refactoring
4. **Follows Standards**: Applies exact same rules as pre-write/edit hooks
5. **Comprehensive**: Scans and fixes entire codebase, not just new code

## Integration with Development Workflow

```bash
# Before committing code with violations
/go-enforce-compliance

# After cloning legacy project
/go-enforce-compliance ./legacy-app

# Periodic compliance audit
/go-enforce-compliance && git commit -m "refactor: compliance enforcement"
```

This command ensures that existing codebases meet the same strict standards that hooks enforce on new code.
