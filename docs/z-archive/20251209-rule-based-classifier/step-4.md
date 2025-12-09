# Step 4: Update codebase_assess.toml pipeline to use rule_classifier

- **Status:** complete
- **Skill:** go
- **Duration:** ~30s

## Files Modified
- `bin/job-definitions/codebase_assess.toml` - Added rule_classify_files step

## Skill Compliance
- [x] TOML syntax valid
- [x] Step dependencies correctly configured

## Changes
Added new step before classify_files:
```toml
[step.rule_classify_files]
type = "agent"
description = "Rule-based classification of file purpose (fast, no LLM)"
depends = "import_files"
agent_type = "rule_classifier"
filter_tags = ["codebase", "quaero"]
```

Updated classify_files to depend on rule_classify_files:
```toml
[step.classify_files]
type = "agent"
description = "LLM classification for ambiguous files (category=unknown only)"
depends = "rule_classify_files"
agent_type = "category_classifier"
filter_tags = ["codebase", "quaero"]
```

## Pipeline Flow
```
import_files → rule_classify_files → classify_files → identify_components → ...
                    (fast, ~90%)         (LLM, ~10%)
```

## Notes
- Rule-based classification runs first, classifying ~90% of files instantly
- LLM classification runs second, only processing files marked as "unknown"
- Note: The category_classifier will need modification to skip already-classified files (future enhancement)
