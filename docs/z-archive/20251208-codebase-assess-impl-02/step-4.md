# Step 4: Update devops_enrich.toml to remove extract_structure step
Model: sonnet | Status: ✅

## Done
- Removed: `[step.extract_structure]` section (lines 27-34) from all three TOML files
- Updated: `[step.analyze_build_system]` depends from "extract_structure" to "import_files"
- Updated: `[step.classify_devops]` depends from "extract_structure" to "import_files"
- Updated: `[step.build_dependency_graph]` depends from "extract_structure, classify_devops" to "import_files, classify_devops"
- Updated: Step numbering comments (Step 3→2, Step 4→3, Step 5→4, Step 6→5)

## Files Changed
- `bin/job-definitions/devops_enrich.toml` - Removed extract_structure step and updated dependencies
- `test/config/job-definitions/devops_enrich.toml` - Same changes
- `test/bin/job-definitions/devops_enrich.toml` - Same changes

## Build Check
Build: ⏳ | Tests: ⏭️
