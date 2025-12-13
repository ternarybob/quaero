# Task 4: Update devops_enrich.toml to remove extract_structure step

Depends: 1 | Critical: no | Model: sonnet

## Addresses User Intent

The extract_structure step in the pipeline is no longer valid; update dependencies to skip it

## Do

- Edit `bin/job-definitions/devops_enrich.toml`
- Remove `[step.extract_structure]` section entirely
- Update `analyze_build_system` step: change `depends = "extract_structure"` to `depends = "import_files"`
- Update `classify_devops` step: change `depends = "extract_structure"` to `depends = "import_files"`
- Update `build_dependency_graph` step: change `depends = "extract_structure, classify_devops"` to `depends = "import_files, classify_devops"`
- Also update `test/config/job-definitions/devops_enrich.toml` and `test/bin/job-definitions/devops_enrich.toml` if they exist

## Accept

- [ ] No extract_structure step in bin/job-definitions/devops_enrich.toml
- [ ] Dependencies updated to reference import_files instead
- [ ] Test config versions also updated
