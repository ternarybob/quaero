  1. Investigations -> docs\fix\queue_manager\websocket_job_logging.md\

  2.C:/Users/bobmc/Pictures/Screenshots/ksnip_20251203-084256.png
  Show inconsistant logging to UI, some 'step' no 'worker'. And a double up of some logs/events.\

  Actions:

  1. Events/logging should be defined by step and worker. This is consistant with the queue model, where job manager, step manager and worker. THe logging receiver, should be a pass thorugh, and the level / fuinction     
  (job/step/worker) should provide the context/level to the logger, which passes this through to the web socket collector / UI.\

  2. Identify why there is doubleing of log entries.\

  3. THe logging needs to be updated with more relebant context. i.e. [time] [level] [worker] Downloading url: xxxx \

  4. Remove the level emoji, replace with standard [INF],[DBG] etc. And match the colors form the Service Logs. Maintain white/transparent background.\

  5. update test\api\websocket_job_events_test.go iterate to pass. pass if websocket messages from [step] and [worker]

  6. update test\ui\queue_test.go -> TestStepEventsDisplay to test for changes listed above, specifically the events showing from [steps] and [worker]. Logging like [worker] page download complete url:xxx.com
  elapsed:xxx.x\

  7. execute test\api\websocket_job_events_test.go iterate to pass. Specifically, the web socket log, should show the message. Not just "Received WebSocket message: type=step_progress" iterate to pass\

  8. execute test\ui\queue_test.go -> TestStepEventsDisplay. Iterate to pass 