# Task 1: Add Child Job Rows to UI Template

- Group: 1 | Mode: sequential | Model: sonnet
- Skill: @frontend-developer | Critical: no | Depends: none
- Sandbox: /tmp/3agents/task-1/ | Source: ./ | Output: ./docs/fix/20251201-dual-steps-ui/

## Files
- `pages/queue.html` - Add child job row template after step rows

## Requirements

The reference screenshot (test/results/ui/nearby-20251130-230801/TestNearbyRestaurantsJob/status_nearby_restaurants_(wheelers_hill)_completed.png) shows:
- Parent job with PARENT badge, status, document count, timestamps
- Below parent: "Step 1/1: search_nearby_restaurants [places_search] [Completed]"
- The step shows its own status and description

Current issue: Children are shown as inline "Progress: 4 pending, 10 running, 6 completed" text
Required: Children should be listed as separate rows under the parent, similar to steps

Update the queue.html template to:
1. After rendering step rows for multi-step jobs, also render child job rows
2. Each child should display on its own line with:
   - Status icon/badge (using same getStatusIcon/getStatusBadgeClass helpers)
   - Child job name
   - Document count
   - Status (pending/running/completed/failed)
3. Children should be visually indented like step rows (margin-left: 2rem)
4. Children should have a distinct style to differentiate from steps

## Acceptance
- [ ] Child jobs are displayed as separate rows under parent
- [ ] Each child shows its own status badge
- [ ] Each child shows its document count
- [ ] Children are visually indented under parent
- [ ] Compiles
- [ ] Tests pass
