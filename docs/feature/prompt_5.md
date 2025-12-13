1. The test is NO monitoriong the pages (without refresh every 10 seconds)
2. The test does not have the required assersions. The code update failed to resolve the bugs
2.1 THe first step, should show 1 -> 15 logs, only shows 5 -> 15
C:\development\quaero\test\results\ui\job-20251212-171401\TestJobDefinitionCodebaseClassify\04_status_codebase_classify_running.png
2.2 Step 2 did NOT autoexpand, to show the logs
test\results\ui\job-20251212-171401\TestJobDefinitionCodebaseClassify\05_status_codebase_classify_completed.png
2.3 No test for API log call count

3. Update the test -> test\ui\job_definition_codebase_classify_test.go, with the following assersions.
  1. Step Log Api request count less < 10. 5000 log entried, and these occur within 130 seconds. Exclude service logs
  2. All steps are expanded in order of step completion. i.e. code_map / import_files -> rule_classify_files. on the runnign job, with NO page refresh.
  3. Step 1 and 2 show 1 -> 15 logs. 