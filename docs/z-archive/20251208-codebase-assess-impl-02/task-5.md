# Task 5: Create codebase_assess.toml job definition

Depends: - | Critical: no | Model: sonnet

## Addresses User Intent

Create the new language-agnostic pipeline as specified in Part 2 of recommendations.md

## Do

- Create `bin/job-definitions/codebase_assess.toml` with the 9-step pipeline
- Include: code_map, import_files, classify_files (agent), extract_build_info (agent), identify_components (agent), build_graph, generate_index (summary), generate_summary (summary), generate_map (summary)
- Follow the exact structure from recommendations.md Part 2

## Accept

- [ ] codebase_assess.toml exists in bin/job-definitions/
- [ ] Contains all 9 steps with correct types and dependencies
- [ ] Job definition validates (can be loaded)
