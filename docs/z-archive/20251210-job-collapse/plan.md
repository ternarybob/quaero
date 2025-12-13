# Plan: Job Steps Collapse Toggle

Type: fix | Workdir: ./docs/fix/20251210-job-collapse/

## User Intent (from manifest)

When the user clicks on a parent job card in the queue page (specifically the header/metadata area with timestamps), the step rows below should collapse or expand. Currently clicking navigates to job details - user wants a toggle for step visibility.

## Active Skills

none

## Tasks

| # | Desc | Depends | Critical | Model | Skill |
|---|------|---------|----------|-------|-------|
| 1 | Add collapsedJobs state to track which parent jobs have collapsed steps | - | no | sonnet | - |
| 2 | Add toggleJobStepsCollapse method and isJobStepsCollapsed helper | 1 | no | sonnet | - |
| 3 | Add click handler to job card metadata area for collapse toggle | 2 | no | sonnet | - |
| 4 | Update renderJobs to skip steps for collapsed parent jobs | 2 | no | sonnet | - |
| 5 | Add visual indicator (chevron icon) showing expand/collapse state | 3 | no | sonnet | - |

## Order

[1] → [2] → [3, 4, 5]
