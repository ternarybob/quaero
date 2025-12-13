1 ALL jobs need to execute using the same procerss steps. This will neable both monitoring andf UI outout.

Top level -> Parent, monior/event (subscriber), logging (all children publish logs), child job order
Job level -> Execute job, assess if futher jobs executed and execute / publish events and logs\

The process hierarchy should only have 2 levels. The job itself, may have levels of data/meta data, however jobs are executed at the same level 

2. The UI needs to be simplified and just track jobs as an events display, under the parent and job name, as defining categories. The ojb is resp. for publishing the events to the parent, the parent is resp. for publishing to the UI.
- As documents are updated, they should have the parent id, jobs id, and hence parent can assess unique document count

eg. 
Nearby Restaurants (Wheelers Hill)
    ---------------------------------------------------------
    - 2 Jobs scheduled for processing 
        - Step 1 - search_nearby_restaurants (places_search)
        - Step 2 - extract_keywords (agent)
    - Status (Pending, Running, Completed, Failed, Cancelled)
    ---------------------------------------------------------
    - (1/2) search_nearby_restaurants
        - Status (Pending, Running, Completed, Failed, Cancelled)
        - (List events, from job)
        - (datetime) Searched for restaurants near Wheelers Hill
        - (datetime) Created 20 documents
    - (2/2) extract_keywords
        - Status (Pending, Running, Completed, Failed, Cancelled)
        - (List events, from job)
        - (datetime) Extracting keywords from 20 documents
        - (datetime) Job Extracted keywords for document id xxxyyy
        - (datetime) Saved keywords for document id xxxyyy
        - (datetime) Job Extracted keywords for document id wwwrrr
        - (datetime) Error -> Job Failed to extract keywords for document id wwwrrr
        - .,..
    
- enables recall, as the log should be all saved under the parent.
- simple UI, simple procers|
- NO multi depth of process / jobs

Actions:
- Review the queue (internal\queue) and align/confirm ALL job types to the simple method. 
    - Ensure parent evvent and logging is implemenbts.
    - Ensure all jobs publish events and logs to the parent.
    - Ensure child jobs are able to create more child jobs, under the parent and are managed and ordered by the queue, with the concurreny settings
- Align the UI to the simple model.
    - Change to a logging type interface, however maintain simple approach.
    - The service drives the output to the UI with websockts whilst the job is running, and then the UI can poll for updates once the job is complete, or the page refreshes mid-job.
