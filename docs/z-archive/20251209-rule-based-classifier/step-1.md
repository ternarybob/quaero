# Step 1: Create rule_classifier agent with pattern-based classification

- **Status:** complete
- **Skill:** go
- **Duration:** ~2min

## Files Modified
- `internal/services/agents/rule_classifier.go` - Created new rule-based classifier agent

## Skill Compliance
- [x] Error wrapping with context - N/A (no errors returned from Execute)
- [x] Structured logging (arbor) - N/A (pure computation, no logging needed)
- [x] Interface-based DI - Implements AgentExecutor interface
- [x] Constructor injection - N/A (stateless agent)

## Implementation Details
- Created `RuleClassifier` struct implementing `AgentExecutor` interface
- Defined 45+ classification rules covering:
  - Test files (Go, JS, Python, integration, mocks)
  - CI/CD (GitHub Actions, GitLab CI, Jenkins, CircleCI)
  - Build files (Docker, Make, CMake)
  - Dependencies (go.mod, package.json, Cargo.toml, etc.)
  - Documentation (README, CHANGELOG, LICENSE, markdown)
  - Configuration (env, yaml, toml, json, editor configs)
  - Source code entrypoints (main.go, index.js, app.py)
  - Interface definitions (protobuf, graphql, openapi)
  - Scripts (shell, powershell, batch)
  - Data files (sql, csv)
  - General source code by language

## Output Format
Matches `category_classifier` output format for compatibility:
```json
{
  "category": "source|test|config|docs|build|ci|script|data|unknown",
  "subcategory": "specific-type",
  "purpose": "Brief description",
  "importance": "high|medium|low",
  "tags": ["tag1", "tag2"],
  "rule_matched": "rule-name"
}
```

## Notes
- Rules are processed in priority order (first match wins)
- Test patterns checked before source patterns to correctly classify `*_test.go`
- Supports cross-platform path matching (normalizes `\` to `/`)
- Files not matching any rule return `category: "unknown"` for LLM fallback
