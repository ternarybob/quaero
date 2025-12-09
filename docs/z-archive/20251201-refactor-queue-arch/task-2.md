# Task 2: Update Manager.ExecuteJobDefinition

Depends: 1 | Critical: yes:architectural-change | Model: opus

## Do

1. Rename "parent job" to "manager job" in ExecuteJobDefinition
   - Change job type from "parent" to "manager"
   - Update all references

2. For each step, create a "step job":
   - Type: "step"
   - ParentID: manager_id
   - ManagerID: manager_id
   - Contains step metadata (name, type, description)

3. Update step execution flow:
   - Create step job BEFORE calling worker.CreateJobs
   - Pass step_id to worker so children reference the step
   - Step job monitors its children (not manager)

4. Manager now monitors step jobs, not worker jobs:
   - Manager tracks step completion status
   - Manager aggregates step-level progress

## Accept

- [ ] Manager creates "manager" type job
- [ ] Each step creates a "step" type job
- [ ] Step jobs have parent_id = manager_id
- [ ] Workers receive step_id to set as parent for spawned jobs
- [ ] Code compiles without errors
