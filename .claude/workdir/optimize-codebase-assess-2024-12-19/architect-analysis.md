# ARCHITECT Analysis: Codebase Assessment Optimization

## Problem Statement
The current `codebase_assess.toml` job definition takes too long for large codebases (1k-5k files) because:

1. **import_files (LocalDirWorker)** - Reads FULL content of EVERY file
   - For 5k files: 5k file reads, 5k document storage operations
   - Each document stores full file content (memory/storage intensive)

2. **classify_files, extract_build_info, identify_components (AgentWorker)**
   - Runs LLM agents on EVERY document
   - For 5k documents: 5k LLM API calls!
   - This is the MAJOR bottleneck (minutes per file = hours/days total)

3. **generate_index, generate_summary, generate_map (SummaryWorker)**
   - Tries to include ALL documents in single LLM call
   - Exceeds token limits on large codebases
   - Often fails or produces truncated output

## Analysis of Existing Code

### code_map_worker.go (ALREADY OPTIMIZED)
The CodeMapWorker is already efficient:
- Builds hierarchical directory tree in-memory
- Stores METADATA only (not full content):
  - LOC, languages, exports, imports
  - File count, directory count, sizes
- Creates lightweight summary documents
- Skip binary files automatically

**Key insight**: code_map is sufficient for initial assessment!

### local_dir_worker.go (BOTTLENECK)
- Reads and stores FULL file content as documents
- Creates 1 document per file
- Not needed for initial code analysis

### agent_worker.go (BOTTLENECK)
- Creates 1 LLM job per document
- Already supports `batch_mode=true` for inline processing
- The category_classifier, entity_recognizer, metadata_enricher are AI-based

### summary_worker.go
- Truncates content at 50k chars per doc
- Has `filter_limit` option (default 1000)
- Needs tighter limits for large codebases

## Optimization Strategy

### Phase 1: Fast Initial Assessment (NEW)
Create `codebase_assess_fast.toml` that:
1. Uses ONLY `code_map` step - no `import_files`
2. Generates summary directly from code_map documents
3. Completes in seconds/minutes instead of hours

### Phase 2: Enhance code_map (OPTIONAL)
Add "key file" detection to code_map:
- README.md, package.json, go.mod, Cargo.toml, etc.
- Main entry points (main.go, index.ts, etc.)
- Build/config files (Makefile, docker-compose.yml, etc.)

### Phase 3: Selective Import (DEFERRED)
For deep analysis (commit/PR updates), import only:
- Files changed in the commit/PR
- Key configuration files
- Files referenced by changed files

## Recommendation

**DO NOT create new workers.** Use existing code_map with optimized job definition.

### Files to Create/Modify:
1. **CREATE** `deployments/local/job-definitions/codebase_assess_fast.toml` - Optimized version
2. **MODIFY** `deployments/local/job-definitions/codebase_assess.toml` - Add filter_limit to summaries

### Why This Approach:
- EXTEND existing `code_map` worker functionality (already optimized)
- REUSE existing summary worker with `filter_limit`
- NO new code needed - only configuration changes
- Follows ANTI-CREATION BIAS principle

## Architecture Decision

Use `code_map` as the PRIMARY data source for initial assessment:

```
codebase_assess_fast (NEW - seconds/minutes):
  step.code_map → step.generate_summary (from code_map docs)

codebase_assess (EXISTING - hours, for deep analysis):
  step.code_map + step.import_files → agent steps → summaries
```

## Justification for Changes

| Change | Why |
|--------|-----|
| New job definition | Different use case - fast vs. deep analysis |
| filter_limit on summaries | Prevent token overflow on large codebases |
| Skip agent steps | Not needed for initial structural analysis |
| Skip import_files | code_map already has structure metadata |
