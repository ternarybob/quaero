# Task 3: Ensure Child Jobs Have Document Count Populated
Depends: 1 | Critical: no | Model: sonnet

## Problem
Child job rows should display their individual document counts. The UI template already has the code at line 250:
```html
<i class="fas fa-file-alt"></i> <span x-text="getDocumentsCount(item.job)"></span> docs
```

But child jobs may not have `document_count` in their metadata because:
1. Child crawler jobs save documents directly
2. The `IncrementDocumentCount()` only increments the PARENT job's count
3. Child jobs don't track their own document count

## Analysis
For child jobs (e.g., crawler jobs created per URL), the document count should be:
- The number of documents that child job created
- This is typically 1 for URL-based crawler jobs (one page = one document)
- For places_search children, each child typically creates 1 document per location

The `getDocumentsCount()` function in queue.html (line 2293) checks:
1. `job.document_count` from metadata
2. `job.metadata.document_count`
3. `job.progress.completed_urls`
4. `job.result_count`

## Do
1. Verify child jobs have `result_count` or `document_count` populated
2. If not, check where child job completion updates these fields
3. Ensure `getDocumentsCount()` returns meaningful values for child jobs
4. If needed, add `document_count` tracking to child job completion

## Accept
- [ ] Child job rows display their document count
- [ ] Count shows "N docs" format (not "N/A")
- [ ] Count reflects actual documents created by that child job
- [ ] Build compiles successfully
