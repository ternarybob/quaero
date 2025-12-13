Review and reachitech the job logging/events/monitor and UI display.
- Jobs/workers should log with simple messages and key/value context
- The job monitor should maintain status of the job, with a list of steps and relevant operational meta data
- The api pulls together the job monitor status and logs, and serves to the UI a structured json view
- The UI renderes the entire job status and logs, based uon the json received. The Ui is triggered to update from the websockets, which is monitoring status and log changes, from the monitor and logging.

Fundamental issues 
1. the backend is drifting into specific code for specific issues. Implement a VERY simple approach, using the tools available, queue manager /  badgerhold and arbor.
2. The C:/Users/bobmc/Pictures/Screenshots/ksnip_20251211-114653.png shows the UI using code to triggering expansion and status, this is NOT required and should be driven from the backend. i.e. show the tree as structured from the API. The json should be structured in such a way as to be able to re-render a running job keep the front code and output simple.
3. Ensure separation of concerns. 
Worker - completes the work and logs with available context (job_id, step_id, worker_id). 
Logging - stores log entries with index (that is it)
Monitor - monitors running jobs and updates status, counts, etc.
Websockets - monitors for changes and sends to the UI
UI - renders to the Users
