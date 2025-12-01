# Task 1: Replace confirm() with Modal in rerunJob

- Group: 1 | Mode: sequential | Model: sonnet
- Skill: @frontend-developer | Critical: no | Depends: none
- Sandbox: /tmp/3agents/task-1/ | Source: C:\development\quaero | Output: C:\development\quaero\docs\plans

## Files
- `pages/queue.html` - Replace native confirm() with window.confirmAction() in rerunJob function

## Requirements

Replace the native browser `confirm()` dialog in the `rerunJob()` function with the existing `window.confirmAction()` modal helper.

Current code (line ~1249):
```javascript
if (!confirm(`Copy and queue job ${jobId.substring(0, 8)}?\n\nThis will create a NEW job (copy) with the same configuration and add it to the queue.\nThe job will NOT execute immediately - it will be queued as "pending".\nThe original job will remain unchanged.`)) {
    return;
}
```

Replace with:
```javascript
const confirmed = await window.confirmAction({
    title: 'Copy and Queue Job',
    message: `This will create a NEW job (copy) with the same configuration and add it to the queue. The job will execute when workers are available. The original job will remain unchanged.`,
    confirmText: 'Copy & Queue',
    type: 'primary'
});

if (!confirmed) {
    return;
}
```

Note: The message should indicate the job WILL execute (after fixing Task 2), not that it "will NOT execute".

Also ensure the function is async (add async keyword if not present).

## Acceptance
- [ ] rerunJob function uses window.confirmAction() instead of confirm()
- [ ] Modal shows appropriate title and message
- [ ] Cancel closes modal without action
- [ ] Confirm proceeds with job copy
- [ ] Compiles (HTML syntax valid)
