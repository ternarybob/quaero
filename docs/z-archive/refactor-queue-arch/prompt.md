Current queue architecture
    - Parent is parent to spawned jobs, and monitors/tracks events, logs, overall status.  
    - Jobs/Steps run a job which can spawn additional children (jobs) into the queue. Each queued job publishs events and logs, which are collected by the parent.

However, as the jobs are not parents to spawned jobs, the parent is managing the spawed jobs. But, the steps should actually be the parent to the spawned jobs. So, without listing 1000+ steps, how to maintain a simple and only 1 level deep parent/child structure?

Changes
- Levels Manager -> Steps -> Jobs
- The Top level is the manager, which contains steps. The manager only monitors events, logs and overall steps status, along with job description details, create/edit/etc.
- The manager publishes events to the UI, however the UI only updates the top level pannel.
- All steps MUST spawn jobs to compelte the required actions. i.e. the step inserts a job into the queue, and then monitors the spawned children (similar to the top level manager). 
- There can be mulitple steps, with dsepedancies on other steps.
- All jobs publish events and logs to the parent (steps), The Step then publishes to the UI (same as the manager), however the UI receives and update the job log pannel
- A job can spawn child jobs, which will be monitored the parent (step). i.e. keeping all jobs at the same level.

Actions: 
- queue code and align to the requirements
- Test test\config\job-definitions\github-repo-collector-by-name.toml to validate
