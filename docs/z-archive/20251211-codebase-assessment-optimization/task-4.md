# Task 4: Implement 100-Item Log Limit with Earlier Logs Indicator

Depends: 1 | Critical: no | Model: sonnet | Skill: frontend

## Addresses User Intent

Implements "100-item limit" on step logs, ordered earliest-to-latest, showing '...' when earlier logs exist.

## Skill Patterns to Apply

- Alpine.js computed properties
- Array slicing for log limits
- CSS for truncation indicator

## Do

1. Add constant for log limit in `pages/queue.html`:
   ```javascript
   maxLogsPerStep: 100,
   ```

2. Update log rendering to show limited logs:
   - Display logs in earliest-to-latest order (ascending timestamp)
   - Show only the last 100 logs
   - When logs exceed 100, display "..." indicator at top

3. Add "earlier logs" indicator:
   ```html
   <template x-if="step.logs && step.logs.length > maxLogsPerStep">
     <div class="earlier-logs-indicator">
       ... {step.logs.length - maxLogsPerStep} earlier logs
     </div>
   </template>
   ```

4. Update `getFilteredTreeLogs` function:
   - Accept maxLogs parameter
   - Return slice of logs from end (most recent)
   - Maintain earliest-to-latest order in display

5. Style the indicator:
   - Gray text, italic
   - Small font size
   - Left-aligned with logs

## Accept

- [ ] Logs display in earliest-to-latest order
- [ ] Maximum 100 logs shown per step
- [ ] "... N earlier logs" indicator shows when logs exceed 100
- [ ] New logs append at bottom (maintaining order)
- [ ] Older logs drop off when limit exceeded
