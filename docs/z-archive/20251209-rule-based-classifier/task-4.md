# Task 4: Update codebase_assess.toml pipeline to use rule_classifier

- **Depends:** 3
- **Skill:** go
- **Critical:** no
- **Files:** bin/job-definitions/codebase_assess.toml

## Actions
- Add new step `rule_classify_files` that uses rule_classifier agent
- This step runs BEFORE the existing `classify_files` step
- Update `classify_files` step to depend on `rule_classify_files`
- The LLM classifier will only process files that rule_classifier marked as "unknown"

## New Step Configuration
```toml
[step.rule_classify_files]
type = "agent"
description = "Rule-based classification of file purpose (no LLM)"
depends = "import_files"
agent_type = "rule_classifier"
filter_tags = ["codebase", "quaero"]
```

## Updated classify_files Step
```toml
[step.classify_files]
type = "agent"
description = "LLM classification for ambiguous files only"
depends = "rule_classify_files"
agent_type = "category_classifier"
filter_tags = ["codebase", "quaero"]
# Note: category_classifier should skip files already classified by rule_classifier
```

## Acceptance
- [ ] Pipeline has rule_classify_files step
- [ ] rule_classify_files depends on import_files
- [ ] classify_files depends on rule_classify_files
- [ ] Pipeline TOML is valid syntax
