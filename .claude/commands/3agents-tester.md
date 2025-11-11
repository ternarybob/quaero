---
name: 3agents-tester
description: read 3agents output (folder), m, review if api and ui tets eixts, update to match the requirements, run the test, report back
---

Test implementation from: $ARGUMENTS

## RULES
**Read from:** `docs/features/{task}/`
**Create tests:** Use @test-writer in `/test/api` or `/test/ui`
**Run tests:** Execute both test suites
**Report:** Simple pass/fail with issues

---

## PROCESS

### 1. Read 3agents Output
From `docs/features/{task}/`:
- `plan.md` - what was planned
- `progress.md` - what was done
- `summary.md` - completion status

### 2. Identify Test Needs
```markdown
# Test Plan

## Coverage Needed
- Step {N}: {what} - Test: {test name} - Exists: {yes|no}
- Step {N}: {what} - Test: {test name} - Exists: {yes|no}

## Tests to Create
- {test name} in /test/{ui|api}
```

### 3. Create Tests (@test-writer)
For each missing test:
- Use @test-writer skill
- Follow existing patterns in `/test/{ui|api}/`
- Keep simple

### 4. Run Tests
```bash
cd /test/ui && go test -v
cd /test/api && go test -v
```

### 5. Report

`test-results.md`:
```markdown
# Test Results: {task}

**Status:** {PASS ✅ | FAIL ❌}

## Tests Run
- ✅ Test{Name} - Step {N}
- ❌ Test{Name} - Step {N}
  - Error: {brief}
  - Fix: {what to do}
- ✅ Test{Name} - Step {N}

**Pass Rate:** {N}/{N} ({XX}%)

## Next Steps

{IF PASS:}
✅ Implementation validated - ready to use

{IF FAIL:}
❌ Issues found - run: `3agents "Fix test failures from docs/features/{task}/test-results.md"`
```

If failures, also create `fixes-needed.md`:
```markdown
# Fixes Needed

1. **{Issue}** - Step {N}
   - Problem: {brief}
   - Fix: {specific action}
   - Files: {list}

2. **{Issue}** - Step {N}
   - Problem: {brief}
   - Fix: {specific action}
   - Files: {list}

## Resume
`3agents "Fix: {brief description}"`
```

---

**Task:** $ARGUMENTS  
**Mode:** Fast test and report