# Task 1: Fix filter_source_type bug in job definition
- Group: 1 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: none
- Sandbox: /tmp/3agents/task-1/ | Source: ./ | Output: docs/feature/20251130-dual-steps-ui/

## Files
- `test/config/job-definitions/nearby-resturants-keywords.toml` - fix filter_source_type

## Requirements
The agent worker filters documents by source_type but the places worker creates documents with source_type="places". The job definition incorrectly uses filter_source_type="crawler" which causes zero documents to match.

Fix: Change `filter_source_type = "crawler"` to `filter_source_type = "places"`

## Acceptance
- [ ] filter_source_type changed to "places"
- [ ] Compiles
