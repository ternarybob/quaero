# Plan: Refactor Job Definition System to Support AI Job Type

## Executive Summary

This refactoring introduces a new `JobDefinitionTypeAI` to replace the overloaded use of "custom" type for AI/agent jobs. The new AI job type will support:

1. **Free-text action definitions** - Users can define custom AI actions (not limited to predefined action types)
2. **Flexible document filters** - Advanced filtering by tags, source_type, date ranges, and custom criteria
3. **Multiple AI operation types**:
   - **Scan**: Add metadata/tags/keywords to existing documents
   - **Enrich**: Add web-sourced information to documents
   - **Generate**: Create new documentation from existing documents

The existing "custom" type remains for non-AI custom workflows, maintaining backward compatibility.

## Dependency Analysis

```
Foundation (Models) → Manager → Examples → Documentation
        ↓                ↓
   Validation    →   Registration
```

**Critical Path**: Model changes → Manager updates → Validation logic

**Parallel Opportunities**: Example files and documentation can be created in parallel with manager updates

## Critical Path Flags

- Step 2: api-breaking (adds new job type, changes validation)
- Step 3: medium complexity (manager logic changes)

## Execution Groups

### Group 1 (Sequential - Foundation)

#### 1. Add JobDefinitionTypeAI constant and update validation
- **Skill:** @go-coder
- **Files:** `internal/models/job_definition.go`
- **Complexity:** medium
- **Critical:** yes:api-breaking
- **Depends on:** none
- **User decision:** no
- **Description:** Add `JobDefinitionTypeAI = "ai"` constant, update `IsValidJobDefinitionType()`, add AI-specific validation logic for action fields and document filters

### Group 2 (Parallel - Implementation)

These can run simultaneously after Group 1:

#### 2a. Enhance document filter support in AgentManager
- **Skill:** @go-coder
- **Files:** `internal/jobs/queue/managers/agent_manager.go`
- **Complexity:** medium
- **Critical:** no
- **Depends on:** Step 1
- **User decision:** no
- **Sandbox:** worker-a
- **Description:** Extend `queryDocuments()` to support tags filter, created_after, updated_after, and flexible filter options

#### 2b. Add AI action field validation
- **Skill:** @go-coder
- **Files:** `internal/models/job_definition.go`
- **Complexity:** low
- **Critical:** no
- **Depends on:** Step 1
- **User decision:** no
- **Sandbox:** worker-b
- **Description:** Add `AIAction` struct and validation for AI job types with operation_type field (scan/enrich/generate)

#### 2c. Update manager registration to handle AI type
- **Skill:** @go-coder
- **Files:** `internal/jobs/definitions/orchestrator.go`
- **Complexity:** low
- **Critical:** no
- **Depends on:** Step 1
- **User decision:** no
- **Sandbox:** worker-c
- **Description:** Update `JobDefinitionOrchestrator` to map "ai" type to AgentManager

### Group 3 (Sequential - Examples and Documentation)

Runs after Group 2 completes:

#### 3a. Create example AI job definitions
- **Skill:** @none
- **Files:** `deployments/local/job-definitions/ai-keyword-extractor.toml`, `deployments/local/job-definitions/ai-web-enricher.toml`
- **Complexity:** low
- **Critical:** no
- **Depends on:** 2a, 2b, 2c
- **User decision:** no
- **Description:** Create example AI job definition TOML files demonstrating scan, enrich, and generate operations

#### 3b. Update keyword-extractor to use AI type
- **Skill:** @none
- **Files:** `deployments/local/job-definitions/keyword-extractor-agent.toml`
- **Complexity:** low
- **Critical:** no
- **Depends on:** 2a, 2b, 2c
- **User decision:** no
- **Description:** Update existing keyword extractor example to use type = "ai" instead of type = "agent"

### Group 4 (Sequential - Verification)

#### 4. Verification and Testing
- **Skill:** @go-coder
- **Files:** All Go files in project
- **Complexity:** low
- **Critical:** yes:build-verification
- **Depends on:** 3a, 3b
- **User decision:** no
- **Description:** Compile codebase and verify no regressions

## Parallel Execution Map

```
[Step 1: Add AI type*] ───┬──> [Step 2a: Enhance filters] ──┐
                          ├──> [Step 2b: Add validation]   ──┤
                          └──> [Step 2c: Update orchestrator]─┼─> [Step 3a: Examples] ──┬─> [Step 4: Verify*]
                                                              └─> [Step 3b: Update keyword-extractor] ─┘

* = High complexity or critical
```

## Final Review Triggers

Steps flagged for Opus final review:
- Step 1: api-breaking (new job type added)
- Step 4: build-verification

## Success Criteria

1. **New AI Job Type Added:**
   - `JobDefinitionTypeAI = "ai"` constant exists
   - Validation accepts "ai" as valid job type
   - AI jobs can specify free-text actions in step.action field

2. **Enhanced Document Filtering:**
   - Support for `tags` filter (e.g., `tags = ["wheelers-hill"]`)
   - Support for `source_type` filter
   - Support for date filters (`created_after`, `updated_after`)
   - Support for `limit` filter

3. **AI Operation Types Defined:**
   - Documentation clearly explains scan/enrich/generate operations
   - Examples demonstrate each operation type

4. **Backward Compatibility:**
   - Existing "custom" type jobs continue to work
   - Existing "agent" action type still supported for AI jobs
   - No breaking changes to existing job definitions

5. **Clean Compilation:**
   - `go build ./...` succeeds without errors
   - All existing tests pass

6. **Example Job Definitions:**
   - At least 2 new AI job definition examples created
   - keyword-extractor updated to use AI type
   - Examples demonstrate tags filter usage

## Implementation Notes

### AI Job Type Structure

```toml
id = "keyword-extractor-ai"
name = "AI Keyword Extractor"
type = "ai"  # New AI job type
job_type = "user"
source_type = "ai"  # or keep existing source_type field blank for AI jobs
description = "Extract keywords using AI"

[[steps]]
name = "extract_keywords"
action = "scan_documents"  # Free-text action (not limited to predefined types)
on_error = "fail"

[steps.config]
# AI-specific config
operation_type = "scan"  # scan | enrich | generate
agent_type = "keyword_extractor"  # Specific AI agent to use
prompt = "Extract important keywords from this document"  # Optional custom prompt

# Document filter with enhanced capabilities
[steps.config.document_filter]
source_type = "crawler"  # Optional: filter by source type
tags = ["wheelers-hill", "important"]  # NEW: filter by tags
created_after = "2025-01-01T00:00:00Z"  # NEW: filter by creation date
updated_after = "2025-11-01T00:00:00Z"  # NEW: filter by update date
limit = 100  # Maximum documents to process
```

### Operation Types

1. **scan** - Process existing documents to add information
   - Add metadata (keywords, categories, sentiment)
   - Add tags for organization
   - Extract entities or concepts
   - **Does not modify document content**

2. **enrich** - Add external information to documents
   - Fetch related web content
   - Add context from APIs
   - Cross-reference with other documents
   - **May add new fields to document metadata**

3. **generate** - Create new documents from existing ones
   - Generate summaries
   - Create reports
   - Build indexes or catalogs
   - **Creates entirely new documents**

### Migration Path

Existing job definitions with `type = "custom"` and `action = "agent"` can be migrated to `type = "ai"` by:
1. Changing `type = "custom"` to `type = "ai"`
2. Optionally adding `operation_type` to step config
3. Optionally enhancing document filters with new fields

The system will continue to support both patterns for backward compatibility.

## Risk Assessment

**Low Risk:**
- Adding new constant (Step 1)
- Creating example files (Steps 3a, 3b)

**Medium Risk:**
- Validation logic changes (Step 1) - New validation paths for AI type
- Filter enhancement (Step 2a) - Additive change, doesn't break existing functionality
- Orchestrator updates (Step 2c) - Simple mapping addition

**High Risk:**
None - this is an additive feature that maintains backward compatibility

**Mitigation:**
- Keep existing "custom" type unchanged
- Add AI type as separate code path
- Extensive validation ensures no breaking changes
- Compile verification catches any integration issues
