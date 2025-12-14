1. Create a new worker, error_generator_worker.go, which generates log items, with delays , and then randomly gernates warnings and error logs. The worker recursivly create new workers, some of which fail
2. Create a new test, which executes a job error_generator.md (test\config\job-definitions)
2.1 Assert that the job (overall) will stop runnning and kill all queued wokers, if a failure occurs. Based upon the errro tollerance setup in the job definition.

[error_tolerance]
max_child_failures = 50
failure_action = "continue"

2.2 assert that the UI for the stop, shows a status of all logs INF 1000, WRN 100, ERR 50 in the step card header
2.3 assert that errors are maintained as a separate block, above on-going logs
3. Run the test and iterate to pass