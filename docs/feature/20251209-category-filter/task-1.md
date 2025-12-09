# Task 1: Add nested metadata filtering to search common.go

- **Depends:** none
- **Skill:** go
- **Critical:** no
- **Files:** internal/services/search/common.go

## Actions
- Modify `matchesMetadata` to support dot-notation for nested fields
- Support `rule_classifier.category` to access nested map values
- Support multi-value matching with comma-separated values

## Acceptance
- [ ] `rule_classifier.category=source` matches docs with that nested value
- [ ] `rule_classifier.category=build,config` matches either value
- [ ] Existing flat metadata filters still work
