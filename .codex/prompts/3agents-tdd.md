---
name: 3agents-tdd
description: Execute a Go test file and iteratively fix code until all tests pass (architecture-compliant)
---

Execute the following request strictly as specified:

$ARGUMENTS

You are operating inside a real Go code repository.
All file reads/writes and commands MUST be real and relative to the repo root.

---

## INPUT GATE (MANDATORY)

A single Go test file MUST be provided.

Rules:
- File must exist
- File name must end with `_test.go`

If invalid → STOP immediately.

---

## CONFIG

```yaml
max_iterations: 5
architecture_docs: docs/architecture/
workdir_root: ./docs/test-fix
```

---

## GLOBAL RULES (NON-NEGOTIABLE)

- **Tests define the specification**
- **Tests MUST NOT be modified**
- Only implementation code may change
- Fixes MUST comply with architecture docs
- Partial success is FAIL
