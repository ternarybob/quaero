This refactor is to update the job steps. The current jobs set the type in the parent, this refactor will move the job type into the step(s). This will enable the steps to define the job type, support multiple types of steps within a single job and focus the job execution in the job/worker not the manager. 

1. review the test\config\job-definitions and understand how this configures jobs and how they are tracked. 

2. review the job manager/worker/monitoring internal\queue 

3. The change should create a generic manager, and type defeined workers. THe workers should all comply to an interface and hence execution and monitoring should be consistant.

4. Remove redudant toml configuration for steps. i.e. there is not reaons to have 
name = "generate_summaries"
action = "create_summaries"  # Free-text action name (agent jobs support custom action names)
THese can be replaced by name (in the step [step.{name}]) and description = "thisd is the job description"

5. Update the test configurations (test\config\job-definitions) and execute the tests. 

6. Once the test pass, update the example configurations (deployments\local\job-definitions) and user test configurations (bin\job-definitions) 

7. Update the docs\architecture\MANAGER_WORKER_ARCHITECTURE.md to align to the new code structure (filename should be lowercase)

8. Remove redundant toml config items and redudant code. 

Note: Breaking changes are ok, code is NOT required to be backward compatable as this service is in agressive development. 