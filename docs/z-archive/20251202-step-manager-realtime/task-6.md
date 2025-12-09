# Task 6: Build and test fix
Depends: 1,2,3,4,5 | Critical: no | Model: sonnet

## Addresses User Intent
Ensures the fix works end-to-end and doesn't break existing functionality.

## Do
1. Build the Go backend: `go build -o /tmp/quaero.exe ./cmd/quaero`
2. Verify no compilation errors
3. Run the application and test:
   - Create a new places_search_manager job
   - Verify Step 1 events appear in real-time in the Events panel
   - Verify Step 2 events appear in real-time as agent jobs execute
   - Verify progress bar updates as child jobs complete
4. Check browser console for any JavaScript errors

## Accept
- [ ] Go code compiles without errors
- [ ] Step events panel shows real-time events during job execution
- [ ] Events bubble up correctly from worker → step → manager
- [ ] Progress updates in real-time (not just on 5-second polls)
