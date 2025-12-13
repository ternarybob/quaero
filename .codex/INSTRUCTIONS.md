# Codex Operating Contract (MANDATORY)

This repository enforces strict, auditable workflows.
Codex MUST follow these rules for all non-trivial work.

---

## Defined Workflows

### 1. 3agents — Architecture-Governed Implementation

The **3agents workflow** is defined in:
- `.codex/prompts/3agents.md`

It enforces:
1. WORKER implements changes
2. VALIDATOR adversarially checks against architecture docs
3. ITERATION until PASS or limit reached

---

### 2. 3agents-tdd — Test-Driven Iteration

The **3agents-tdd workflow** is defined in:
- `.codex/prompts/3agents-tdd.md`

It enforces:
- Tests as the specification
- No modification of test files unless explicitly allowed
- Iterative fix → test → validate loop
- Architecture compliance during fixes

---

## Workflow Selection Rules (NON-NEGOTIABLE)

Codex MUST select the workflow as follows:

### Use **3agents** when the request involves:
- A feature
- A fix
- A refactor
- Architecture-sensitive changes
- Non-test-driven implementation work
