# Task 3: Simplify UI to Render Tree from Backend JSON Structure
Depends: 2 | Critical: no | Model: sonnet | Skill: frontend

## Addresses User Intent
"The UI renders the entire job status and logs, based upon the json received"
"Show the tree as structured from the API"

## Skill Patterns to Apply
From frontend/SKILL.md:
- Alpine.js reactive data binding
- Use x-data for component state
- Template expressions for conditional rendering

## Do
- Modify pages/queue.html loadJobTreeData() function
- Use backend `expanded` field as initial state for each step
- Simplify auto-expand logic to respect backend decision
- Keep user override capability (if user collapses, respect that)

## Accept
- [ ] UI uses backend `expanded` field as default state
- [ ] User can still toggle expand/collapse locally
- [ ] Auto-expand respects user's explicit collapse action
- [ ] No JavaScript errors in console
