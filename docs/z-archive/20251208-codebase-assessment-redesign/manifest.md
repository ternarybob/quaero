# Fix: Codebase Assessment Redesign

- Slug: codebase-assessment-redesign | Type: fix | Date: 2025-12-08
- Request: "Rethink the process for large codebase assessment to provide index, summary, and map. Review workers and provide actionable recommendations."
- Prior: docs/fix/code_assessment/prompt_2.md

## User Intent

Analyze and redesign the codebase assessment pipeline to:
1. Support ANY codebase (not just C/C++)
2. Create a navigable **index** of the codebase
3. Generate a comprehensive **summary** document
4. Build a structural **map** of the codebase
5. Enable users to ask questions via chat about: how to build, what it does, how to test

## Success Criteria

- [ ] Documented analysis of current pipeline gaps
- [ ] Recommendations for language-agnostic assessment
- [ ] Proposed pipeline steps with worker mappings
- [ ] Test specification for TDD implementation
- [ ] Actionable implementation plan for 3agents
