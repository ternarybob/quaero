# Task 8: Test with github-repo-collector-by-name.toml

Depends: 7 | Critical: no | Model: sonnet

## Do

1. Read test job definition:
   - `test\config\job-definitions\github-repo-collector-by-name.toml`

2. Run the job through the UI:
   - Start quaero server
   - Navigate to Queue Management
   - Execute the job definition
   - Observe the new hierarchy

3. Verify:
   - Manager job created with type="manager"
   - Step job(s) created with type="step", parent=manager
   - Worker jobs created with parent=step, manager_id=manager
   - Step monitors its jobs correctly
   - Manager monitors steps correctly
   - UI displays hierarchy correctly
   - Events flow correctly

4. Test edge cases:
   - Step with no spawned jobs
   - Step with failed jobs
   - Multiple steps with dependencies
   - Job that spawns grandchildren (should stay under step)

## Accept

- [ ] Job definition executes successfully
- [ ] Manager → Step → Job hierarchy visible in UI
- [ ] Step progress updates correctly
- [ ] Manager progress updates correctly
- [ ] All jobs complete and manager shows completed
