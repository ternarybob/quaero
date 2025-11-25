# Complete: TOML Job Definition File Updates

## Stats
- Steps: 4 (1a, 1b, 2a, 2b, 3)
- Parallel Groups: 2 (Group 1: renames, Group 2: bin copies)
- Quality: 10/10

## Worker Summaries

### Step 1a-1b: Rename AI TOML Files

**Actions:**
1. Renamed `ai-web-enricher.toml` to `agent-web-enricher.toml`
2. Renamed `ai-document-generator.toml` to `agent-document-generator.toml`
3. Updated header comments from "AI" to "Agent"
4. Updated description comments to use "agent" terminology
5. Preserved `type = "ai"` (JobDefinitionType constant - intentionally stays "ai")
6. Updated tags arrays from `["ai", ...]` to `["agent", ...]`

**Files Changed:**
- `deployments/local/job-definitions/agent-web-enricher.toml` - Created
- `deployments/local/job-definitions/agent-document-generator.toml` - Created
- `deployments/local/job-definitions/ai-web-enricher.toml` - Deleted
- `deployments/local/job-definitions/ai-document-generator.toml` - Deleted

### Step 2a-2b: Create bin agent TOML files

**Actions:**
1. Copied `agent-web-enricher.toml` to `bin/job-definitions/`
2. Copied `agent-document-generator.toml` to `bin/job-definitions/`

**Files Changed:**
- `bin/job-definitions/agent-web-enricher.toml` - Created (2215 bytes)
- `bin/job-definitions/agent-document-generator.toml` - Created (2582 bytes)

### Step 3: Fix Test TOML Type Fields

**Actions:**
1. Updated `type = "custom"` to `type = "ai"` in all agent job definition files
2. Updated comments to reflect agent job type

**Files Changed:**
- `bin/job-definitions/keyword-extractor-agent.toml` - Updated type to "ai"
- `test/config/job-definitions/keyword-extractor-agent.toml` - Updated type to "ai"
- `test/config/job-definitions/test-agent-job.toml` - Updated type to "ai"

## Review

**Status:** APPROVED

**Key Decisions:**
- File naming convention: `agent-*.toml` for agent job definitions
- `type = "ai"` preserved in TOML files (JobDefinitionType constant)
- Queue jobs use `"agent"` type internally (after Go code refactor)

## Verify

```bash
go build ./...     # PASS
go test ./internal/models/... -run TestJobDefinition  # PASS
```

## Final State

### deployments/local/job-definitions/
| Filename | Type Field |
|----------|------------|
| agent-web-enricher.toml | `type = "ai"` |
| agent-document-generator.toml | `type = "ai"` |
| keyword-extractor-agent.toml | `type = "ai"` |
| news-crawler.toml | `type = "crawler"` |
| nearby-restaurants-places.toml | `type = "places"` |

### bin/job-definitions/
| Filename | Type Field |
|----------|------------|
| agent-web-enricher.toml | `type = "ai"` |
| agent-document-generator.toml | `type = "ai"` |
| keyword-extractor-agent.toml | `type = "ai"` |
| news-crawler.toml | `type = "crawler"` |
| nearby-restaurants-places.toml | `type = "places"` |

### test/config/job-definitions/
| Filename | Type Field |
|----------|------------|
| keyword-extractor-agent.toml | `type = "ai"` |
| test-agent-job.toml | `type = "ai"` |
| news-crawler.toml | `type = "crawler"` |
| nearby-restaurants-places.toml | `type = "places"` |

**Done:** 2025-11-26T09:20:00Z
