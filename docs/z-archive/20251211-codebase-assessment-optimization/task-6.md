# Task 6: Build and Verify All Changes

Depends: 1,2,3,4,5 | Critical: no | Model: sonnet | Skill: go

## Addresses User Intent

Final verification that all changes compile and work together.

## Skill Patterns to Apply

- Go build process
- Integration testing approach
- End-to-end verification

## Do

1. Run Go build:
   ```bash
   cd C:\development\quaero
   scripts/build.ps1
   ```

2. Verify no compilation errors

3. Manual verification checklist:
   - Start the server
   - Create a test job with multiple steps
   - Verify tree view shows all steps
   - Verify status icons are correct
   - Verify light theme is applied
   - Verify logs have 100-item limit
   - Verify divs instead of scrollable boxes

4. Fix any issues found

## Accept

- [ ] `scripts/build.ps1` completes without errors
- [ ] Server starts successfully
- [ ] Tree view displays correctly with all fixes
- [ ] No console errors in browser
- [ ] All user intent items verified
