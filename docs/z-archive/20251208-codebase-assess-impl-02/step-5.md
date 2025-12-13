# Step 5: Create codebase_assess.toml
Model: sonnet | Status: ✅

## Done
- Created: New job definition file `bin/job-definitions/codebase_assess.toml`
- Implemented: Language-agnostic codebase assessment pipeline with 3 phases:
  - Phase 1: Import & Index (code_map, import_files in parallel)
  - Phase 2: Analysis (classify_files, extract_build_info, identify_components)
  - Phase 3: Synthesis (build_graph, generate_index, generate_summary, generate_map)
- Configured: Support for 20+ file extensions across multiple languages
- Added: Three output documents (Index, Summary, Map) with detailed prompts

## Files Changed
- `bin/job-definitions/codebase_assess.toml` - New file (107 lines)

## Build Check
Build: ⏳ | Tests: ⏭️
