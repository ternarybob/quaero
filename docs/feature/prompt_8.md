The test test\ui\job_definition_general_test.go, needs to replicate test\ui\job_definition_codebase_classify_test.go assertions.

1. Testing whilst job is running, not page refresh and screen shots.
2. assessing / assert job status, and only finish once job is complete, with timeout 5 minutes.
3. The test should be configured to run the error_generator job, as now.
4. Add another step, same error_generator job, but with a different name.

