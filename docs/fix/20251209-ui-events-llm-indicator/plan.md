# Plan: Remove Step Queue Stats and Verify LLM Indicator

Type: fix | Workdir: ./docs/fix/20251209-ui-events-llm-indicator/

## User Intent (from manifest)
1. Events should clearly indicate whether a worker is making an LLM call or a rule-based operation
2. Remove the inaccurate step queue stats ("X pending, Y running, Z completed, W failed") from step cards

## Active Skills
go, frontend

## Tasks
| # | Desc | Depends | Critical | Model | Skill |
|---|------|---------|----------|-------|-------|
| 1 | Remove step progress stats from queue.html | - | no | sonnet | frontend |
| 2 | Verify LLM indicator in agent_worker.go | - | no | sonnet | go |
| 3 | Build and test | 1,2 | no | sonnet | go |

## Order
[1,2] → [3]

## Analysis

### Issue 1: Step Queue Stats
The step cards display `item.step.progress` which shows:
- `pending`, `running`, `completed`, `failed` counts

This data comes from either:
1. `_stepProgress[stepDef.name]` - WebSocket updates
2. `parentJob.pending_children` etc - Fallback using parent job's aggregate child stats

**Problem**: Both sources show GLOBAL queue stats, not step-specific counts. A step that completed hours ago still shows "918 pending" because it's reading the parent job's current totals.

**Solution**: Remove the step progress stats display entirely (lines 191-198 in queue.html).

### Issue 2: LLM Indicator
Current implementation already exists in `agent_worker.go`:
- `IsRuleBased()` check determines prefix
- `logPrefix := "AI"` or `logPrefix = "Rule"`
- Events show: `"AI: category_classifier"` or `"Rule: rule_classifier"`

**Verification needed**: Confirm this is working correctly in logs.
