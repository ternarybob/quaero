---
name: 3agents
description: Adversarial worker/validator workflow enforcing architecture docs
---

Execute the following request strictly using this workflow:

$ARGUMENTS

You are operating inside a real repository.
All file reads, writes, and commands MUST be real and relative to the repo root.

---

## OVERVIEW

This is a strict adversarial workflow with enforced iteration.

Logical roles (not concurrent):
1. WORKER — implements changes
2. VALIDATOR — adversarially checks against architecture requirements
3. ITERATION — repeat until PASS or max iterations reached

Architecture docs are hard requirements, not guidance.

---

## CONFIG

```yaml
architecture_docs: docs/architecture/
max_iterations: 3
workdir_root: ./docs
```

---

## GLOBAL RULES (NON-NEGOTIABLE)

- Architecture docs define correctness
- Partial compliance is FAIL
- Validator is adversarial and pessimistic
- Evidence must be concrete (file paths, line numbers, diffs)
- No silent assumptions

### Build & Test Constraints
- Allowed tests ONLY:
  - /test/api
  - /test/ui
- Binaries:
  - go build -o /tmp/<name>
  - NEVER write binaries to repo root

---

## PHASE 0 — SETUP (ONCE)

### Step 0.1 — Create Workdir

Determine:
- type = feature | fix
- slug = kebab-case from request
- date = YYYYMMDD

```bash
mkdir -p ./docs/{type}/{date}-{slug}
```
Set:
```
workdir = ./docs/{type}/{date}-{slug}
```

---

### Step 0.2 — Load Architecture Requirements

```bash
cat docs/architecture/manager_worker_architecture.md
cat docs/architecture/QUEUE_LOGGING.md
cat docs/architecture/QUEUE_UI.md
cat docs/architecture/QUEUE_SERVICES.md
cat docs/architecture/workers.md
```

If any file is missing → STOP and FAIL.

---

### Step 0.3 — Write Manifest

WRITE `{workdir}/manifest.md`

---

## PHASE 1 — WORKER IMPLEMENTATION

For iteration N (starting at 1):

1. Read `{workdir}/manifest.md`
2. Re-read architecture docs
3. Implement changes
4. Run build/tests
5. Document work

WRITE `{workdir}/step-{N}.md`

---

## PHASE 2 — VALIDATOR (ADVERSARIAL)

Validator assumes implementation is wrong until proven otherwise.

WRITE `{workdir}/validation-{N}.md`

---

## PHASE 3 — ITERATION CONTROL

- FAIL and N < max_iterations → iterate
- PASS → write summary and STOP
- Max iterations reached → STOP and report blockers
