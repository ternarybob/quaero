# Fix: Codebase Assessment Implementation

- Slug: codebase-assess-impl | Type: fix | Date: 2025-12-08
- Request: "Implement recommendations from docs/fix/20251208-codebase-assessment-redesign/recommendations.md"
- Prior: docs/fix/20251208-codebase-assessment-redesign/

## User Intent

Implement the codebase assessment pipeline redesign:
1. Remove redundant code (extract_structure_worker, devops_enrich.toml)
2. Create new language-agnostic codebase_assess.toml pipeline
3. Add multi-language test fixture
4. Implement TDD test for the new pipeline
5. Enhance DependencyGraphWorker for language-agnostic detection
6. Add new agent types for codebase assessment

Breaking changes are acceptable. Do NOT preserve backward compatibility.

## Success Criteria

- [ ] extract_structure_worker.go deleted
- [ ] devops_enrich.toml job definition deleted or replaced
- [ ] codebase_assess.toml created with 9-step pipeline
- [ ] Multi-language test fixture exists (Go + Python + JS)
- [ ] TestCodebaseAssessment_FullFlow test implemented
- [ ] Build passes with all changes
- [ ] Tests pass (or fail gracefully for TDD)
