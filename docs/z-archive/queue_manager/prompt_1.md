The step manager is NOT updating in real time.

C:/Users/bobmc/Pictures/Screenshots/ksnip_20251202-064322.png

The job is comeplte, however the job queue tool bar @ top of page, is updateing however the step manager pannell is not.

Actions: 
1. Review the internal\queue and ensure that the structure is clean, 
manager - monitors all steps and shows job description details. create/update/start
step manager - monitors worker and shows worker progress in current step, status, works running/compelted, etc
worker - the actual process running in the background

- Events and logs are the communication between the layers. 
- All events/logs for all jobs (filterable to > DBG) should be collected and stored in the database at the manager level.
- All events/logs for all jobs (filterable to > INF) should be collected published to the UI i.e. matching the UI. 

Review how the UI receives the worker events/logs and how they are placed into the pannels. This will determin is the manager or the step manager publishs the events/logs to the UI. Note that the job context, and hence what is important, summaries etc, are at the step manager level and step managers are resp. for tracking the workers, progress, start/stop, etc.

The code / laters should be designed to bubble up events/logs from the worker to the step manager and then to the manager. It should be clean and not context should be required at each layer, to understand the child. i.e. separation oif concerns and each layer performs the correct function.

2. The logs show that a job will start, the events/logs will initally publish from the worker, and publish from the step manager, however stop (updating) until the job is compelted. bin\logs\quaero.2025-12-02T07-12-10.log

