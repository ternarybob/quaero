test\config\job-definitions\nearby-resturants-keywords.toml

1. Update the UI to show no. of steps and names. And split progress into the steps. i.e. show all child/steps in order of execution/dependany under the parent as separate job lines. Ensure they update independantly, using the existing websocket approach.

2. Create a new test for the nearby-resturants-keywords.toml job definition, and execute the job

3. The test should verify 
- job completes successfully
- job creates documents
- job extracts keywords from the documents
- the keywords are stored in the database

4. Execute the test, iterate to complete/pass

Notes:
- Current this job executes however does not stop/complete. C:/Users/bobmc/Pictures/Screenshots/ksnip_20251130-154100.png this is fail.
- folow test\ui\queue_test.go -> TestNewsCrawlerCrash as a template for monitoring.