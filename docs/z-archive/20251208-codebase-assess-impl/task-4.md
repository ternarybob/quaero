# Task 4: Create multi-language test fixture

Depends: - | Critical: no | Model: sonnet

## Addresses User Intent

Creates test fixture with Go, Python, JS files for language-agnostic testing

## Do

- Create `test/fixtures/multi_lang_project/` directory
- Add: README.md, go.mod, Makefile, main.go
- Add: pkg/utils.go
- Add: scripts/setup.py
- Add: web/package.json, web/index.js
- Add: docs/architecture.md

## Accept

- [ ] test/fixtures/multi_lang_project/ directory exists
- [ ] At least 10 files across 3 languages (Go, Python, JS)
- [ ] README.md contains build/run/test instructions
