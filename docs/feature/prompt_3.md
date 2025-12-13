The UI issue remains, regarding a running job and no page refresh. C:/Users/bobmc/Pictures/Screenshots/ksnip_20251211-121500.png
  
  The Ui is clearly NOT processing accoring to websocket triggers and also is NOT rendering from logs collected.\
  
  Break this down to simplier functions.
  
  - The backend should provide a job structure endpoint - 1. overall (status, created, started, ended)  2. steps (status, no. of logs)
  - The above should then be rendered and UI watches for web socket triggers to update, the status and logs 
  - If a job is running, then the service provide the UI (via websockets) context=job;job_id=123123;status={running|completed}
  - If a step is running, then the service provide the UI (via websockets) context=job_step;job_id=123123;step_id=123123;status={running|completed};log_refresh=true|false;
  - The UI is watching the websocket messages are derives actions. i.e. if context=job_step and log_refresh=true, then the UI will refresh the logs for that step.
  Notes:
  1. The log endpoint should provide logs, based upon the params. i.e. job_id | joid_id+step_id
  2. The expand logs is a UI function and default is to have all expanded. However the UI should have the capability to understand , though the job endpoint, the params required to get logs for the step. THere is not need to keep the logs (UI side), when the step is collapsed.  