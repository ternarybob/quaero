The test\ui\job_definition_codebase_classify_test.go test found the following.

FAILING assertions (test expectation mismatches, not functional issues):
- Assertion 0: Progressive log streaming (batch mode processes synchronously)
- Assertion 1: WebSocket message count slightly over 40 limit
- Assertion 4: Line numbering gaps (due to concurrent logging in batch mode)
- Assertion 6: Minor total count mismatch (37 logs difference)\

Fixes to implement 
1. Assertion 0: Progressive log streaming (batch mode processes synchronously)
   - Maintain the webwocket refresh trigger and UI api call ONLY approach. However, ensure the webwocket trigger, as a scaling rate limiter, which processes or triggers the UI to get logs the following way. 
   1. Job start (should be convered in UI by status change) -> refresh all step logs
   4. Step start (should be convered in UI by status change) -> refresh step logs
   3. 1 sec, 2 sec, 3 sec, 4 sec (scale) -> 10 seconds and then process as per normal -> every 10 seconds. 
   4. Step complete (should be convered in UI by status change) -> refresh step logs
   4. Job completion (should be convered in UI by statsu change) -> refresh all step logs

2. Assertion 1: WebSocket message count slightly over 40 limit
   - The threshold can be calculated by the number of steps and time taken for each step, this would be an calculated assertion.

3. Assertion 4: Line numbering gaps (due to concurrent logging in batch mode)
   - To enable the line number assertion, all levels can be included in the log assessment.  
