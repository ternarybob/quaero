# Task 4: Remove Redundant Client-Side Expansion Computation
Depends: 3 | Critical: no | Model: sonnet | Skill: frontend

## Addresses User Intent
"The UI is using code to trigger expansion and status, this is NOT required and should be driven from the backend"

## Skill Patterns to Apply
From frontend/SKILL.md:
- Keep templates declarative
- Minimize JavaScript logic in templates
- Use backend data as source of truth

## Do
- Remove/simplify the client-side shouldExpand calculation in loadJobTreeData()
- Remove redundant hasLogs/status checks that backend now handles
- Clean up any dead code related to old expansion logic
- Ensure tree renders correctly from backend JSON alone

## Accept
- [ ] Client-side expansion computation removed or minimal
- [ ] UI expansion driven by backend `expanded` field
- [ ] No dead/unused expansion code remaining
- [ ] Tree view works correctly with running jobs
