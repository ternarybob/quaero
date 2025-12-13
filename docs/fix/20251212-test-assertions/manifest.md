# Fix: Test Assertions for Codebase Classify
- Slug: test-assertions | Type: fix | Date: 2025-12-12
- Request: "docs\feature\prompt_5.md"
- Prior: docs/fix/20251212-websocket-log-debounce/ (related - fixed underlying debounce issues)

## User Intent
The test `test\ui\job_definition_codebase_classify_test.go` does not have proper assertions to validate the UI behavior. The user wants:

1. **Test WITHOUT page refresh**: Monitor the queue page in real-time without refreshing every 10 seconds
2. **API call count assertion**: Verify that Step Log API requests are < 10 (with ~5000 log entries over ~130 seconds). Exclude service logs.
3. **Auto-expand verification**: All steps should auto-expand in order of step completion (import_files/code_map → rule_classify_files) on the running job WITHOUT page refresh
4. **Log display assertion**: Step 1 and Step 2 should show logs starting from line 1 (not line 5), showing 1 → 15 logs

## Success Criteria
- [ ] Test monitors job WITHOUT page refresh (uses WebSocket/real-time updates)
- [ ] Test asserts Step Log API request count < 10 (excluding service logs)
- [ ] Test asserts all steps auto-expand in order of completion
- [ ] Test asserts Step 1 (import_files) shows logs 1 → 15 (not 5 → 15)
- [ ] Test asserts Step 2 (code_map) shows logs 1 → 15
- [ ] Test passes when run with `go test -v ./ui -run TestJobDefinitionCodebaseClassify`

## Skills Assessment
| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | ✅ | ✅ | Test code is Go, using chromedp for UI testing |
| frontend | .claude/skills/frontend/SKILL.md | ✅ | ❌ | Not modifying frontend code |

**Active Skills:** go
