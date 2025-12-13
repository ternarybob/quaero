# Task 3: Create codebase_assess.toml job definition

Depends: - | Critical: no | Model: sonnet

## Addresses User Intent

Creates new language-agnostic pipeline for codebase assessment

## Do

- Create `bin/job-definitions/codebase_assess.toml` with 9-step pipeline from recommendations.md
- Create `test/config/job-definitions/codebase_assess.toml` (copy for test config)

## Accept

- [ ] codebase_assess.toml exists in bin/job-definitions/
- [ ] codebase_assess.toml exists in test/config/job-definitions/
- [ ] Pipeline has all 9 steps: code_map, import_files, classify_files, extract_build_info, identify_components, build_graph, generate_index, generate_summary, generate_map
