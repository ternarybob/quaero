1. The job/tree is not shoing all the steps.
C:/Users/bobmc/Pictures/Screenshots/ksnip_20251211-084148.png

2. The jobs is still running (icon) however is comeplted.

3. Clean the services and the front end, there should be no context specific code, unless absolutly nessecary. i.e. all job events/logs are standard logs, with context (key/value). The UI simply uses the context it has to create views.

4. The UI tree view should expand as the events are received. There is a 100 item limit on step events/logs, maintain this and order from earlly to latest, show '...' to the top, showing there are earlier logs.

5. Switch to a light view with the tree view. Black text, light gray background

6. As much as possible, maintain a div rather than a scrollable text box. 
