# Fix: Codebase Assessment Pipeline Implementation
- Slug: codebase-assessment-impl-03 | Type: fix | Date: 2025-12-09
- Request: "Fix codebase assessment pipeline - UAT shows 'no documents found matching tags: [codebase {project_name}]', code_map step requires dir_path, and agent types (metadata_enricher, category_classifier, entity_recognizer) are not implemented"
- Prior: docs/fix/20251209-codebase-assessment-redesign/, docs/fix/20251208-codebase-assess-impl-02/

## User Intent
Make the Codebase Assessment Pipeline functional:
1. Fix the placeholder `{project_name}` issue - tags should use concrete values or be properly substituted
2. Implement or wire up the missing agent types that the pipeline requires
3. Ensure the pipeline can process documents and complete successfully

## Success Criteria
- [ ] Pipeline completes without "no documents found matching tags" error
- [ ] Agent types metadata_enricher, category_classifier, entity_recognizer are available/implemented
- [ ] Test `test\ui\codebase_assessment_test.go` passes or shows pipeline completing
- [ ] UAT `bin\job-definitions\codebase_assess.toml` works with real projects
