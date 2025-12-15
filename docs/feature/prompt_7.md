1. The job (error_generator) is NOT captuing failed jobs as errors and logging to the step. THe job maybe failing (error) however there is NOT logging to this effect. Update the worker to capture failed jobs and create a ERR log    
entry.\

2. The job (error_generator) needs to create WRN level errors. 

3. THe job steps should show the total (regardless of level) log entries and number shown (like logs:56/100). The log api endpoint, should include as part of the response.

4. Update test\ui\job_definition_general_test.go and create assertions

4.1  Job log filter level filter C:/Users/bobmc/Pictures/Screenshots/ksnip_20251214-132208.png Filter like (same) as settings?a=logs -> C:/Users/bobmc/Pictures/Screenshots/ksnip_20251214-131927.png

Assertions
- Click on filter shows ERR/WRN/INF... 
- Selected levels shows ONLY level items. \
- When the filter changes, this is a refresh request to the API, wwith level. Match the API log count with the items shown.

4.2 Remove free test filter. assert NO free test filter.

4.3 The refresh logs button is the same as "refresh Job Queue" button. C:/Users/bobmc/Pictures/Screenshots/ksnip_20251214-133829.png This button is the 'standard' accross the entire app. Assert the standard is met. 