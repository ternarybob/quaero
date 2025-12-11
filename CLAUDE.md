# Quaero

## Stack
- Go 1.25+, BadgerDB, Arbor logging, TOML config
- Alpine.js + Spectre CSS (no HTMX)
- Google ADK with Gemini

## Build
- Use `scripts/build.ps1` or `scripts/build.sh` - never direct `go build`
- Binaries to `/tmp/` only
- Tests in `test/api/` and `test/ui/`

## Skills
- `.claude/skills/go/SKILL.md` - Go patterns
- `.claude/skills/frontend/SKILL.md` - Frontend patterns

## Workflow
- `.claude/commands/3agents-skills.md` - Multi-agent workflow
- Invoke with `/3agents-skills {request}`