The [[steps]] is not KISS, for all job type types.
eg.
[[steps]] most fields are redudant, as the step is idenpendant of the parent [[steps]]

[steps.config] define a configuration for a step, and then further hiearcical configs, like location, filters etc. [steps.config.location]

Action:\

Change the job steps to be defined as the following:

1. The name of a step is in the toml eg. [step.{step-name}]

2. Step depedancies  
dependant={common delimeted names of dependant steps, within the config (not outside)}
If the step does not have a dependancy, then it is queued along with other steps. If there is a dependancy listed, then it should await the dependancy to complete

Note: most jobs do NOT have dependancies, and hence only run one step. 

3. configuration items should be flat, within a step.
eg. (bin\job-definitions\agent-document-generator.toml)
[steps.config.document_filter]
source_type = "crawler"
tags = ["technical-documentation"]
limit = 20
updated_after = "2025-10-01T00:00:00Z"

should be 
[step.{step-name}]
source_type = "crawler"
tags = ["technical-documentation"]
limit = 20
updated_after = "2025-10-01T00:00:00Z"

4. Remove redudant toml configuration for steps. i.e. there is not reaons to have 
name = "generate_summaries"
action = "create_summaries"  # Free-text action name (agent jobs support custom action names)
THese can be replaced by name (in the step [step.{name}]) and description = "thisd is the job description"

5. Update the test configurations (test\config\job-definitions) and execute the tests. 

6. Once the test pass, update the example configurations (deployments\local\job-definitions) and user test configurations (bin\job-definitions) 

7. Remove reduidant toml config items and redudant code. 

Note: Breaking changes are ok, this service is in agressive development. 