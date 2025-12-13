Problem:
You have reverted the changes and the UI design, and these do NOT match requriements.
C:/Users/bobmc/Pictures/Screenshots/ksnip_20251201-070540.png

This was close -> test\results\ui\nearby-20251130-230801\TestNearbyRestaurantsJob\status_nearby_restaurants_(wheelers_hill)_completed.png
- listed children in separate lines
- status for children was independant

Action:
1. Update the UI to match the following requirements:
- ALL queued jobs, need to have a consistant and same UI
- The parent states the details about the job and shows the overall status (completed, failed, running, etc.)
- the parent lists the no. of UNIQUE documents created/uypdates/deleted.
- The children show the status of the child job, the progress and the no. of UNIQUE documents created/updated/deleted.
- the children are listed in order of execution, noting that the parent desides on order, where some jobs may list a dependancy

2. Add a config option to the job definition to enable the document filtering 
- 'document_filter_tags' lists the documents filter, buy 'tags' that a step should be executed against. If not provided, the step should be executed against all documents.  