# Progress

| Task | Status | Notes |
|------|--------|-------|
| 1 | ✅ | Added Depends field to JobStep struct |
| 2 | ✅ | Refactored TOML parsing - supports both [step.name] and [[steps]] |
| 3 | ✅ | Updated validation for flat filter_* fields and depends validation |
| 4 | ✅ | Updated agent_manager to read flat filter_* fields |
| 5 | ✅ | Updated all test TOML files to new format |
| 6 | ✅ | Updated bin/ and deployments/ TOML files to new format |
| 7 | ✅ | Code is clean - removed backward compatibility per user request |

Deps: [x] 1→[2] [x] 2→[3] [x] 3→[4] [x] 4→[5] [x] 5→[6] [x] 6→[7]

## Completion
All tasks completed successfully. Build passes. Breaking change: old `[[steps]]` format removed.
