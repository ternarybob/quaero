# Feature: Category-Based Filtering for Agent Steps

- **Slug:** category-filter
- **Type:** feature
- **Date:** 2025-12-09
- **Request:** "Add category-based filtering to agent_worker so LLM agent steps only process files matching specific categories from rule_classifier results"
- **Prior:** ./docs/feature/20251209-rule-based-classifier/

## User Intent
Reduce LLM calls in codebase_assess pipeline by filtering agent steps to only process relevant files based on their rule_classifier category. Currently `extract_build_info` and `identify_components` run on ALL 1000+ files. They should only run on files matching specific categories (e.g., build/config/docs for extract_build_info, source for identify_components).

## Success Criteria
- [ ] Agent steps can filter by document metadata category (from rule_classifier)
- [ ] `extract_build_info` only processes build, config, docs category files
- [ ] `identify_components` only processes source category files
- [ ] `classify_files` (LLM) only processes unknown category files
- [ ] Pipeline TOML updated with filter_category config
- [ ] Build and tests pass

## Skills Required
- [x] go - Modify agent_worker to filter by metadata category
- [ ] frontend - N/A
