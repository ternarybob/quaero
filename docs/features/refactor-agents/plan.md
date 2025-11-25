# Plan: TOML Job Definition File Updates

## Summary

The refactoring in commit 1c58fae reverted the job type naming from `"ai"` to `"agent"` in the Go code, but TOML job definition files were not updated to match. This plan addresses two issues:

1. **File naming inconsistency**: AI-related TOML files use `ai-` prefix instead of `agent-` prefix
2. **Model alignment**: TOML `type` field uses `"ai"` but per the requirements, `JobDefinitionTypeAI` should remain `"ai"` - this is intentional

## Current State Analysis

### deployments/local/job-definitions/
| Current Filename | Type Field | Issue |
|------------------|------------|-------|
| ai-web-enricher.toml | `type = "ai"` | Filename uses "ai-" prefix |
| ai-document-generator.toml | `type = "ai"` | Filename uses "ai-" prefix |
| keyword-extractor-agent.toml | `type = "ai"` | Correct naming (uses agent suffix) |
| news-crawler.toml | `type = "crawler"` | No issues |
| nearby-restaurants-places.toml | `type = "places"` | No issues |

### bin/job-definitions/
| Current Filename | Type Field | Issue |
|------------------|------------|-------|
| keyword-extractor-agent.toml | `type = "custom"` | Uses "custom" but should use "ai" |
| news-crawler.toml | `type = "crawler"` | No issues |
| nearby-restaurants-places.toml | `type = "places"` | No issues |

### test/config/job-definitions/
| Current Filename | Type Field | Issue |
|------------------|------------|-------|
| keyword-extractor-agent.toml | `type = "custom"` | Uses "custom" but is agent job |
| test-agent-job.toml | `type = "custom"` | Uses "custom" but is agent job |
| news-crawler.toml | `type = "crawler"` | No issues |
| nearby-restaurants-places.toml | `type = "places"` | No issues |

## Key Clarifications

Per `requirements.md`:
- **JobDefinitionType** (`internal/models/job_definition.go`): Uses `type = "ai"` in TOML files
- **QueueJob.Type**: Uses `"agent"` in Go code (after refactor)
- This intentional separation is documented: "Job definitions use `type = "ai"` (JobDefinitionType), but queue jobs use `type = "agent"` (QueueJob.Type)"

## Execution Groups

### Group 1 (Parallel - Rename AI files in deployments/local)

1a. **Rename ai-web-enricher.toml to agent-web-enricher.toml**
    - Skill: @code-architect
    - Files: `deployments/local/job-definitions/ai-web-enricher.toml`
    - Critical: no
    - Depends on: none
    - Action: Rename file, update comments to use "agent" terminology

1b. **Rename ai-document-generator.toml to agent-document-generator.toml**
    - Skill: @code-architect
    - Files: `deployments/local/job-definitions/ai-document-generator.toml`
    - Critical: no
    - Depends on: none
    - Action: Rename file, update comments to use "agent" terminology

### Group 2 (Parallel - Add missing agent files to bin/job-definitions)

2a. **Create agent-web-enricher.toml in bin/job-definitions**
    - Skill: @code-architect
    - Files: `bin/job-definitions/agent-web-enricher.toml`
    - Critical: no
    - Depends on: 1a
    - Action: Copy from deployments/local with appropriate defaults

2b. **Create agent-document-generator.toml in bin/job-definitions**
    - Skill: @code-architect
    - Files: `bin/job-definitions/agent-document-generator.toml`
    - Critical: no
    - Depends on: 1b
    - Action: Copy from deployments/local with appropriate defaults

### Group 3 (Sequential - Fix test config type fields)

3. **Update test TOML files to use correct type = "ai"**
   - Skill: @code-architect
   - Files:
     - `test/config/job-definitions/keyword-extractor-agent.toml`
     - `test/config/job-definitions/test-agent-job.toml`
     - `bin/job-definitions/keyword-extractor-agent.toml`
   - Critical: no
   - Depends on: none
   - Action: Change `type = "custom"` to `type = "ai"` for agent jobs

### Group 4 (Sequential - Verification)

4. **Run build and tests**
   - Skill: @test-writer
   - Files: None (verification)
   - Critical: no
   - Depends on: Groups 1-3
   - Action: `go build ./...` and `go test ./...`

## Execution Map

```
[1a] ──┬──> [2a] ──┐
       │          ├──> [4]
[1b] ──┴──> [2b] ──┤
                   │
[3] ──────────────┘
```

## File Changes Summary

### Renames
- `deployments/local/job-definitions/ai-web-enricher.toml` -> `agent-web-enricher.toml`
- `deployments/local/job-definitions/ai-document-generator.toml` -> `agent-document-generator.toml`

### New Files
- `bin/job-definitions/agent-web-enricher.toml`
- `bin/job-definitions/agent-document-generator.toml`

### Modifications
- `test/config/job-definitions/keyword-extractor-agent.toml` - change type to "ai"
- `test/config/job-definitions/test-agent-job.toml` - change type to "ai"
- `bin/job-definitions/keyword-extractor-agent.toml` - change type to "ai"

## Success Criteria

- All agent-related TOML files use `agent-` prefix in filename
- All agent job definitions use `type = "ai"` (JobDefinitionTypeAI)
- `bin/job-definitions/` contains all example agent job definitions
- Build succeeds
- Tests pass
