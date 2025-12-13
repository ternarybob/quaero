---
name: 3agents
description: Adversarial worker/validator loop enforcing architecture docs
---

Execute the following request strictly according to this workflow:

$ARGUMENTS

You are operating inside a real code repository.
All file reads/writes and commands MUST be real and relative to the repo root.

---

## OVERVIEW

This is a **strict adversarial implementation loop**.

Roles (logical, not concurrent):
1. **WORKER** — implements the requested change
2. **VALIDATOR** — adversarially validates against architecture docs
3. **ITERATION** — repeat until PASS or max iterations reached

Architecture docs are **hard requirements**, not guidelines.

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
- Partial compliance is **FAIL**
- Validator is adversarial and pessimistic
- Evidence must be concrete (file paths, code snippets, diffs)
- No silent assumptions

### Build & Test
- Only allowed test commands:
  - `/test/api`
  - `/test/ui`
- Binaries:
  - `go build -o /tmp/<name>`
  - NEVER write binaries to repo root

---

## PHASE 0 — SETUP (ONCE)

### Step 0.1 — Create Workdir

Determine:
- `{type}` = feature | fix
- `{slug}` = kebab-case from request
- `{date}` = YYYYMMDD

```bash
mkdir -p ./docs/{type}/{date}-{slug}
```

Set:
```text
workdir = ./docs/{type}/{date}-{slug}
```

---

### Step 0.2 — Load Architecture Requirements

Mandatory inputs:

```bash
cat docs/architecture/manager_worker_architecture.md
cat docs/architecture/QUEUE_LOGGING.md
cat docs/architecture/QUEUE_UI.md
cat docs/architecture/QUEUE_SERVICES.md
cat docs/architecture/workers.md
```

If any file is missing → **STOP and FAIL**.

---

### Step 0.3 — Write Manifest

**WRITE** `{workdir}/manifest.md`
