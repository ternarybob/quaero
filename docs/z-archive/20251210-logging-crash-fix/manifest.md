# Fix: Logging Crash and TRACE Level Issues
- Slug: logging-crash-fix | Type: fix | Date: 2025-12-10
- Request: "1. Service crashed, without any logging generated. 2. Appears trace logs are marked as INFO. This should NOT be the case. Clean contextual logging is required - a worker has NO context of the UI, and should not be logging for the UI."
- Prior: none

## User Intent
Fix two logging-related issues:
1. **Silent crash** - Service crashes without generating any log output, making debugging impossible
2. **TRACE logs at wrong level** - Debug/trace statements are using `.Info()` instead of `.Trace()` or `.Debug()`, polluting production logs

The user wants clean, contextual logging where:
- Workers log at appropriate levels (TRACE/DEBUG for internal diagnostics)
- Crashes are properly logged before the service terminates
- Log levels accurately reflect the importance of messages

## Success Criteria
- [ ] TRACE messages use `.Trace()` or `.Debug()` level, not `.Info()`
- [ ] Panic recovery flushes logs before terminating
- [ ] No more "TRACE:" prefix in INFO-level logs
- [ ] Service crash produces useful diagnostic output

## Skills Assessment
| Skill | Path | Exists | Relevant | Reason |
|-------|------|--------|----------|--------|
| go | .claude/skills/go/SKILL.md | Yes | Yes | Go code changes in workers and logging |
| frontend | .claude/skills/frontend/SKILL.md | Yes | No | No frontend changes needed |

**Active Skills:** go
