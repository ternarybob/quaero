# Task 1: Audit and Document Current Expansion Logic Issues
Depends: - | Critical: no | Model: sonnet | Skill: none

## Addresses User Intent
Understand the current state of UI expansion logic to identify what needs to be moved to backend

## Skill Patterns to Apply
N/A - no skill for this task (research/audit only)

## Do
- Review pages/queue.html loadJobTreeData() function (lines 2417-2445)
- Document the current auto-expand logic and conditions
- Review GetJobTreeHandler in job_handler.go to see what expansion state is returned
- Identify the gap between backend response and UI expectations

## Accept
- [ ] Current expansion logic documented
- [ ] Gap between backend/UI identified
- [ ] Clear list of changes needed
