# Task 2: Fix child refresh interval to not poll when all children present
Depends: 1 | Critical: no | Model: sonnet | Skill: frontend

## Addresses User Intent
Stops the continuous /jobs?parent_id=... requests that fire every 2 seconds. The interval should only fetch if children are actually missing.

## Skill Patterns to Apply
- Alpine.js reactive patterns
- Minimize network requests

## Do
1. In queue.html, modify the _childRefreshInterval logic (line 1875) to:
   - Clear interval when no parent jobs have missing children
   - Only re-enable when a parent job gets new child_count > current children
2. Add tracking for whether fetch is already in progress to avoid duplicate requests
3. Consider removing the interval entirely if WebSocket events reliably deliver child job spawns

## Accept
- [ ] No continuous /jobs?parent_id requests when all children are present
- [ ] Children still fetched when actually missing
- [ ] No duplicate concurrent requests for same parent
