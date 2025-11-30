# Task 2: Add Test for filter_source_type Filtering
Depends: 1 | Critical: no | Model: sonnet

## Context
Step 2 (extract_keywords) has `filter_source_type = "places"` configuration.
This filter should ensure the agent only processes documents from the places search (source_type="places").

Since each test uses a fresh database:
- Step 1 creates 20 documents with source_type="places"
- Step 2 should filter and find exactly 20 documents to process

## Do
1. Add test to verify filter_source_type works correctly
2. After job completes, check that Step 2 processed exactly 20 documents
3. Verify document count matches what Step 1 created
4. Test that the agent step's document count equals the places step's document count

## Accept
- [ ] Test verifies filter_source_type filters to 20 documents
- [ ] Test validates Step 2 document count matches Step 1 count
- [ ] Test should FAIL if filtering doesn't work correctly
