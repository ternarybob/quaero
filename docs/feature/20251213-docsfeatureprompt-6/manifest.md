# Manifest — docsfeatureprompt_6.md

## Workdir

- Type: feature
- Date: 2025-12-13
- Slug: docsfeatureprompt-6
- Workdir: `docs/feature/20251213-docsfeatureprompt-6`

## Request

- Input reference: `docsfeatureprompt_6.md`
- Status: **BLOCKED** — file not found in repository root or under `docs/`

### Evidence (file missing)

- `test -f docsfeatureprompt_6.md` → `MISSING`
- `rg --files | rg -i 'docsfeatureprompt(_|-)?6\\.md'` → no matches
- `git ls-files | rg -i 'docsfeatureprompt'` → no matches

## Architecture Requirements Loaded (Phase 0.2)

- `docs/architecture/manager_worker_architecture.md`
- `docs/architecture/QUEUE_LOGGING.md`
- `docs/architecture/QUEUE_UI.md`
- `docs/architecture/QUEUE_SERVICES.md`
- `docs/architecture/workers.md`

## Constraints (Global Rules)

- Architecture docs in `docs/architecture/` define correctness.
- Allowed tests only: `test/api`, `test/ui`.
- Binaries: `go build -o /tmp/<name>` only; never write binaries to repo root.

## Intended Work (Once request file is available)

1. Read the prompt file (`docsfeatureprompt_6.md`) to understand required changes.
2. Implement changes in the codebase consistent with the queue architecture docs.
3. Run allowed build/test commands (as applicable) and document results.
4. Produce `step-N.md` and `validation-N.md` evidence.

