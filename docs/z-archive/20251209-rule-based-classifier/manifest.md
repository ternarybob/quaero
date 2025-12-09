# Feature: Rule-Based File Classifier

- **Slug:** rule-based-classifier
- **Type:** feature
- **Date:** 2025-12-09
- **Request:** "Implement option 1 - Add a rule_classifier step before classify_files that handles obvious cases via pattern matching, then filter what goes to the LLM step. This reduces LLM calls by ~90% for large codebases."
- **Prior:** none

## User Intent
Add a rule-based pre-classification system that classifies files by filename patterns, directory structure, and extensions without LLM calls. Only ambiguous files (~10%) should be sent to the LLM-based category_classifier. This dramatically reduces cost and time for codebases with 1000+ files.

## Success Criteria
- [ ] Rule-based classifier worker implemented that classifies files by patterns
- [ ] Classification rules cover: test, build, ci, docs, config, source/entrypoint, data, script categories
- [ ] Existing classify_files step only processes files not already classified
- [ ] Integration tested with codebase_assess pipeline
- [ ] Build and tests pass

## Skills Required
- [x] go - Core implementation of rule_classifier worker and agent
- [ ] frontend - N/A
