# Plan: Rule-Based File Classifier

- **Type:** feature
- **Workdir:** ./docs/feature/20251209-rule-based-classifier/
- **Skills:** go

## Overview

Add a rule-based pre-classification agent that classifies files by filename patterns, directory structure, and extensions without LLM calls. Only ambiguous files (~10%) will be sent to the existing LLM-based category_classifier.

## Tasks

| # | Description | Depends | Skill | Critical | Files |
|---|-------------|---------|-------|----------|-------|
| 1 | Create rule_classifier agent with pattern-based classification | - | go | no | internal/services/agents/rule_classifier.go |
| 2 | Register rule_classifier in agent service | 1 | go | no | internal/services/agents/service.go |
| 3 | Add rule_classifier to valid agent types in agent_worker | 2 | go | no | internal/queue/workers/agent_worker.go |
| 4 | Update codebase_assess.toml pipeline to use rule_classifier | 3 | go | no | bin/job-definitions/codebase_assess.toml |

## Execution Order
[1] → [2] → [3] → [4] sequential (each depends on previous)

## Classification Rules

The rule_classifier will use these pattern-based rules:

| Pattern | Category | Subcategory |
|---------|----------|-------------|
| `*_test.go`, `*.test.js`, `*.spec.ts`, `/test/` | test | unit-test |
| `*_integration_test.go`, `/integration/` | test | integration-test |
| `Dockerfile*`, `docker-compose*` | build | container |
| `Makefile`, `CMakeLists.txt`, `*.ninja` | build | build-system |
| `.github/workflows/*`, `.gitlab-ci.yml`, `Jenkinsfile` | ci | pipeline |
| `/docs/`, `*.md`, `README*`, `CHANGELOG*` | docs | documentation |
| `main.go`, `main.py`, `index.js`, `app.py`, `/cmd/*/main.go` | source | entrypoint |
| `.env*`, `config.*`, `/config/`, `*.toml`, `*.yaml` in root | config | configuration |
| `*.json`, `*.csv`, `*.sql` in `/data/` | data | dataset |
| `*.sh`, `*.ps1`, `*.bat`, `/scripts/` | script | automation |
| `go.mod`, `go.sum`, `package.json`, `Cargo.toml` | build | dependency |
| `*.proto`, `*.graphql`, `*.swagger.*` | source | interface |
| `*_mock.go`, `*_stub.go`, `/mocks/` | test | mock |

Files not matching any pattern get `category: "unknown"` and should be passed to LLM classifier.

## Validation Checklist
- [ ] All tests pass: `go test ./internal/services/agents/...`
- [ ] Build succeeds: `go build -o /tmp/quaero ./cmd/quaero`
- [ ] rule_classifier correctly classifies test files
- [ ] rule_classifier correctly classifies config files
- [ ] rule_classifier returns "unknown" for ambiguous files
- [ ] Pipeline can use rule_classifier as a step
