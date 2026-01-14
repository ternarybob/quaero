# Codebuff Configuration

This directory contains Codebuff slash commands and skills.

## Commands

| Command | Description |
|---------|-------------|
| `/3agents` | Adversarial multi-agent workflow for high-quality code changes |

### Usage

```
/3agents implement feature X for component Y
```

The command executes autonomously through phases:
1. **ARCHITECT** - Analyzes requirements, creates step docs
2. **WORKER** - Implements each step
3. **VALIDATOR** - Reviews each step (hostile stance)
4. **FINAL VALIDATOR** - Reviews all changes together
5. **DOCUMENTARIAN** - Updates architecture docs

## Skills

| Skill | Path | Purpose |
|-------|------|--------|
| Refactoring | `skills/refactoring/SKILL.md` | Core code modification patterns |
| Go | `skills/go/SKILL.md` | Go language patterns |
| Frontend | `skills/frontend/SKILL.md` | Alpine.js + Bulma patterns |
| Adversarial Workflow | `skills/adversarial-workflow/SKILL.md` | Multi-agent workflow details |
| Test Architecture | `skills/test-architecture/SKILL.md` | Test patterns and output requirements |

## Workdir

Each `/3agents` execution creates a workdir:

```
.codebuff/workdir/YYYY-MM-DD-HHMM-task-name/
├── requirements.md         # Extracted requirements
├── architect-analysis.md   # Patterns and decisions
├── step_N.md              # Step specifications
├── step_N_impl.md         # Implementation notes
├── step_N_valid.md        # Validation results
├── final_validation.md    # Final review
├── summary.md             # Final summary (REQUIRED)
├── architecture-updates.md # Doc changes
└── logs/
    ├── build_step*.log    # Per-step build output
    ├── build_final.log    # Final build
    └── test_final.log     # Final test run
```

## Key Principles

1. **CORRECTNESS over SPEED** - Quality is paramount
2. **Requirements are LAW** - No interpretation allowed
3. **EXISTING PATTERNS ARE LAW** - Match codebase style
4. **CLEANUP IS MANDATORY** - Remove dead code
5. **NO STOPPING** - Execute autonomously without prompts
6. **OUTPUT CAPTURE** - All command output to log files
