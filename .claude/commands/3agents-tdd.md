---
name: 3agents-tdd
description: TDD enforcement - tests are IMMUTABLE, fix code until tests pass
---

Execute: $ARGUMENTS

**Read first:** `.claude/skills/refactoring/SKILL.md`

## INPUT VALIDATION
```
Must be *_test.go file or STOP
```

## FUNDAMENTAL RULE
```
┌─────────────────────────────────────────────────────────────────┐
│ TESTS ARE IMMUTABLE LAW                                         │
│                                                                  │
│ • Touch a test file = FAILED                                    │
│ • Weaken an assertion = FAILED                                  │
│ • Skip/delete a test = FAILED                                   │
│                                                                  │
│ Test expects X, code returns Y → FIX THE CODE                   │
└─────────────────────────────────────────────────────────────────┘
```

## WORKFLOW

### PHASE 0: UNDERSTAND
1. Read test file - extract requirements
2. Read skills for applicable patterns:
   - `.claude/skills/refactoring/SKILL.md` - Core patterns
   - `.claude/skills/go/SKILL.md` - Go changes
   - `.claude/skills/frontend/SKILL.md` - Frontend changes
   - `.claude/skills/monitoring/SKILL.md` - UI tests (screenshots, monitoring, results)

### PHASE 1: RUN TEST
```bash
go test -v -run "Test.*" {test_file}
```
- **ALL PASS →** Complete
- **ANY FAIL →** Fix & Iterate

### PHASE 2: FIX & ITERATE (max 5)
```
TEST FAILS
     │
     ▼
┌─────────────────────────┐
│ ANALYZE                 │
│ • Test is RIGHT         │
│ • Code is WRONG         │
└───────────┬─────────────┘
            ▼
┌─────────────────────────┐
│ FIX (apply skills)      │
│ • Modify code ONLY      │
│ • EXTEND > MODIFY > CREATE│
│ • Follow Go/Frontend skill│
│ • Run build             │
└───────────┬─────────────┘
            ▼
┌─────────────────────────┐
│ VERIFY                  │
│ • No test files changed │
│ • Re-run tests          │
└───────────┬─────────────┘
     ┌──────┴──────┐
     ▼             ▼
   FAIL          PASS → Complete
     │
     └──► Loop
```

### PHASE 3: COMPLETE
- All tests pass
- No test files modified

## FORBIDDEN (AUTO-FAIL)

| Action | Result |
|--------|--------|
| Modify `*_test.go` | FAILURE |
| Add `t.Skip()` | FAILURE |
| Change expected values | FAILURE |
| Weaken assertions | FAILURE |

## INVOKE
```
/test-iterate test/ui/job_definition_test.go
```