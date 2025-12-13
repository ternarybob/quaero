1. The UI is displaying all the events (scrolling), in the step. The step workers finish within 1 second the UI should only display initial 100, then last 100. i.e. The websocket will message [step_1] start, UI will get the events from the api, websocket will says [step_1] complete, UI will get the events from the api.

2. The Service Logs also need to be updated to same buffering approach. When there is high a volume in logging, the UI is not able to keep up, and creates a bottle neck in the UI. The UI (service logs) should display the logs in batches and be triggered by the websocket. 

eg. C:/Users/bobmc/Pictures/Screenshots/ksnip_20251210-071029.png the UI is createing many APi requests long after the job has completed.
