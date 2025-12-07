# Feature: DevOps Enrichment Pipeline for C/C++ Codebase Analysis

- Slug: devops-enrichment-pipeline | Type: feature | Date: 2025-12-07
- Request: "Implement a multi-pass enrichment pipeline that transforms raw C/C++ source files into DevOps-actionable knowledge"
- Prior: none

## User Intent

Enable a DevOps engineer to understand a large, legacy C/C++ codebase (2GB, 30 years old) well enough to build CI/CD pipelines - without requiring C/C++ programming expertise. The pipeline should:

1. Extract structural information from C/C++ files (includes, defines, platform conditionals)
2. Analyze build systems (Makefiles, CMakeLists.txt, vcxproj) for targets, flags, dependencies
3. Classify files by role, component, test type using LLM
4. Build a dependency graph across all documents
5. Generate an actionable DevOps summary/guide for CI/CD

Each pass adds metadata tags to documents, building a queryable knowledge base.

## Success Criteria

- [ ] 5 job actions registered: `extract_structure`, `analyze_build_system`, `classify_devops`, `build_dependency_graph`, `aggregate_devops_summary`
- [ ] DevOps metadata schema implemented with all specified fields (includes, defines, platforms, build_targets, file_role, component, test_type, etc.)
- [ ] File detection patterns working for C/C++ source files and build files
- [ ] `devops_enrich` job definition runs passes in correct sequence
- [ ] API endpoints implemented: GET /api/devops/summary, /components, /graph, /platforms; POST /api/devops/enrich
- [ ] LLM prompts crafted for DevOps-focused classification and summary generation
- [ ] Dependency graph stored in KV storage under `devops:dependency_graph`
- [ ] Summary stored as searchable document with ID `devops-summary`
- [ ] Unit tests for all 5 actions pass
- [ ] API integration tests pass
- [ ] Long-running integration test with progress monitoring passes (following queue_test.go pattern)
- [ ] Test fixtures created in `test/fixtures/cpp_project/`
- [ ] Handles 1000+ file corpus without timeout/memory issues
- [ ] Idempotency via `enrichment_passes` tracking
- [ ] Error handling with retry/backoff for LLM failures
