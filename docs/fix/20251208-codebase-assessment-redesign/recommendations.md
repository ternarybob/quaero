# Codebase Assessment Pipeline Redesign

## Executive Summary

The current `devops_enrich.toml` pipeline is **limited to C/C++ codebases** and focuses on DevOps metadata extraction without producing the three key artifacts users need: an **index**, a **summary**, and a **map**. This document proposes a redesigned, language-agnostic pipeline that leverages existing workers more effectively and introduces LLM-based analysis for comprehensive codebase understanding.

### Implementation Principles

| Principle | Description |
|-----------|-------------|
| **Remove Redundant Code** | Actively identify and delete unused workers, dead code paths, and deprecated functionality. Do not preserve code "just in case." |
| **Breaking Changes OK** | Do NOT assess for backward compatibility. Clean, correct implementation takes priority over preserving existing interfaces or behavior. |

---

## Part 1: Current State Analysis

### Current Pipeline (`devops_enrich.toml`)

```
Step 1: extract_structure   → C/C++ regex patterns (includes, defines, platforms)
Step 2: analyze_build       → LLM analysis of build files (CMake, Makefile)
Step 3: classify_devops     → LLM classification of file roles
Step 4: dependency_graph    → Graph from extracted includes
Step 5: aggregate_summary   → DevOps summary (requires LLM)
```

### Critical Gaps

| Gap | Description | Impact |
|-----|-------------|--------|
| **C/C++ Only** | `extract_structure` uses hardcoded regex for `.c/.cpp/.h` files | Cannot process Go, Python, JS, Rust, Java, etc. |
| **No Index** | No searchable file listing with purpose/description | Users can't navigate or find relevant files |
| **No Map** | No visual or hierarchical structure representation | Users don't understand codebase layout |
| **No Summary** | `aggregate_summary` requires LLM (often unavailable in tests) | No high-level overview document |
| **No Content Storage** | Files indexed but content not stored in a chat-queryable format | Cannot answer "how do I build this?" |

### Available But Unused Workers

| Worker | Capability | Current Use |
|--------|------------|-------------|
| **Code Map Worker** | Creates hierarchical structure, LOC, languages per directory | **NOT USED** in devops_enrich |
| **Summary Worker** | LLM-powered summary from tagged documents | **NOT USED** in devops_enrich |
| **Agent Worker** | AI agents for classification, summarization, entity extraction | **NOT USED** in devops_enrich |
| **Local Dir Worker** | Imports file content as documents | Used for import only |

---

## Part 2: Redesigned Pipeline

### Goals

1. **Language-Agnostic**: Works with any codebase (Go, Python, JS, Rust, Java, C/C++, etc.)
2. **Three Artifacts**: Produces Index, Summary, and Map documents
3. **Chat-Ready**: Stores content in format suitable for RAG/chat queries
4. **Scalable**: Handles 10k+ files efficiently

### New Pipeline: `codebase_assess.toml`

```toml
# Codebase Assessment Pipeline - Language Agnostic
id = "codebase_assess"
name = "Codebase Assessment Pipeline"
type = "custom"
description = "Comprehensive codebase analysis producing index, summary, and map"
tags = ["assessment", "any-language"]
timeout = "4h"

# ============================================================================
# PHASE 1: IMPORT & INDEX (Parallel)
# ============================================================================

# Step 1: Create Hierarchical Code Map
# Uses CodeMapWorker to build directory tree with stats
[step.code_map]
type = "code_map"
description = "Build hierarchical code structure map"
dir_path = "{input_path}"
project_name = "{project_name}"
max_depth = 15
skip_summarization = true  # We'll do AI summarization separately

# Step 2: Import File Contents
# Uses LocalDirWorker to import actual file contents as documents
[step.import_files]
type = "local_dir"
description = "Import codebase files as documents"
dir_path = "{input_path}"
tags = ["codebase", "{project_name}"]
include_extensions = [".go", ".py", ".js", ".ts", ".java", ".rs", ".c", ".cpp", ".h", ".hpp", ".rb", ".php", ".cs", ".swift", ".kt", ".scala", ".md", ".txt", ".toml", ".yaml", ".yml", ".json"]
exclude_paths = [".git", "node_modules", "vendor", "__pycache__", "dist", "build", "target", ".venv"]
max_file_size = 1048576  # 1MB limit per file

# ============================================================================
# PHASE 2: ANALYSIS (Depends on Phase 1)
# ============================================================================

# Step 3: Classify Files with LLM Agent
# Uses AgentWorker with category_classifier agent type
[step.classify_files]
type = "agent"
description = "LLM classification of file purpose and role"
depends = "import_files"
agent_type = "category_classifier"
filter_tags = ["codebase", "{project_name}"]

# Step 4: Extract Build Instructions
# Uses AgentWorker to identify build/run/test commands
[step.extract_build_info]
type = "agent"
description = "Extract how to build, run, and test the project"
depends = "import_files"
agent_type = "metadata_enricher"
filter_tags = ["codebase", "{project_name}"]
filter_source_type = "local_dir"
# Focus on: README, Makefile, package.json, go.mod, Cargo.toml, etc.

# Step 5: Identify Key Components
# Uses AgentWorker to recognize architectural components
[step.identify_components]
type = "agent"
description = "Identify major components and entry points"
depends = "classify_files"
agent_type = "entity_recognizer"
filter_tags = ["codebase", "{project_name}"]

# ============================================================================
# PHASE 3: SYNTHESIS (Depends on Phase 2)
# ============================================================================

# Step 6: Build Dependency Graph
# Generic dependency extraction (not C/C++ specific)
[step.build_graph]
type = "dependency_graph"
description = "Build file dependency graph from analysis"
depends = "identify_components"
filter_tags = ["codebase", "{project_name}"]

# Step 7: Generate Codebase Index
# Creates searchable index document
[step.generate_index]
type = "summary"
description = "Generate codebase index document"
depends = "classify_files, code_map"
filter_tags = ["codebase", "{project_name}"]
prompt = |
  Create a comprehensive INDEX of this codebase. For each file:
  - File path
  - Purpose (one line)
  - Key exports/functions
  - Related files

  Organize by directory. Make it searchable.

# Step 8: Generate Codebase Summary
# Creates executive summary document
[step.generate_summary]
type = "summary"
description = "Generate codebase summary document"
depends = "extract_build_info, identify_components"
filter_tags = ["codebase", "{project_name}"]
prompt = |
  Create a comprehensive SUMMARY of this codebase covering:

  1. OVERVIEW: What does this project do? (2-3 sentences)
  2. ARCHITECTURE: Key components and how they interact
  3. BUILD: How to build the project (exact commands)
  4. RUN: How to run the project
  5. TEST: How to run tests
  6. KEY FILES: Most important files to understand first
  7. DEPENDENCIES: External libraries and tools needed

# Step 9: Generate Codebase Map
# Creates visual/hierarchical map document
[step.generate_map]
type = "summary"
description = "Generate codebase map document"
depends = "code_map, build_graph"
filter_tags = ["codebase", "{project_name}"]
prompt = |
  Create a structural MAP of this codebase:

  1. DIRECTORY TREE: Show folder structure with purpose annotations
  2. COMPONENT DIAGRAM: Major components and their relationships
  3. DATA FLOW: How data moves through the system
  4. ENTRY POINTS: Where execution starts
  5. EXTENSION POINTS: Where to add new functionality

[error_tolerance]
max_child_failures = 50
failure_action = "continue"
```

---

## Part 3: Worker Recommendations

### Step-to-Worker Mapping

| Step | Worker | Why This Worker |
|------|--------|-----------------|
| `code_map` | **CodeMapWorker** | Already creates hierarchical structure, LOC, languages |
| `import_files` | **LocalDirWorker** | Imports file content as searchable documents |
| `classify_files` | **AgentWorker** (`category_classifier`) | LLM-based, language-agnostic classification |
| `extract_build_info` | **AgentWorker** (`metadata_enricher`) | Extracts build/run/test from READMEs, configs |
| `identify_components` | **AgentWorker** (`entity_recognizer`) | Identifies architectural entities |
| `build_graph` | **DependencyGraphWorker** | Needs modification to be language-agnostic |
| `generate_index` | **SummaryWorker** | LLM synthesis of index document |
| `generate_summary` | **SummaryWorker** | LLM synthesis of summary document |
| `generate_map` | **SummaryWorker** | LLM synthesis of map document |

### Required Worker Modifications

#### 1. DependencyGraphWorker Enhancement

**Current**: Builds graph from C/C++ `includes` metadata only
**Needed**: Generic dependency detection from:
- Import statements (Go, Python, JS, Java, Rust)
- `require` calls (Node.js)
- Use declarations (Rust)
- Package references (C#, Java)

**Action**: Modify `dependency_graph_worker.go` to:
1. Accept language hint or auto-detect
2. Use LLM for dependency extraction when patterns unknown
3. Fall back to metadata-based graph when available

#### 2. ExtractStructureWorker Deletion

**Current**: Regex patterns for C/C++ only
**Action**: **DELETE** `extract_structure_worker.go` entirely
- C/C++ regex approach does not scale to other languages
- AgentWorker + LLM provides superior language-agnostic analysis
- No backward compatibility concerns - breaking changes are acceptable

#### 3. New Agent Types for AgentWorker

Add these agent types to support codebase assessment:

```go
// In agent_service.go or similar
const (
    AgentTypeBuildExtractor    = "build_extractor"     // Extract build/run/test commands
    AgentTypeArchitectureMap   = "architecture_mapper" // Identify architectural patterns
    AgentTypeFileIndexer       = "file_indexer"        // Create per-file summaries
)
```

---

## Part 4: Test Specification (TDD)

### New Test: `TestCodebaseAssessment_FullFlow`

```go
// test/ui/codebase_assessment_test.go

func TestCodebaseAssessment_FullFlow(t *testing.T) {
    // Setup: Use a multi-language test fixture (Go + Python + JS)

    // Phase 1: Import & Index
    // - Assert: Documents created for each file
    // - Assert: Code map document exists with directory structure
    // - Assert: File count matches expected

    // Phase 2: Analysis
    // - Assert: Files have classification metadata
    // - Assert: Build info extracted from README/configs
    // - Assert: Components identified

    // Phase 3: Synthesis
    // - Assert: INDEX document exists with file listings
    // - Assert: SUMMARY document exists with build instructions
    // - Assert: MAP document exists with structure

    // Validation
    // - Assert: Can search for "how to build" and get relevant results
    // - Assert: Can search for file by purpose
    // - Assert: Dependencies linked correctly
}

// Test assertions for each artifact
func assertIndexDocument(t *testing.T, doc *models.Document) {
    // Must contain file paths
    assert.Contains(t, doc.ContentMarkdown, "src/")
    // Must have purpose descriptions
    assert.Contains(t, doc.ContentMarkdown, "Purpose:")
    // Must be organized by directory
    assert.Contains(t, doc.ContentMarkdown, "##")
}

func assertSummaryDocument(t *testing.T, doc *models.Document) {
    // Must have overview section
    assert.Contains(t, doc.ContentMarkdown, "OVERVIEW")
    // Must have build instructions
    assert.Contains(t, doc.ContentMarkdown, "BUILD")
    // Must have run instructions
    assert.Contains(t, doc.ContentMarkdown, "RUN")
    // Must have test instructions
    assert.Contains(t, doc.ContentMarkdown, "TEST")
}

func assertMapDocument(t *testing.T, doc *models.Document) {
    // Must have directory tree
    assert.Contains(t, doc.ContentMarkdown, "DIRECTORY")
    // Must have component info
    assert.Contains(t, doc.ContentMarkdown, "COMPONENT")
    // Must have entry points
    assert.Contains(t, doc.ContentMarkdown, "ENTRY")
}
```

### Test Fixture: Multi-Language Project

Create `test/fixtures/multi_lang_project/`:
```
multi_lang_project/
├── README.md           # Project overview, build/run/test instructions
├── go.mod              # Go module definition
├── Makefile            # Build commands
├── main.go             # Go entry point
├── pkg/
│   └── utils.go        # Go utilities
├── scripts/
│   └── setup.py        # Python setup script
├── web/
│   ├── package.json    # Node.js config
│   └── index.js        # JS entry point
└── docs/
    └── architecture.md # Architecture docs
```

---

## Part 5: Implementation Tasks for 3agents

### Task 0: Assess and Remove Redundant Code

**Scope**: `internal/queue/workers/`, `bin/job-definitions/`
**Action**:
- Delete `extract_structure_worker.go` (C/C++ specific, replaced by AgentWorker + LLM)
- Delete `devops_enrich.toml` job definition (superseded by `codebase_assess.toml`)
- Remove any dead code paths in DependencyGraphWorker that only handle C/C++ includes
- Delete associated tests for removed workers
**Acceptance**: No unused worker code remains; build passes

### Task 1: Create New Job Definition

**File**: `bin/job-definitions/codebase_assess.toml`
**Action**: Create new pipeline as specified in Part 2
**Acceptance**: Job definition loads and validates

### Task 2: Add Multi-Language Test Fixture

**Dir**: `test/fixtures/multi_lang_project/`
**Action**: Create test fixture with Go, Python, JS files
**Acceptance**: Fixture has at least 10 files across 3 languages

### Task 3: Implement Test Spec

**File**: `test/ui/codebase_assessment_test.go`
**Action**: Implement TDD test as specified in Part 4
**Acceptance**: Test runs (may fail initially - that's TDD)

### Task 4: Enhance DependencyGraphWorker

**File**: `internal/queue/workers/dependency_graph_worker.go`
**Action**: Add language-agnostic dependency detection via LLM
**Acceptance**: Graph builds for Go/Python/JS imports

### Task 5: Add New Agent Types

**File**: `internal/services/agent_service.go` (or equivalent)
**Action**: Add `build_extractor`, `architecture_mapper`, `file_indexer`
**Acceptance**: Agent types available for use in job definitions

### Task 6: Run Tests & Iterate

**Action**: Run `go test ./test/ui/... -run Codebase`
**Acceptance**: All assertions pass

---

## Part 6: API Endpoints for Chat

To enable chat-based queries ("How do I build this?"), ensure these endpoints work:

### Search Endpoint
```
GET /api/search?q=how+to+build&tags=codebase,{project}
```

### Summary Endpoint
```
GET /api/devops/summary?project={project}
```

### Map Endpoint
```
GET /api/devops/graph?project={project}
```

---

## Appendix: Comparison

| Aspect | Current (`devops_enrich`) | Proposed (`codebase_assess`) |
|--------|---------------------------|------------------------------|
| Languages | C/C++ only | Any language |
| Index | None | Full file index with purposes |
| Summary | DevOps-focused (often empty) | Comprehensive with build/run/test |
| Map | Dependency graph only | Hierarchical + component diagram |
| Chat-ready | No | Yes (documents stored for search) |
| LLM Required | Yes (often unavailable) | Yes (essential for quality) |
| Workers Used | 5 | 9 (including code_map, summary) |
