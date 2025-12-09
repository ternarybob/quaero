# Task 3: Update codebase_assess.toml pipeline with category filters

- **Depends:** 2
- **Skill:** go
- **Critical:** no
- **Files:** bin/job-definitions/codebase_assess.toml

## Actions
- Add `filter_category` to each agent step
- Configure appropriate categories per step

## Changes
```toml
[step.rule_classify_files]
# No filter - runs on all imported files
filter_tags = ["codebase", "quaero"]
agent_type = "rule_classifier"

[step.classify_files]
# Only process files rule_classifier couldn't classify
filter_category = ["unknown"]
filter_tags = ["codebase", "quaero"]
agent_type = "category_classifier"

[step.extract_build_info]
# Only build-related files need build info extraction
filter_category = ["build", "config", "docs"]
filter_tags = ["codebase", "quaero"]
agent_type = "metadata_enricher"

[step.identify_components]
# Only source files need component identification
filter_category = ["source"]
filter_tags = ["codebase", "quaero"]
agent_type = "entity_recognizer"
```

## Acceptance
- [ ] classify_files filters to unknown category
- [ ] extract_build_info filters to build/config/docs
- [ ] identify_components filters to source
- [ ] Pipeline TOML valid syntax
